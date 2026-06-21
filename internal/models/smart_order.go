package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// SmartOrder — умовний ордер (SL/TP/Trailing Stop) що зберігається в БД.
// Перевіряється фоновим SmartOrderService кожні 5 секунд.
type SmartOrder struct {
	ID               int64      `db:"id"                  json:"id"`
	UserID           int64      `db:"user_id"             json:"-"`
	Exchange         string     `db:"exchange"            json:"exchange"`
	Symbol           string     `db:"symbol"              json:"symbol"`
	Side             string     `db:"side"                json:"side"`
	Category         string     `db:"category"            json:"category"`
	Quantity         float64    `db:"quantity"            json:"quantity"`
	Type             string     `db:"type"                json:"type"`
	TriggerPrice     float64    `db:"trigger_price"       json:"trigger_price"`
	CallbackRate     float64    `db:"callback_rate"       json:"callback_rate"`
	PeakPrice        float64    `db:"peak_price"          json:"peak_price"`
	Status           string     `db:"status"              json:"status"`
	TriggeredOrderID *int64     `db:"triggered_order_id"  json:"triggered_order_id,omitempty"`
	ErrorMessage     *string    `db:"error_message"       json:"error_message,omitempty"`
	CreatedAt        time.Time  `db:"created_at"          json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"          json:"updated_at"`
}

// SmartOrderRepository — всі DB-операції для умовних ордерів.
type SmartOrderRepository struct {
	db *sqlx.DB
}

func NewSmartOrderRepository(db *sqlx.DB) *SmartOrderRepository {
	return &SmartOrderRepository{db: db}
}

// Create зберігає новий умовний ордер. Заповнює o.ID після вставки.
func (r *SmartOrderRepository) Create(o *SmartOrder) error {
	res, err := r.db.Exec(`
		INSERT INTO smart_orders
		    (user_id, exchange, symbol, side, category, quantity,
		     type, trigger_price, callback_rate, peak_price, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
		o.UserID, o.Exchange, o.Symbol, o.Side, o.Category, o.Quantity,
		o.Type, o.TriggerPrice, o.CallbackRate, o.PeakPrice,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		o.ID = id
	}
	return nil
}

// ListActive повертає всі активні умовні ордери (для фонового сервісу).
func (r *SmartOrderRepository) ListActive() ([]SmartOrder, error) {
	var orders []SmartOrder
	err := r.db.Select(&orders,
		"SELECT * FROM smart_orders WHERE status = 'active' ORDER BY id")
	return orders, err
}

// ListByUser повертає умовні ордери конкретного користувача.
func (r *SmartOrderRepository) ListByUser(userID int64) ([]SmartOrder, error) {
	orders := make([]SmartOrder, 0)
	err := r.db.Select(&orders,
		"SELECT * FROM smart_orders WHERE user_id = ? ORDER BY created_at DESC LIMIT 100",
		userID)
	return orders, err
}

// GetByID повертає умовний ордер за ID що належить user.
func (r *SmartOrderRepository) GetByID(id, userID int64) (*SmartOrder, error) {
	var o SmartOrder
	err := r.db.Get(&o,
		"SELECT * FROM smart_orders WHERE id = ? AND user_id = ?", id, userID)
	return &o, err
}

// Cancel встановлює статус 'cancelled'.
func (r *SmartOrderRepository) Cancel(id, userID int64) error {
	_, err := r.db.Exec(
		"UPDATE smart_orders SET status = 'cancelled' WHERE id = ? AND user_id = ? AND status = 'active'",
		id, userID,
	)
	return err
}

// MarkTriggered оновлює статус на 'triggered' і зберігає ID ордеру що виконав умову.
func (r *SmartOrderRepository) MarkTriggered(id, orderID int64) error {
	_, err := r.db.Exec(
		"UPDATE smart_orders SET status = 'triggered', triggered_order_id = ? WHERE id = ?",
		orderID, id,
	)
	return err
}

// MarkFailed зберігає помилку виконання.
func (r *SmartOrderRepository) MarkFailed(id int64, errMsg string) error {
	_, err := r.db.Exec(
		"UPDATE smart_orders SET status = 'failed', error_message = ? WHERE id = ?",
		errMsg, id,
	)
	return err
}

// UpdatePeak оновлює peak_price для trailing stop ордерів.
func (r *SmartOrderRepository) UpdatePeak(id int64, peakPrice float64) error {
	_, err := r.db.Exec(
		"UPDATE smart_orders SET peak_price = ? WHERE id = ?",
		peakPrice, id,
	)
	return err
}
