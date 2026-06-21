package services

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// syncRepository — всі DB-операції потрібні для синку.
// Окремий тип щоб не змішувати з SyncService логікою.
type syncRepository struct {
	db *sqlx.DB
}

func newSyncRepository(db *sqlx.DB) *syncRepository {
	return &syncRepository{db: db}
}

// --- Assets ---

// getOrCreateAsset повертає asset_id, створюючи запис якщо його немає.
func (r *syncRepository) getOrCreateAsset(symbol string) (int64, error) {
	var id int64
	err := r.db.Get(&id, "SELECT id FROM assets WHERE symbol = ? LIMIT 1", symbol)
	if err == nil {
		return id, nil
	}
	res, err := r.db.Exec(
		"INSERT IGNORE INTO assets (symbol, name, current_price) VALUES (?, ?, 0)",
		symbol, symbol,
	)
	if err != nil {
		return 0, err
	}
	id, err = res.LastInsertId()
	if err != nil || id == 0 {
		// INSERT IGNORE не вставив (вже існує) — дістаємо знову
		err = r.db.Get(&id, "SELECT id FROM assets WHERE symbol = ? LIMIT 1", symbol)
	}
	return id, err
}

// --- Balances ---

// upsertBalance — upsert запис у user_portfolios.
func (r *syncRepository) upsertBalance(userID, assetID int64, exchange string, qty float64) error {
	_, err := r.db.Exec(`
		INSERT INTO user_portfolios (user_id, asset_id, exchange, quantity, updated_at)
		VALUES (?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE quantity = VALUES(quantity), updated_at = NOW()
	`, userID, assetID, exchange, qty)
	return err
}

// deleteStaleBalances — видаляє записи, які не оновились під час поточного синку.
func (r *syncRepository) deleteStaleBalances(userID int64, syncStart time.Time) error {
	_, err := r.db.Exec(
		"DELETE FROM user_portfolios WHERE user_id = ? AND updated_at < ?",
		userID, syncStart,
	)
	return err
}

// --- Positions ---

// upsertPosition — upsert відкритої позиції.
func (r *syncRepository) upsertPosition(p positionRow) error {
	_, err := r.db.NamedExec(`
		INSERT INTO open_positions
			(user_id, asset_id, symbol, exchange, side, margin_mode,
			 entry_price, mark_price, quantity, pnl, leverage,
			 liq_price, margin, trade_size, updated_at)
		VALUES
			(:user_id, :asset_id, :symbol, :exchange, :side, :margin_mode,
			 :entry_price, :mark_price, :quantity, :pnl, :leverage,
			 :liq_price, :margin, :trade_size, NOW())
		ON DUPLICATE KEY UPDATE
			asset_id    = VALUES(asset_id),
			margin_mode = VALUES(margin_mode),
			entry_price = VALUES(entry_price),
			mark_price  = VALUES(mark_price),
			quantity    = VALUES(quantity),
			pnl         = VALUES(pnl),
			leverage    = VALUES(leverage),
			liq_price   = VALUES(liq_price),
			margin      = VALUES(margin),
			trade_size  = VALUES(trade_size),
			updated_at  = NOW()
	`, p)
	return err
}

// deleteStalePositions — видаляє закриті позиції (не оновились під час синку).
func (r *syncRepository) deleteStalePositions(userID int64, syncStart time.Time) error {
	_, err := r.db.Exec(
		"DELETE FROM open_positions WHERE user_id = ? AND updated_at < ?",
		userID, syncStart,
	)
	return err
}

// transferCommentsToHistory — копіює коментарі із відкритих позицій (що закриваються)
// у відповідний найновіший запис в position_history.
func (r *syncRepository) transferCommentsToHistory(userID int64, syncStart time.Time) error {
	type stalePos struct {
		Symbol   string `db:"symbol"`
		Side     string `db:"side"`
		Exchange string `db:"exchange"`
		Comment  string `db:"comment"`
	}
	var stale []stalePos
	err := r.db.Select(&stale, `
		SELECT symbol, side, exchange, comment
		FROM open_positions
		WHERE user_id = ? AND updated_at < ? AND comment IS NOT NULL AND comment != ''
	`, userID, syncStart)
	if err != nil || len(stale) == 0 {
		return err
	}

	for _, p := range stale {
		if _, err := r.db.Exec(`
			UPDATE position_history
			SET comment = ?
			WHERE user_id = ? AND symbol = ? AND side = ? AND exchange = ?
			  AND (comment IS NULL OR comment = '')
			ORDER BY closed_at DESC
			LIMIT 1
		`, p.Comment, userID, p.Symbol, p.Side, p.Exchange); err != nil {
			return err
		}
	}
	return nil
}

