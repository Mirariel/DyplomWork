package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// Bot — конфігурація та стан grid-бота.
type Bot struct {
	ID           int64     `db:"id"            json:"id"`
	UserID       int64     `db:"user_id"       json:"-"`
	Exchange     string    `db:"exchange"      json:"exchange"`
	Symbol       string    `db:"symbol"        json:"symbol"`
	Category     string    `db:"category"      json:"category"`
	PriceLow     float64   `db:"price_low"     json:"price_low"`
	PriceHigh    float64   `db:"price_high"    json:"price_high"`
	Grids        int       `db:"grids"         json:"grids"`
	QtyPerGrid   float64   `db:"qty_per_grid"  json:"qty_per_grid"`
	Status       string    `db:"status"        json:"status"`
	TotalProfit  float64   `db:"total_profit"  json:"total_profit"`
	ErrorMessage *string   `db:"error_message" json:"error_message,omitempty"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// BotGrid — один рівень цінової сітки бота.
type BotGrid struct {
	ID              int64      `db:"id"                json:"id"`
	BotID           int64      `db:"bot_id"            json:"bot_id"`
	Level           int        `db:"level"             json:"level"`
	Price           float64    `db:"price"             json:"price"`
	Side            string     `db:"side"              json:"side"`
	ExchangeOrderID *string    `db:"exchange_order_id" json:"exchange_order_id,omitempty"`
	Status          string     `db:"status"            json:"status"`
	FilledAt        *time.Time `db:"filled_at"         json:"filled_at,omitempty"`
	CreatedAt       time.Time  `db:"created_at"        json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"        json:"updated_at"`
}

// BotRepository — DB-операції для ботів.
type BotRepository struct {
	db *sqlx.DB
}

func NewBotRepository(db *sqlx.DB) *BotRepository {
	return &BotRepository{db: db}
}

// Create зберігає нового бота.
func (r *BotRepository) Create(b *Bot) error {
	res, err := r.db.Exec(`
		INSERT INTO bots (user_id, exchange, symbol, category, price_low, price_high, grids, qty_per_grid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.UserID, b.Exchange, b.Symbol, b.Category,
		b.PriceLow, b.PriceHigh, b.Grids, b.QtyPerGrid,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		b.ID = id
	}
	return nil
}

// GetByID повертає бота що належить user.
func (r *BotRepository) GetByID(id, userID int64) (*Bot, error) {
	var b Bot
	err := r.db.Get(&b, "SELECT * FROM bots WHERE id = ? AND user_id = ?", id, userID)
	return &b, err
}

// ListByUser повертає всі боти користувача.
func (r *BotRepository) ListByUser(userID int64) ([]Bot, error) {
	bots := make([]Bot, 0)
	err := r.db.Select(&bots,
		"SELECT * FROM bots WHERE user_id = ? ORDER BY created_at DESC", userID)
	return bots, err
}

// ListRunning повертає всі запущені боти (для фонового сервісу).
func (r *BotRepository) ListRunning() ([]Bot, error) {
	var bots []Bot
	err := r.db.Select(&bots, "SELECT * FROM bots WHERE status = 'running'")
	return bots, err
}

// SetStatus оновлює статус бота.
func (r *BotRepository) SetStatus(id int64, status string, errMsg *string) error {
	_, err := r.db.Exec(
		"UPDATE bots SET status = ?, error_message = ? WHERE id = ?",
		status, errMsg, id,
	)
	return err
}

// AddProfit додає суму до total_profit.
func (r *BotRepository) AddProfit(id int64, profit float64) error {
	_, err := r.db.Exec(
		"UPDATE bots SET total_profit = total_profit + ? WHERE id = ?",
		profit, id,
	)
	return err
}

// CreateGrids зберігає рівні сітки (batch insert).
func (r *BotRepository) CreateGrids(grids []BotGrid) error {
	for i := range grids {
		res, err := r.db.Exec(`
			INSERT INTO bot_grids (bot_id, level, price, side, exchange_order_id, status)
			VALUES (?, ?, ?, ?, ?, ?)`,
			grids[i].BotID, grids[i].Level, grids[i].Price,
			grids[i].Side, grids[i].ExchangeOrderID, grids[i].Status,
		)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err == nil && id > 0 {
			grids[i].ID = id
		}
	}
	return nil
}

// ListOpenGrids повертає рівні сітки зі статусом 'open'.
func (r *BotRepository) ListOpenGrids(botID int64) ([]BotGrid, error) {
	grids := make([]BotGrid, 0)
	err := r.db.Select(&grids,
		"SELECT * FROM bot_grids WHERE bot_id = ? AND status = 'open' ORDER BY level",
		botID)
	return grids, err
}

// ListAllGrids повертає всі рівні сітки бота.
func (r *BotRepository) ListAllGrids(botID int64) ([]BotGrid, error) {
	grids := make([]BotGrid, 0)
	err := r.db.Select(&grids,
		"SELECT * FROM bot_grids WHERE bot_id = ? ORDER BY level",
		botID)
	return grids, err
}

// UpdateGrid оновлює стан рівня сітки.
func (r *BotRepository) UpdateGrid(id int64, exchangeOrderID, status string) error {
	_, err := r.db.Exec(`
		UPDATE bot_grids SET exchange_order_id = ?, status = ?,
		filled_at = IF(? = 'filled', NOW(), filled_at)
		WHERE id = ?`,
		exchangeOrderID, status, status, id,
	)
	return err
}

// CancelAllGrids скасовує всі open рівні сітки.
func (r *BotRepository) CancelAllGrids(botID int64) error {
	_, err := r.db.Exec(
		"UPDATE bot_grids SET status = 'cancelled' WHERE bot_id = ? AND status IN ('pending','open')",
		botID,
	)
	return err
}
