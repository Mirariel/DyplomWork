package services

import (
	"fmt"
	"log/slog"
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
	logger     *slog.Logger
}

func NewSyncService(db *sqlx.DB, enc *EncryptionService, logger *slog.Logger) *SyncService {
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
			s.logger.Info("sync: starting full sync", "user_id", userID, "exchange", c.cred.Exchange)

			if err := s.syncExchangeFull(userID, c); err != nil {
				s.logger.Error("sync: exchange error", "user_id", userID, "exchange", c.cred.Exchange, "error", err)
				errMsg := err.Error()
				s.repo.updateSyncStatus(c.cred.ID, &errMsg)
				return
			}

			s.logger.Info("sync: exchange done", "user_id", userID, "exchange", c.cred.Exchange, "duration_s", time.Since(start).Seconds())
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

// SyncAllUsers — повний синк для всіх користувачів з активними credentials.
// Використовується фоновим scheduler'ом.
func (s *SyncService) SyncAllUsers() {
	var userIDs []int64
	if err := s.db.Select(&userIDs,
		"SELECT DISTINCT user_id FROM external_api_credentials WHERE is_active = 1",
	); err != nil {
		s.logger.Error("sync: get users for background sync", "error", err)
		return
	}
	s.logger.Info("sync: background sync started", "users", len(userIDs))
	for _, id := range userIDs {
		if err := s.SyncUser(id); err != nil {
			s.logger.Error("sync: background user sync failed", "user_id", id, "error", err)
		}
	}
	s.logger.Info("sync: background sync finished", "users", len(userIDs))
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
				s.logger.Error("sync: history error", "exchange", c.cred.Exchange, "error", err)
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
		s.logger.Error("sync: balances error", "exchange", c.cred.Exchange, "error", err)
	}

	if positions, err := ex.GetOpenPositions(c.creds); err == nil {
		s.processPositions(userID, positions, c.cred.Exchange)
	} else {
		s.logger.Error("sync: positions error", "exchange", c.cred.Exchange, "error", err)
	}

	if trades, err := ex.GetClosedTrades(c.creds, startMs, now.UnixMilli()); err == nil {
		s.processHistory(userID, trades, c.cred.Exchange)
	} else {
		s.logger.Error("sync: history error", "exchange", c.cred.Exchange, "error", err)
	}

	return nil
}

func (s *SyncService) processBalances(userID int64, balances []exchange.Balance, exchangeName string) {
	for _, b := range balances {
		assetID, err := s.repo.getOrCreateAsset(b.Symbol)
		if err != nil {
			s.logger.Error("sync: getOrCreateAsset", "symbol", b.Symbol, "error", err)
			continue
		}
		if err := s.repo.upsertBalance(userID, assetID, exchangeName, b.Quantity); err != nil {
			s.logger.Error("sync: upsertBalance", "symbol", b.Symbol, "error", err)
		}
	}
}

func (s *SyncService) processPositions(userID int64, positions []exchange.Position, exchangeName string) {
	s.logger.Info("sync: processing positions", "count", len(positions), "exchange", exchangeName)
	for _, p := range positions {
		// Нормалізуємо side до допустимих значень ENUM
		side := strings.ToUpper(p.Side)
		if side != "LONG" && side != "SHORT" {
			side = "LONG"
		}

		assetID, err := s.repo.getOrCreateAsset(p.Symbol)
		if err != nil {
			s.logger.Error("sync: getOrCreateAsset", "symbol", p.Symbol, "error", err)
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
			s.logger.Error("sync: upsertPosition", "symbol", p.Symbol, "error", err)
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
	s.logger.Info("sync: history inserted", "exchange", exchangeName, "inserted", inserted, "total", len(trades))
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
			s.logger.Error("sync: decrypt apiKey", "exchange", c.Exchange, "error", err)
			continue
		}
		apiSecret, err := s.encryption.Decrypt(c.ApiSecretEncrypted)
		if err != nil {
			s.logger.Error("sync: decrypt apiSecret", "exchange", c.Exchange, "error", err)
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
		s.logger.Warn("sync: unknown exchange", "exchange", name)
		return nil
	}
	return ex
}
