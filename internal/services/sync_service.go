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
	db            *sqlx.DB
	encryption    *EncryptionService
	exchanges     map[string]exchange.Exchange
	repo          *syncRepository
	futuresRepo   *models.FuturesPositionRepository
	spotTradeRepo *models.SpotTradeRepository
	logger        *slog.Logger
}

func NewSyncService(db *sqlx.DB, enc *EncryptionService, logger *slog.Logger) *SyncService {
	return &SyncService{
		db:            db,
		encryption:    enc,
		exchanges:     exchange.Registry(),
		repo:          newSyncRepository(db),
		futuresRepo:   models.NewFuturesPositionRepository(db),
		spotTradeRepo: models.NewSpotTradeRepository(db),
		logger:        logger,
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

// SyncFuturesForUser синхронізує ф'ючерсні позиції одного користувача з усіх бірж.
// Викликається після додавання credentials (auto-discovery) або за розкладом.
func (s *SyncService) SyncFuturesForUser(userID int64) error {
	creds, err := s.getCredentials(userID)
	if err != nil || len(creds) == 0 {
		return err
	}

	var wg sync.WaitGroup
	for _, c := range creds {
		wg.Add(1)
		go func(c credWithKeys) {
			defer wg.Done()
			s.syncFuturesExchange(userID, c)
		}(c)
	}
	wg.Wait()
	return nil
}

// SyncFuturesAllUsers синхронізує ф'ючерсні позиції всіх активних користувачів.
// Використовується фоновим scheduler'ом (30 s).
func (s *SyncService) SyncFuturesAllUsers() {
	var userIDs []int64
	if err := s.db.Select(&userIDs,
		"SELECT DISTINCT user_id FROM external_api_credentials WHERE is_active = 1",
	); err != nil {
		s.logger.Error("futures sync: get users", "error", err)
		return
	}
	for _, id := range userIDs {
		if err := s.SyncFuturesForUser(id); err != nil {
			s.logger.Error("futures sync: user failed", "user_id", id, "error", err)
		}
	}
}

func (s *SyncService) syncFuturesExchange(userID int64, c credWithKeys) {
	ex := s.getExchange(c.cred.Exchange)
	if ex == nil {
		return
	}

	syncStart := time.Now()

	positions, err := ex.GetOpenPositions(c.creds)
	if err != nil {
		s.logger.Error("futures sync: get positions", "exchange", c.cred.Exchange, "error", err)
		return
	}

	for _, p := range positions {
		side := strings.ToUpper(p.Side)
		if side != "LONG" && side != "SHORT" {
			side = "LONG"
		}
		marginType := p.MarginType
		if marginType == "" {
			marginType = "cross"
		}
		fp := &models.FuturesPosition{
			UserID:        userID,
			Exchange:      c.cred.Exchange,
			Symbol:        p.Symbol,
			Side:          side,
			Size:          p.Quantity,
			EntryPrice:    p.EntryPrice,
			MarkPrice:     p.MarkPrice,
			UnrealizedPnl: p.PnL,
			Leverage:      p.Leverage,
			MarginType:    marginType,
		}
		if err := s.futuresRepo.Upsert(fp); err != nil {
			s.logger.Error("futures sync: upsert", "symbol", p.Symbol, "error", err)
		}
	}

	// Видаляємо позиції, що зникли з біржі (закриті)
	if err := s.futuresRepo.DeleteStale(userID, c.cred.Exchange, syncStart); err != nil {
		s.logger.Error("futures sync: delete stale", "exchange", c.cred.Exchange, "error", err)
	}

	s.logger.Info("futures sync: done", "exchange", c.cred.Exchange, "positions", len(positions))
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
	balances, err := ex.GetBalances(c.creds)
	if err != nil {
		s.logger.Error("sync: balances error", "user_id", userID, "exchange", c.cred.Exchange, "operation", "balances", "error", err)
		return err
	}
	s.processBalances(userID, balances, c.cred.Exchange)
	s.logger.Info("sync: balances ok", "user_id", userID, "exchange", c.cred.Exchange, "count", len(balances))

	if positions, err := ex.GetOpenPositions(c.creds); err == nil {
		s.processPositions(userID, positions, c.cred.Exchange)
		s.logger.Info("sync: positions ok", "user_id", userID, "exchange", c.cred.Exchange, "count", len(positions))
	} else {
		s.logger.Error("sync: positions error", "user_id", userID, "exchange", c.cred.Exchange, "error", err)
	}

	if trades, err := ex.GetClosedTrades(c.creds, startMs, now.UnixMilli()); err == nil {
		s.processHistory(userID, trades, c.cred.Exchange)
		s.logger.Info("sync: history ok", "user_id", userID, "exchange", c.cred.Exchange, "count", len(trades))
	} else {
		s.logger.Error("sync: history error", "user_id", userID, "exchange", c.cred.Exchange, "error", err)
	}

	// Spot trades (optional — only if exchange implements SpotTrader)
	if st, ok := ex.(exchange.SpotTrader); ok {
		if spotTrades, err := st.GetRecentTrades(c.creds, startMs, now.UnixMilli()); err == nil {
			s.processSpotTrades(userID, spotTrades, c.cred.Exchange)
		} else {
			s.logger.Error("sync: spot trades error", "user_id", userID, "exchange", c.cred.Exchange, "error", err)
		}
	}

	return nil
}

func (s *SyncService) processBalances(userID int64, balances []exchange.Balance, exchangeName string) {
	for _, b := range balances {
		assetID, err := s.repo.getOrCreateAsset(b.Symbol)
		if err != nil {
			s.logger.Error("sync: getOrCreateAsset", "user_id", userID, "exchange", exchangeName, "symbol", b.Symbol, "error", err)
			continue
		}
		if err := s.repo.upsertBalance(userID, assetID, exchangeName, b.Quantity); err != nil {
			s.logger.Error("sync: upsertBalance", "user_id", userID, "exchange", exchangeName, "symbol", b.Symbol, "error", err)
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
			s.logger.Error("sync: getOrCreateAsset", "user_id", userID, "exchange", exchangeName, "symbol", p.Symbol, "error", err)
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
			s.logger.Error("sync: upsertPosition", "user_id", userID, "exchange", exchangeName, "symbol", p.Symbol, "error", err)
		}
	}
}

func (s *SyncService) processHistory(userID int64, trades []exchange.ClosedTrade, exchangeName string) {
	inserted := 0
	for _, t := range trades {
		closedAt := time.UnixMilli(t.ClosedAt).UTC().Format("2006-01-02 15:04:05")

		var openedAt *string
		if t.OpenedAt > 0 {
			s := time.UnixMilli(t.OpenedAt).UTC().Format("2006-01-02 15:04:05")
			openedAt = &s
		}

		// lev=0 means OKX returned empty/zero — store "0x" so UI can show "—"
		// lev>0 means real leverage — store as "Nx"
		levStr := "0x"
		if t.Leverage > 0 {
			levStr = fmt.Sprintf("%dx", t.Leverage)
		}

		maxSize := t.NotionalUsd
		if maxSize == 0 {
			maxSize = t.Quantity * t.EntryPrice
		}

		row := historyRow{
			UserID:      userID,
			Symbol:      t.Symbol,
			Side:        t.Side,
			Leverage:    levStr,
			MarginMode:  "cross",
			EntryPrice:  t.EntryPrice,
			ExitPrice:   t.ClosePrice,
			Quantity:    t.Quantity,
			RealizedPnl: t.PnL,
			Fee:         t.Fee,
			MaxSize:     maxSize,
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

func (s *SyncService) processSpotTrades(userID int64, trades []exchange.SpotTrade, exchangeName string) {
	inserted := 0
	for _, t := range trades {
		row := &models.SpotTrade{
			UserID:   userID,
			Exchange: exchangeName,
			Symbol:   t.Symbol,
			Side:     t.Side,
			Quantity: t.Quantity,
			Price:    t.Price,
			Fee:      t.Fee,
			FeeAsset: t.FeeAsset,
			TradedAt: t.TradedAt,
		}
		if err := s.spotTradeRepo.Upsert(row); err == nil {
			inserted++
		}
	}
	s.logger.Info("sync: spot trades upserted", "exchange", exchangeName, "inserted", inserted, "total", len(trades))
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
