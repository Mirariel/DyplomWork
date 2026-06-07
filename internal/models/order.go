package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// Order — запис ордеру в БД.
type Order struct {
	ID               int64      `db:"id"                 json:"id"`
	UserID           int64      `db:"user_id"            json:"-"`
	Exchange         string     `db:"exchange"           json:"exchange"`
	ExchangeOrderID  string     `db:"exchange_order_id"  json:"exchange_order_id"`
	Symbol           string     `db:"symbol"             json:"symbol"`
	Side             string     `db:"side"               json:"side"`
	Type             string     `db:"type"               json:"type"`
	Category         string     `db:"category"           json:"category"`
	Quantity         float64    `db:"quantity"           json:"quantity"`
	Price            float64    `db:"price"              json:"price"`
	FilledQty        float64    `db:"filled_qty"         json:"filled_qty"`
	AvgPrice         float64    `db:"avg_price"          json:"avg_price"`
	Status           string     `db:"status"             json:"status"`
	ErrorMessage     *string    `db:"error_message"      json:"error_message,omitempty"`
	CreatedAt        time.Time  `db:"created_at"         json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"         json:"updated_at"`
}

// OrderRepository — всі DB-операції для ордерів.
type OrderRepository struct {
	db *sqlx.DB
}

func NewOrderRepository(db *sqlx.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create зберігає новий ордер. Заповнює o.ID після вставки.
func (r *OrderRepository) Create(o *Order) error {
	res, err := r.db.Exec(`
		INSERT INTO orders
		    (user_id, exchange, exchange_order_id, symbol, side, type, category,
		     quantity, price, filled_qty, avg_price, status, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.UserID, o.Exchange, o.ExchangeOrderID, o.Symbol, o.Side, o.Type, o.Category,
		o.Quantity, o.Price, o.FilledQty, o.AvgPrice, o.Status, o.ErrorMessage,
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

// GetByID повертає ордер за ID що належить user.
func (r *OrderRepository) GetByID(id, userID int64) (*Order, error) {
	var o Order
	err := r.db.Get(&o,
		"SELECT * FROM orders WHERE id = ? AND user_id = ?", id, userID)
	return &o, err
}

// ListByUser повертає ордери користувача. status="" — всі, інакше фільтрує.
func (r *OrderRepository) ListByUser(userID int64, status string) ([]Order, error) {
	var orders []Order
	var err error
	if status != "" {
		err = r.db.Select(&orders,
			"SELECT * FROM orders WHERE user_id = ? AND status = ? ORDER BY created_at DESC LIMIT 100",
			userID, status)
	} else {
		err = r.db.Select(&orders,
			"SELECT * FROM orders WHERE user_id = ? ORDER BY created_at DESC LIMIT 100",
			userID)
	}
	return orders, err
}

// UpdateStatus оновлює статус, exchange_order_id та fill-дані після відповіді біржі.
func (r *OrderRepository) UpdateStatus(id int64, exchangeOrderID, status string, filledQty, avgPrice float64) error {
	_, err := r.db.Exec(`
		UPDATE orders
		SET exchange_order_id = ?, status = ?, filled_qty = ?, avg_price = ?
		WHERE id = ?`,
		exchangeOrderID, status, filledQty, avgPrice, id,
	)
	return err
}

// MarkFailed зберігає помилку розміщення.
func (r *OrderRepository) MarkFailed(id int64, errMsg string) error {
	_, err := r.db.Exec(
		"UPDATE orders SET status = 'rejected', error_message = ? WHERE id = ?",
		errMsg, id,
	)
	return err
}
