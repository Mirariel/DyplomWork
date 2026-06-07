package cache

// RedisPriceStore — реалізація PriceStorer на базі Redis.
//
// Ціни зберігаються як JSON-blob за ключем "prices:{exchange}" з TTL.
// При недоступності Redis — повертає cache miss (ok=false), сервіс
// фолбекає на живий fetch з біржі. Жодного panic.
//
// Щоб увімкнути: замінити NewMemoryPriceStore() на NewRedisPriceStore(client) в main.go.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "prices:"

// RedisPriceStore реалізує PriceStorer поверх Redis.
type RedisPriceStore struct {
	client *redis.Client
}

// NewRedisPriceStore створює RedisPriceStore.
// client має бути попередньо ініціалізований через redis.NewClient().
func NewRedisPriceStore(client *redis.Client) *RedisPriceStore {
	return &RedisPriceStore{client: client}
}

func (r *RedisPriceStore) Get(exchange string) (map[string]float64, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	data, err := r.client.Get(ctx, keyPrefix+exchange).Bytes()
	if err != nil {
		// redis.Nil = cache miss, інші = помилка з'єднання — обидва = "немає в кеші"
		return nil, false
	}

	var prices map[string]float64
	if err := json.Unmarshal(data, &prices); err != nil {
		return nil, false
	}
	return prices, true
}

func (r *RedisPriceStore) Set(exchange string, prices map[string]float64, ttl time.Duration) {
	data, err := json.Marshal(prices)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Помилку ігноруємо — при недоступному Redis просто не кешуємо
	r.client.Set(ctx, keyPrefix+exchange, data, ttl)
}
