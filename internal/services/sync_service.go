package services

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
)

// SyncService — паралельна синхронізація портфелю з кількома біржами.
// Кожна біржа синкується у своїй goroutine.
type SyncService struct {
	db         *sqlx.DB
	encryption *EncryptionService
	exchanges  map[string]exchange.Exchange
	repo       *syncRepository
	logger     *log.Logger
}

func NewSyncService(db *sqlx.DB, enc *EncryptionService, logger *log.Logger) *SyncService {
	return &SyncService{
		db:         db,
		encryption: enc,
		exchanges:  exchange.Registry(),
		repo:       newSyncRepository(db),
		logger:     logger,
	}
}

// credWithKeys — розшифровані облікові дані однієї біржі
type credWithKeys struct {
	cred  models.ExternalApiCredential
	creds exchange.Credentials
}

// SyncUser — повний синк: баланси + позиції + закриті угоди + спот-трейди.
// Всі біржі синкуються паралельно, після чого перераховуємо avg_buy_price та чистимо застарілі записи.
func (s *SyncService) SyncUser(userID int64) error {
	creds, err := s.getCredentials(userID)
	if err != nil || len(creds) == 0 {
		return err
	}

	syncStart := time.Now()

	// Паралельний синк усіх бірж
	var wg sync.WaitGroup
	for _, c := range creds {
		wg.Add(1)
		go func(c credWithKeys) {
			defer wg.Done()
			start := time.Now()
			s.logger.Printf("[sync] starting full sync for user=%d exchange=%s", userID, c.cred.Exchange)

			if err := s.syncExchangeFull(userID, c); err != nil {
				s.logger.Printf("[sync] error user=%d exchange=%s: %v", userID, c.cred.Exchange, err)
				errMsg := err.Error()
				s.repo.updateSyncStatus(c.cred.ID, &errMsg)
				return
			}

			s.logger.Printf("[sync] done user=%d exchange=%s in %.2fs",
				userID, c.cred.Exchange, time.Since(start).Seconds())
			s.repo.updateSyncStatus(c.cred.ID, nil)
		}(c)
	}
	wg.Wait()

	// Пост-обробка (послідовно, після завершення всіх goroutine)
	s.repo.transferCommentsToHistory(userID, syncStart)
	s.repo.deleteStalePositions(userID, syncStart)
	s.repo.deleteStaleBalances(userID, syncStart)
	s.repo.updateAverageBuyPrices(userID)

	return nil
}

// SyncPositions — лише відкриті позиції (+ 3 дні history для переносу коментарів).
func (s *SyncService) SyncPositions(userID int64) error {
	creds, err := s.getCredentials(userID)
	if err != nil || len(creds) == 0 {
		return err
	}

	syncStart := time.Now()
	now := time.Now()
	start3d := now.Add(-3 * 24 * time.Hour)

	var wg sync.WaitGroup
	for _, c := range creds {
		wg.Add(1)
		go func(c credWithKeys) {
			defer wg.Done()
			ex := s.getExchange(c.cred.Exchange)
			if ex == nil {
				return
			}

			// Спочатку history щоб коментар мав куди перейти
			if trades, err := ex.GetClosedTrades(c.creds, start3d.UnixMilli(), now.UnixMilli()); err == nil {
				s.processHistory(userID, trades, c.cred.Exchange)
			}

			if positions, err := ex.GetOpenPositions(c.creds); err == nil {
				s.processPositions(userID, positions, c.cred.Exchange)
			}
		}(c)
	}
	wg.Wait()

	s.repo.transferCommentsToHistory(userID, syncStart)
	s.repo.deleteStalePositions(userID, syncStart)
	return nil
}

// SyncBalances — лише спот-баланси.
func (s *SyncService) SyncBalances(userID int64) error {
	creds, err := s.getCredentials(userID)
	if err != nil || len(creds) == 0 {
		return err
	}

	syncStart := time.Now()

	var wg sync.WaitGroup
	for _, c := range creds {
		wg.Add(1)
		go func(c credWithKeys) {
			defer wg.Done()
			ex := s.getExchange(c.cred.Exchange)
			if ex == nil {
				return
			}
			if balances, err := ex.GetBalances(c.creds); err == nil {
				s.processBalances(userID, balances, c.cred.Exchange)
			}
		}(c)
	}
	wg.Wait()

	s.repo.deleteStaleBalances(userID, syncStart)
	return nil
}

// SyncRecentHistory — легкий синк закритих угод за останні N днів.
func (s *SyncService) SyncRecentHistory(userID int64, days int) error {
	creds, err := s.getCredentials(userID)
	if err != nil || len(creds) == 0 {
		return err
	}

	now := time.Now()
	startMs := now.Add(-time.Duration(days) * 24 * time.Hour).UnixMilli()
	endMs := now.UnixMilli()

	var wg sync.WaitGroup
	for _, c := range creds {
		wg.Add(1)
		go func(c credWithKeys) {
			defer wg.Done()
			ex := s.getExchange(c.cred.Exchange)
			if ex == nil {
				return
			}
			trades, err := ex.GetClosedTrades(c.creds, startMs, endMs)
			if err != nil {
				s.logger.Printf("[sync] history error exchange=%s: %v", c.cred.Exchange, err)
				return
			}
			s.processHistory(userID, trades, c.cred.Exchange)
		}(c)
	}
	wg.Wait()
	return nil
}

// --- internal ---

