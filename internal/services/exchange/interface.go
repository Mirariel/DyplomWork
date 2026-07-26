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
	Side       string  // "LONG" | "SHORT"
	Quantity   float64
	EntryPrice float64
	MarkPrice  float64
	Leverage   int
	PnL        float64
	MarginType string  // "cross" | "isolated"
}

// ClosedTrade — закрита угода з PnL
type ClosedTrade struct {
	Symbol      string
	Side        string
	Quantity    float64
	EntryPrice  float64
	ClosePrice  float64
	PnL         float64
	Leverage    int     // кредитне плече (0 = невідомо / спот)
	Fee         float64 // комісія (абсолютне значення USDT)
	NotionalUsd float64 // повний розмір позиції в USD (з API; 0 = використати qty×entry)
	MarginMode  string  // "cross" | "isolated" (порожнє = невідомо, фолбек "cross")
	OpenedAt    int64   // Unix timestamp ms, 0 = невідомо
	ClosedAt    int64   // Unix timestamp ms
}

// SpotTrade — одна спот-угода (buy/sell)
type SpotTrade struct {
	Symbol    string
	Side      string  // "buy" | "sell"
	Quantity  float64
	Price     float64
	Fee       float64
	FeeAsset  string
	TradedAt  int64 // Unix timestamp ms
}

// SpotTrader — опціональний інтерфейс для отримання спот-угод.
// Перевіряти через type assertion: if st, ok := ex.(exchange.SpotTrader); ok { ... }
// Реалізовано: Binance, OKX, Bybit.
type SpotTrader interface {
	GetRecentTrades(creds Credentials, startMs, endMs int64) ([]SpotTrade, error)
}

// Exchange — інтерфейс, який мають реалізувати всі адаптери бірж
type Exchange interface {
	Name() string
	GetBalances(creds Credentials) ([]Balance, error)
	GetOpenPositions(creds Credentials) ([]Position, error)
	GetClosedTrades(creds Credentials, startMs, endMs int64) ([]ClosedTrade, error)
	GetPrices(symbols []string) (map[string]float64, error)
}
