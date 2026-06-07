package services

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/okochadmytro/tradetracker/internal/cache"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
)

const priceTTL = 30 * time.Second

// PriceService — отримує актуальні ціни з бірж і оновлює таблицю assets.
// Ціни кешуються через cache.PriceStorer (memory або Redis — залежить від DI).
type PriceService struct {
	db        *sqlx.DB
	exchanges map[string]exchange.Exchange
	cache     cache.PriceStorer
	logger    *slog.Logger
}

func NewPriceService(db *sqlx.DB, store cache.PriceStorer, logger *slog.Logger) *PriceService {
	return &PriceService{
		db:        db,
		exchanges: exchange.Registry(),
		cache:     store,
		logger:    logger,
	}
}

// UpdatePrices оновлює current_price для переданих символів.
// Ціни беруться паралельно з усіх бірж, пріоритет: Binance → OKX → Bybit.
func (ps *PriceService) UpdatePrices(symbols []string) error {
	if len(symbols) == 0 {
		return nil
	}

	allPrices := ps.fetchAllPrices()

	stmt, err := ps.db.Prepare(
		"UPDATE assets SET current_price = ?, price_updated_at = NOW() WHERE symbol = ?",
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	stables := map[string]bool{
		"USDT": true, "USDC": true, "DAI": true, "FDUSD": true, "BUSD": true, "USD": true,
	}

	for _, sym := range symbols {
		search := strings.ReplaceAll(sym, "-", "")
		if stables[search] {
			stmt.Exec(1.0, sym)
			continue
		}
		if price := ps.resolvePrice(search, allPrices); price > 0 {
			stmt.Exec(price, sym)
		}
	}
	return nil
}

// UpdateAllAssets оновлює ціни для всіх символів що є в таблиці assets.
// Використовується фоновим scheduler'ом.
func (ps *PriceService) UpdateAllAssets() error {
	var symbols []string
	if err := ps.db.Select(&symbols, "SELECT DISTINCT symbol FROM assets"); err != nil {
		return err
	}
	return ps.UpdatePrices(symbols)
}

// GetLivePrices повертає поточні ціни для набору символів без запису в БД.
// Використовується для WebSocket real-time оновлень.
func (ps *PriceService) GetLivePrices(symbols []string) map[string]float64 {
	allPrices := ps.fetchAllPrices()
	result := make(map[string]float64, len(symbols))

	stables := map[string]bool{
		"USDT": true, "USDC": true, "DAI": true, "FDUSD": true, "BUSD": true,
	}

	for _, sym := range symbols {
		search := strings.ReplaceAll(sym, "-", "")
		if stables[search] {
			result[sym] = 1.0
			continue
		}
		if p := ps.resolvePrice(search, allPrices); p > 0 {
			result[sym] = p
		}
	}
	return result
}

// fetchAllPrices — паралельно отримує ціни з усіх бірж через cache.PriceStorer.
// Повертає map[exchangeName]map[symbol]price.
func (ps *PriceService) fetchAllPrices() map[string]map[string]float64 {
	type result struct {
		name   string
		prices map[string]float64
	}

	ch := make(chan result, len(ps.exchanges))
	var wg sync.WaitGroup

	for name, ex := range ps.exchanges {
		if cached, ok := ps.cache.Get(name); ok {
			ch <- result{name: name, prices: cached}
			continue
		}

		wg.Add(1)
		go func(name string, ex exchange.Exchange) {
			defer wg.Done()
			prices, err := ex.GetPrices(nil)
			if err != nil {
				ps.logger.Error("prices: fetch error", "exchange", name, "error", err)
				prices = make(map[string]float64)
			}
			ps.cache.Set(name, prices, priceTTL)
			ch <- result{name: name, prices: prices}
		}(name, ex)
	}

	wg.Wait()
	close(ch)

	all := make(map[string]map[string]float64)
	for r := range ch {
		all[r.name] = r.prices
	}
	return all
}

// resolvePrice — пріоритет Binance → OKX → Bybit.
func (ps *PriceService) resolvePrice(symbol string, all map[string]map[string]float64) float64 {
	priority := []string{"binance", "okx", "bybit"}
	for _, name := range priority {
		prices := all[name]
		if p, ok := prices[symbol+"USDT"]; ok && p > 0 {
			return p
		}
		if p, ok := prices[symbol]; ok && p > 0 {
			return p
		}
	}
	return 0
}
