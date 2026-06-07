// Package cache визначає інтерфейс для зберігання цін бірж.
//
// Поточна реалізація — in-memory (MemoryPriceStore).
// Щоб перейти на Redis достатньо написати RedisPriceStore що реалізує той самий PriceStorer —
// і замінити одну стрічку в main.go, не торкаючись PriceService.
package cache

import "time"

// PriceStorer зберігає і повертає ціни в розрізі бірж.
// Ключ — назва біржі ("binance", "okx", "bybit").
// Значення — map[symbol]price, наприклад {"BTCUSDT": 67420.5}.
type PriceStorer interface {
	// Get повертає закешовані ціни для біржі.
	// ok=false якщо кеш відсутній або протермінований.
	Get(exchange string) (prices map[string]float64, ok bool)

	// Set зберігає ціни для біржі з вказаним TTL.
	Set(exchange string, prices map[string]float64, ttl time.Duration)
}
