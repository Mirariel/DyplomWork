package services

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	topSymbolsKey   = "top_symbols"
	topSymbolsTTL   = 1 * time.Hour
	topSymbolsCount = 20
	binanceTickers  = "https://api.binance.com/api/v3/ticker/24hr"
)

// TopSymbolsService підтягує топ монети по 24h obsягу торгів з Binance.
// Зберігає список в Redis з TTL 1h; повертає закешований список через GetTopSymbols.
type TopSymbolsService struct {
	redis  *goredis.Client // nil → без Redis, GetTopSymbols поверне порожній список
	logger *slog.Logger
}

func NewTopSymbolsService(client *goredis.Client, logger *slog.Logger) *TopSymbolsService {
	return &TopSymbolsService{redis: client, logger: logger}
}

// FetchTopSymbols завантажує актуальні дані з Binance, зберігає в Redis і повертає список.
// При помилці мережі — логує і повертає поточний кеш (або порожній список).
func (s *TopSymbolsService) FetchTopSymbols() []string {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(binanceTickers)
	if err != nil {
		s.logger.Error("top_symbols: binance fetch failed", "error", err)
		return s.GetTopSymbols()
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("top_symbols: read body failed", "error", err)
		return s.GetTopSymbols()
	}

	type ticker struct {
		Symbol      string `json:"symbol"`
		QuoteVolume string `json:"quoteVolume"`
	}
	var tickers []ticker
	if err := json.Unmarshal(body, &tickers); err != nil {
		s.logger.Error("top_symbols: parse failed", "error", err)
		return s.GetTopSymbols()
	}

	// Фільтр: тільки USDT пари
	var usdt []ticker
	for _, t := range tickers {
		if strings.HasSuffix(t.Symbol, "USDT") {
			usdt = append(usdt, t)
		}
	}

	// Сортуємо по quoteVolume DESC
	sort.Slice(usdt, func(i, j int) bool {
		return parseTopFloat(usdt[i].QuoteVolume) > parseTopFloat(usdt[j].QuoteVolume)
	})

	// Беремо топ base assets (стрипаємо "USDT")
	top := make([]string, 0, topSymbolsCount)
	for _, t := range usdt {
		if len(top) >= topSymbolsCount {
			break
		}
		base := strings.TrimSuffix(t.Symbol, "USDT")
		if base != "" {
			top = append(top, base)
		}
	}

	// Зберігаємо в Redis
	if s.redis != nil {
		data, _ := json.Marshal(top)
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		if err := s.redis.Set(ctx, topSymbolsKey, data, topSymbolsTTL).Err(); err != nil {
			s.logger.Warn("top_symbols: redis store failed", "error", err)
		} else {
			s.logger.Info("top_symbols: updated in Redis", "count", len(top), "first", top[0])
		}
	}

	return top
}

// GetTopSymbols повертає закешований список символів з Redis.
// Повертає порожній slice якщо кеш відсутній або Redis недоступний.
func (s *TopSymbolsService) GetTopSymbols() []string {
	if s.redis == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	data, err := s.redis.Get(ctx, topSymbolsKey).Bytes()
	if err != nil {
		return nil
	}
	var symbols []string
	if err := json.Unmarshal(data, &symbols); err != nil {
		return nil
	}
	return symbols
}

func parseTopFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
