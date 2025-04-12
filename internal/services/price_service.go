package services

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
)

// priceCache — in-memory кеш цін із TTL
type priceCache struct {
	mu        sync.RWMutex
	prices    map[string]float64
	updatedAt time.Time
	ttl       time.Duration
}

func newPriceCache(ttl time.Duration) *priceCache {
	return &priceCache{
		prices: make(map[string]float64),
		ttl:    ttl,
	}
}

func (c *priceCache) get() (map[string]float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if time.Since(c.updatedAt) > c.ttl {
		return nil, false
	}
	// Повертаємо копію щоб не блокувати
	cp := make(map[string]float64, len(c.prices))
	for k, v := range c.prices {
		cp[k] = v
	}
	return cp, true
}

func (c *priceCache) set(prices map[string]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prices = prices
	c.updatedAt = time.Now()
}

// PriceService — отримує актуальні ціни з бірж і оновлює таблицю assets.
// Ціни кешуються в пам'яті на 30 секунд (паралельний fetch з усіх 3 бірж).
type PriceService struct {
	db        *sqlx.DB
	exchanges map[string]exchange.Exchange
	caches    map[string]*priceCache
	logger    *log.Logger
}

func NewPriceService(db *sqlx.DB, logger *log.Logger) *PriceService {
	exchanges := exchange.Registry()
	caches := make(map[string]*priceCache, len(exchanges))
	for name := range exchanges {
		caches[name] = newPriceCache(30 * time.Second)
	}
	return &PriceService{
		db:        db,
		exchanges: exchanges,
		caches:    caches,
		logger:    logger,
	}
}

// UpdatePrices — оновлює current_price для переданих символів.
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

		price := ps.resolvePrice(search, allPrices)
		if price > 0 {
			stmt.Exec(price, sym)
		}
	}
	return nil
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

// fetchAllPrices — паралельно отримує ціни з усіх бірж, кешує на 30 с.
// Повертає map[exchangeName]map[symbol]price
func (ps *PriceService) fetchAllPrices() map[string]map[string]float64 {
	type result struct {
		name   string
		prices map[string]float64
	}

	ch := make(chan result, len(ps.exchanges))
	var wg sync.WaitGroup

	for name, ex := range ps.exchanges {
		// Спочатку перевіряємо кеш
		if cached, ok := ps.caches[name].get(); ok {
			ch <- result{name: name, prices: cached}
			continue
		}

		wg.Add(1)
		go func(name string, ex exchange.Exchange) {
			defer wg.Done()
			prices, err := ex.GetPrices(nil)
			if err != nil {
				ps.logger.Printf("[prices] fetch error %s: %v", name, err)
				prices = make(map[string]float64)
			}
			ps.caches[name].set(prices)
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

// resolvePrice — пріоритет Binance → OKX → Bybit
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
