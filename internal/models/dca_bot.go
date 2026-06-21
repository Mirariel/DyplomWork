package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// DCABot — Dollar Cost Averaging бот.
// Купує фіксовану суму USD на вказаний символ кожні interval_hours годин.
type DCABot struct {
	ID            int64      `db:"id"             json:"id"`
	UserID        int64      `db:"user_id"        json:"-"`
	Exchange      string     `db:"exchange"       json:"exchange"`
	Symbol        string     `db:"symbol"         json:"symbol"`
	Category      string     `db:"category"       json:"category"`
	AmountUSD     float64    `db:"amount_usd"     json:"amount_usd"`
	IntervalHours int        `db:"interval_hours" json:"interval_hours"`
	NextBuyAt     time.Time  `db:"next_buy_at"    json:"next_buy_at"`
	Status        string     `db:"status"         json:"status"`
	TotalInvested float64    `db:"total_invested" json:"total_invested"`
	TotalQuantity float64    `db:"total_quantity" json:"total_quantity"`
	OrdersPlaced  int        `db:"orders_placed"  json:"orders_placed"`
	ErrorMessage  *string    `db:"error_message"  json:"error_message,omitempty"`
	CreatedAt     time.Time  `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"     json:"updated_at"`
}

// DCABotRepository — DB-операції для DCA ботів.
type DCABotRepository struct {
	db *sqlx.DB
}

func NewDCABotRepository(db *sqlx.DB) *DCABotRepository {
	return &DCABotRepository{db: db}
}

// Create зберігає нового DCA бота.
func (r *DCABotRepository) Create(b *DCABot) error {
	res, err := r.db.Exec(`
		INSERT INTO dca_bots
		    (user_id, exchange, symbol, category, amount_usd, interval_hours, next_buy_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.UserID, b.Exchange, b.Symbol, b.Category,
		b.AmountUSD, b.IntervalHours, b.NextBuyAt,
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

// GetByID повертає DCA бота що належить user.
func (r *DCABotRepository) GetByID(id, userID int64) (*DCABot, error) {
	var b DCABot
	err := r.db.Get(&b, "SELECT * FROM dca_bots WHERE id = ? AND user_id = ?", id, userID)
	return &b, err
}

// ListByUser повертає всі DCA боти користувача.
func (r *DCABotRepository) ListByUser(userID int64) ([]DCABot, error) {
	bots := make([]DCABot, 0)
	err := r.db.Select(&bots,
		"SELECT * FROM dca_bots WHERE user_id = ? ORDER BY created_at DESC", userID)
	return bots, err
}

// ListDue повертає всі запущені боти у яких час наступної покупки вже настав.
func (r *DCABotRepository) ListDue() ([]DCABot, error) {
	var bots []DCABot
	err := r.db.Select(&bots,
		"SELECT * FROM dca_bots WHERE status = 'running' AND next_buy_at <= NOW()")
	return bots, err
}

// SetStatus оновлює статус бота.
func (r *DCABotRepository) SetStatus(id int64, status string, errMsg *string) error {
	_, err := r.db.Exec(
		"UPDATE dca_bots SET status = ?, error_message = ? WHERE id = ?",
		status, errMsg, id,
	)
	return err
}

// RecordBuy оновлює статистику після успішної покупки і планує наступну.
func (r *DCABotRepository) RecordBuy(id int64, qty, amountUSD float64, nextBuyAt time.Time) error {
	_, err := r.db.Exec(`
		UPDATE dca_bots
		SET total_invested = total_invested + ?,
		    total_quantity = total_quantity + ?,
		    orders_placed  = orders_placed + 1,
		    next_buy_at    = ?,
		    error_message  = NULL
		WHERE id = ?`,
		amountUSD, qty, nextBuyAt, id,
	)
	return err
}