func (s *SyncService) syncExchangeFull(userID int64, c credWithKeys) error {
	ex := s.getExchange(c.cred.Exchange)
	if ex == nil {
		return fmt.Errorf("unknown exchange: %s", c.cred.Exchange)
	}

	now := time.Now()
	startMs := now.Add(-90 * 24 * time.Hour).UnixMilli()

	// Всі 4 операції послідовно в межах одного exchange (API rate limits)
	if balances, err := ex.GetBalances(c.creds); err == nil {
		s.processBalances(userID, balances, c.cred.Exchange)
	} else {
		s.logger.Printf("[sync] balances error exchange=%s: %v", c.cred.Exchange, err)
	}

	if positions, err := ex.GetOpenPositions(c.creds); err == nil {
		s.processPositions(userID, positions, c.cred.Exchange)
	} else {
		s.logger.Printf("[sync] positions error exchange=%s: %v", c.cred.Exchange, err)
	}

	if trades, err := ex.GetClosedTrades(c.creds, startMs, now.UnixMilli()); err == nil {
		s.processHistory(userID, trades, c.cred.Exchange)
	} else {
		s.logger.Printf("[sync] history error exchange=%s: %v", c.cred.Exchange, err)
	}

	return nil
}

func (s *SyncService) processBalances(userID int64, balances []exchange.Balance, exchangeName string) {
	for _, b := range balances {
		assetID, err := s.repo.getOrCreateAsset(b.Symbol)
		if err != nil {
			s.logger.Printf("[sync] getOrCreateAsset %s: %v", b.Symbol, err)
			continue
		}
		if err := s.repo.upsertBalance(userID, assetID, exchangeName, b.Quantity); err != nil {
			s.logger.Printf("[sync] upsertBalance %s: %v", b.Symbol, err)
		}
	}
}

func (s *SyncService) processPositions(userID int64, positions []exchange.Position, exchangeName string) {
	s.logger.Printf("[sync] processing %d positions from %s", len(positions), exchangeName)
	for _, p := range positions {
		// Нормалізуємо side до допустимих значень ENUM
		side := strings.ToUpper(p.Side)
		if side != "LONG" && side != "SHORT" {
			side = "LONG"
		}

		assetID, err := s.repo.getOrCreateAsset(p.Symbol)
		if err != nil {
			s.logger.Printf("[sync] getOrCreateAsset %s: %v", p.Symbol, err)
			continue
		}

		row := positionRow{
			UserID:     userID,
			AssetID:    assetID,
			Symbol:     p.Symbol,
			Exchange:   exchangeName,
			Side:       side,
			MarginMode: "cross",
			EntryPrice: p.EntryPrice,
			MarkPrice:  p.MarkPrice,
			Quantity:   p.Quantity,
			PnL:        p.PnL,
			Leverage:   fmt.Sprintf("%dx", p.Leverage),
		}
		if err := s.repo.upsertPosition(row); err != nil {
			s.logger.Printf("[sync] upsertPosition %s: %v", p.Symbol, err)
		}
	}
}

func (s *SyncService) processHistory(userID int64, trades []exchange.ClosedTrade, exchangeName string) {
	inserted := 0
	for _, t := range trades {
		closedAt := time.UnixMilli(t.ClosedAt).UTC().Format("2006-01-02 15:04:05")
		var openedAt *string
		if t.ClosedAt > 0 {
			// Bybit provides opened_at via CreatedTime; для Binance/OKX залишаємо nil
		}

		row := historyRow{
			UserID:      userID,
			Symbol:      t.Symbol,
			Side:        t.Side,
			Leverage:    "1x",
			MarginMode:  "cross",
			EntryPrice:  t.EntryPrice,
			ExitPrice:   t.ClosePrice,
			Quantity:    t.Quantity,
			RealizedPnl: t.PnL,
			Fee:         0,
			MaxSize:     t.Quantity * t.EntryPrice,
			OpenedAt:    openedAt,
			ClosedAt:    closedAt,
			Exchange:    exchangeName,
		}
		if err := s.repo.insertHistory(row); err == nil {
			inserted++
		}
	}
	s.logger.Printf("[sync] history %s: inserted %d/%d records", exchangeName, inserted, len(trades))
}

func (s *SyncService) getCredentials(userID int64) ([]credWithKeys, error) {
	var raw []models.ExternalApiCredential
	err := s.db.Select(&raw,
		"SELECT * FROM external_api_credentials WHERE user_id = ? AND is_active = 1",
		userID)
	if err != nil {
		return nil, err
	}

	result := make([]credWithKeys, 0, len(raw))
	for _, c := range raw {
		apiKey, err := s.encryption.Decrypt(c.ApiKeyEncrypted)
		if err != nil {
			s.logger.Printf("[sync] decrypt apiKey for %s: %v", c.Exchange, err)
			continue
		}
		apiSecret, err := s.encryption.Decrypt(c.ApiSecretEncrypted)
		if err != nil {
			s.logger.Printf("[sync] decrypt apiSecret for %s: %v", c.Exchange, err)
			continue
		}
		passphrase := ""
		if c.PassphraseEncrypted != nil {
			passphrase, _ = s.encryption.Decrypt(*c.PassphraseEncrypted)
		}

		result = append(result, credWithKeys{
			cred: c,
			creds: exchange.Credentials{
				APIKey:     apiKey,
				APISecret:  apiSecret,
				Passphrase: passphrase,
			},
		})
	}
	return result, nil
}

func (s *SyncService) getExchange(name string) exchange.Exchange {
	ex, ok := s.exchanges[strings.ToLower(name)]
	if !ok {
		s.logger.Printf("[sync] unknown exchange: %s", name)
		return nil
	}
	return ex
}
