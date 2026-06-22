package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// SpotTrade represents a synced spot trade from an exchange.
type SpotTrade struct {
	ID       int64     `db:"id"`
	UserID   int64     `db:"user_id"`
	Exchange string    `db:"exchange"`
	Symbol   string    `db:"symbol"`
	Side     string    `db:"side"`
	Quantity float64   `db:"quantity"`
	Price    float64   `db:"price"`
	Fee      float64   `db:"fee"`
	FeeAsset string    `db:"fee_asset"`
	TradedAt int64     `db:"traded_at"`
	// computed
	CreatedAt time.Time `db:"created_at,omitempty"`
}

// SpotTradeRepository provides DB operations for spot_trades.
type SpotTradeRepository struct {
	db *sqlx.DB
}

func NewSpotTradeRepository(db *sqlx.DB) *SpotTradeRepository {
	return &SpotTradeRepository{db: db}
}

// Upsert inserts a spot trade, ignoring duplicates (unique key on user/exchange/symbol/side/traded_at/quantity).
func (r *SpotTradeRepository) Upsert(t *SpotTrade) error {
	_, err := r.db.Exec(`
		INSERT IGNORE INTO spot_trades
			(user_id, exchange, symbol, side, quantity, price, fee, fee_asset, traded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.UserID, t.Exchange, t.Symbol, t.Side,
		t.Quantity, t.Price, t.Fee, t.FeeAsset, t.TradedAt,
	)
	return err
}

// GetByUser returns spot trades for a user, optionally filtered by exchange.
// Results are sorted newest first, limited to 500.
func (r *SpotTradeRepository) GetByUser(userID int64, exchange string) ([]SpotTrade, error) {
	var rows []SpotTrade
	if exchange != "" {
		err := r.db.Select(&rows, `
			SELECT id, user_id, exchange, symbol, side, quantity, price, fee, fee_asset, traded_at
			FROM spot_trades
			WHERE user_id = ? AND exchange = ?
			ORDER BY traded_at DESC LIMIT 500`,
			userID, exchange)
		return rows, err
	}
	err := r.db.Select(&rows, `
		SELECT id, user_id, exchange, symbol, side, quantity, price, fee, fee_asset, traded_at
		FROM spot_trades
		WHERE user_id = ?
		ORDER BY traded_at DESC LIMIT 500`,
		userID)
	return rows, err
}