// --- History ---

// insertHistory — INSERT IGNORE закритої угоди (UNIQUE KEY: user_id, symbol, closed_at, realized_pnl).
func (r *syncRepository) insertHistory(h historyRow) error {
	_, err := r.db.NamedExec(`
		INSERT IGNORE INTO position_history
			(user_id, symbol, side, leverage, margin_mode,
			 entry_price, exit_price, quantity, realized_pnl,
			 fee, max_size, opened_at, closed_at, exchange)
		VALUES
			(:user_id, :symbol, :side, :leverage, :margin_mode,
			 :entry_price, :exit_price, :quantity, :realized_pnl,
			 :fee, :max_size, :opened_at, :closed_at, :exchange)
	`, h)
	return err
}

// --- Transactions ---

// insertTransaction — INSERT IGNORE спот-угоди.
func (r *syncRepository) insertTransaction(userID, assetID int64, side string, qty, price, fee float64, exchange, tradeTime string) error {
	_, err := r.db.Exec(`
		INSERT IGNORE INTO transactions (user_id, asset_id, type, quantity, price, fee, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, assetID, side, qty, price, fee, "Imported from "+exchange, tradeTime)
	return err
}

// updateAverageBuyPrices — перераховує avg_buy_price для всіх активів користувача
// на основі транзакцій типу 'buy' (тільки якщо manually_set = 0).
func (r *syncRepository) updateAverageBuyPrices(userID int64) error {
	_, err := r.db.Exec(`
		UPDATE user_portfolios up
		SET up.avg_buy_price = (
			SELECT SUM(t.quantity * t.price) / SUM(t.quantity)
			FROM transactions t
			WHERE t.user_id = up.user_id
			  AND t.asset_id = up.asset_id
			  AND t.type = 'buy'
		)
		WHERE up.user_id = ?
		  AND up.manually_set = 0
		  AND EXISTS (
			  SELECT 1 FROM transactions t
			  WHERE t.user_id = up.user_id AND t.asset_id = up.asset_id AND t.type = 'buy'
		  )
	`, userID)
	return err
}

// --- Credentials ---

func (r *syncRepository) updateSyncStatus(credID int64, syncErr *string) error {
	_, err := r.db.Exec(
		"UPDATE external_api_credentials SET last_sync_at = NOW(), last_sync_error = ? WHERE id = ?",
		syncErr, credID,
	)
	return err
}

// --- Row types ---

type positionRow struct {
	UserID     int64   `db:"user_id"`
	AssetID    int64   `db:"asset_id"`
	Symbol     string  `db:"symbol"`
	Exchange   string  `db:"exchange"`
	Side       string  `db:"side"`
	MarginMode string  `db:"margin_mode"`
	EntryPrice float64 `db:"entry_price"`
	MarkPrice  float64 `db:"mark_price"`
	Quantity   float64 `db:"quantity"`
	PnL        float64 `db:"pnl"`
	Leverage   string  `db:"leverage"`
	LiqPrice   float64 `db:"liq_price"`
	Margin     float64 `db:"margin"`
	TradeSize  float64 `db:"trade_size"`
}

type historyRow struct {
	UserID      int64    `db:"user_id"`
	Symbol      string   `db:"symbol"`
	Side        string   `db:"side"`
	Leverage    string   `db:"leverage"`
	MarginMode  string   `db:"margin_mode"`
	EntryPrice  float64  `db:"entry_price"`
	ExitPrice   float64  `db:"exit_price"`
	Quantity    float64  `db:"quantity"`
	RealizedPnl float64  `db:"realized_pnl"`
	Fee         float64  `db:"fee"`
	MaxSize     float64  `db:"max_size"`
	OpenedAt    *string  `db:"opened_at"`
	ClosedAt    string   `db:"closed_at"`
	Exchange    string   `db:"exchange"`
}
