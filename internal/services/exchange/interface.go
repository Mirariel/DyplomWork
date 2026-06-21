package exchange

import "errors"

// ErrNoCredentials повертається коли не знайдено активних credentials для біржі.
var ErrNoCredentials = errors.New("no active credentials found for exchange")

// Credentials містить дані доступу до біржі
type Credentials struct {
	APIKey     string
	APISecret  string
	Passphrase string // тільки для OKX
}

// Balance — спот-баланс одного активу
type Balance struct {
	Symbol   string
	Quantity float64
}

// Position — відкрита ф'ючерсна/маржинальна позиція
type Position struct {
	Symbol     string
	Side       string  // "long" | "short"
	Quantity   float64
	EntryPrice float64
	MarkPrice  float64
	Leverage   int
	PnL        float64
}

// ClosedTrade — закрита угода з PnL
type ClosedTrade struct {
	Symbol     string
	Side       string
	Quantity   float64
	EntryPrice float64
	ClosePrice float64
	PnL        float64
	ClosedAt   int64 // Unix timestamp ms
}

// Exchange — інтерфейс, який мають реалізувати всі адаптери бірж
type Exchange interface {
	Name() string
	GetBalances(creds Credentials) ([]Balance, error)
	GetOpenPositions(creds Credentials) ([]Position, error)
	GetClosedTrades(creds Credentials, startMs, endMs int64) ([]ClosedTrade, error)
	GetPrices(symbols []string) (map[string]float64, error)
}
