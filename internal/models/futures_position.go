package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// FuturesPosition — відкрита ф'ючерсна позиція, синхронізована з біржі.
// Зберігається в таблиці futures_positions (міграція 000008).
type FuturesPosition struct {
	ID            int64     `db:"id"             json:"id"`
	UserID        int64     `db:"user_id"        json:"user_id"`
	Exchange      string    `db:"exchange"       json:"exchange"`
	Symbol        string    `db:"symbol"         json:"symbol"`
	Side          string    `db:"side"           json:"side"`
	Size          float64   `db:"size"           json:"size"`
	EntryPrice    float64   `db:"entry_price"    json:"entry_price"`
	MarkPrice     float64   `db:"mark_price"     json:"mark_price"`
	UnrealizedPnl float64   `db:"unrealized_pnl" json:"unrealized_pnl"`
	Leverage      int       `db:"leverage"       json:"leverage"`
	MarginType    string    `db:"margin_type"    json:"margin_type"`
	SyncedAt      time.Time `db:"synced_at"      json:"synced_at"`
}

type FuturesPositionRepository struct {
	db *sqlx.DB
}

func NewFuturesPositionRepository(db *sqlx.DB) *FuturesPositionRepository {
	return &FuturesPositionRepository{db: db}
}

// Upsert вставляє або оновлює ф'ючерсну позицію (по user_id, exchange, symbol, side).
func (r *FuturesPositionRepository) Upsert(p *FuturesPosition) error {
	_, err := r.db.Exec(`
		INSERT INTO futures_positions
			(user_id, exchange, symbol, side, size, entry_price, mark_price, unrealized_pnl, leverage, margin_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			size           = VALUES(size),
			entry_price    = VALUES(entry_price),
			mark_price     = VALUES(mark_price),
			unrealized_pnl = VALUES(unrealized_pnl),
			leverage       = VALUES(leverage),
			margin_type    = VALUES(margin_type),
			synced_at      = CURRENT_TIMESTAMP
	`,
		p.UserID, p.Exchange, p.Symbol, p.Side,
		p.Size, p.EntryPrice, p.MarkPrice, p.UnrealizedPnl,
		p.Leverage, p.MarginType,
	)
	return err
}

// GetByUser повертає всі відкриті ф'ючерсні позиції користувача.
// Якщо exchange != "", фільтрує по біржі.
func (r *FuturesPositionRepository) GetByUser(userID int64, exchange string) ([]FuturesPosition, error) {
	var rows []FuturesPosition
	var err error
	if exchange != "" {
		err = r.db.Select(&rows,
			`SELECT * FROM futures_positions WHERE user_id = ? AND exchange = ? ORDER BY exchange, symbol`,
			userID, exchange,
		)
	} else {
		err = r.db.Select(&rows,
			`SELECT * FROM futures_positions WHERE user_id = ? ORDER BY exchange, symbol`,
			userID,
		)
	}
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []FuturesPosition{}
	}
	return rows, nil
}

// DeleteStale видаляє позиції, що не оновлювалися з часу syncedBefore
// (закриті позиції не повертаються Exchange і мають бути видалені).
func (r *FuturesPositionRepository) DeleteStale(userID int64, exchange string, syncedBefore time.Time) error {
	_, err := r.db.Exec(
		`DELETE FROM futures_positions WHERE user_id = ? AND exchange = ? AND synced_at < ?`,
		userID, exchange, syncedBefore,
	)
	return err
}
