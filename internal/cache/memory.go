package cache

import (
	"sync"
	"time"
)

type memEntry struct {
	prices    map[string]float64
	expiresAt time.Time
}

// MemoryPriceStore — реалізація PriceStorer на базі sync.RWMutex.
// Потокобезпечна, не потребує зовнішніх залежностей.
// Підходить для single-instance деплою.
type MemoryPriceStore struct {
	mu      sync.RWMutex
	entries map[string]memEntry
}

func NewMemoryPriceStore() *MemoryPriceStore {
	return &MemoryPriceStore{
		entries: make(map[string]memEntry),
	}
}

func (m *MemoryPriceStore) Get(exchange string) (map[string]float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, ok := m.entries[exchange]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}

	// Повертаємо копію — caller не повинен мутувати кеш
	cp := make(map[string]float64, len(e.prices))
	for k, v := range e.prices {
		cp[k] = v
	}
	return cp, true
}

func (m *MemoryPriceStore) Set(exchange string, prices map[string]float64, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[exchange] = memEntry{
		prices:    prices,
		expiresAt: time.Now().Add(ttl),
	}
}
