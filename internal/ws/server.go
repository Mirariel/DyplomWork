package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
)

// TopSymbolsGetter надає список топ символів (by volume) для пріоритизації в WS broadcast.
type TopSymbolsGetter interface {
	GetTopSymbols() []string
}

// UpdateMessage — повідомлення, яке сервер відправляє клієнту кожні 2с
type UpdateMessage struct {
	Type                 string                        `json:"type"`
	Positions            []LivePosition                `json:"positions"`
	SpotPrices           map[string]float64            `json:"spot_prices"`
	SpotPricesByExchange map[string]map[string]float64 `json:"spot_prices_by_exchange"`
	TopSymbols           []string                      `json:"top_symbols"`
}

// LivePosition — відкрита позиція з live PnL%
type LivePosition struct {
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	Exchange   string  `json:"exchange"`
	MarginMode string  `json:"margin_mode"`
	Leverage   string  `json:"leverage"`
	EntryPrice float64 `json:"entry_price"`
	MarkPrice  float64 `json:"mark_price"`
	LiqPrice   float64 `json:"liq_price"`
	Quantity   float64 `json:"quantity"`
	TradeSize  float64 `json:"trade_size"`
	Margin     float64 `json:"margin"`
	PnL        float64 `json:"pnl"`
	PnLPct     float64 `json:"pnl_pct"`
}

// Server — WebSocket broadcast сервер.
// Кожні 2с отримує live-дані і відправляє кожному авторизованому користувачу.
type Server struct {
	hub        *Hub
	db         *sqlx.DB
	encryption *services.EncryptionService
	prices     *services.PriceService
	topSymbols TopSymbolsGetter // опціонально; nil → top_symbols буде порожнім
	exchanges  map[string]exchange.Exchange
	jwtSecret  string
	logger     *slog.Logger
}

func NewServer(
	hub *Hub,
	db *sqlx.DB,
	enc *services.EncryptionService,
	prices *services.PriceService,
	topSymbols TopSymbolsGetter,
	jwtSecret string,
	logger *slog.Logger,
) *Server {
	return &Server{
		hub:        hub,
		db:         db,
		encryption: enc,
		prices:     prices,
		topSymbols: topSymbols,
		exchanges:  exchange.Registry(),
		jwtSecret:  jwtSecret,
		logger:     logger,
	}
}

// Run запускає головний broadcast цикл (викликати як goroutine).
// Зупиняється коли ctx скасовано.
func (s *Server) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	s.logger.Info("ws: broadcast loop started")

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("ws: broadcast loop stopped")
			return
		case <-ticker.C:
			users := s.hub.authenticatedUsers()
			if len(users) == 0 {
				continue
			}

			spotPricesByExchange := s.prices.GetLivePricesByExchange()
			// Merge all exchange prices into a flat map (first-wins per symbol)
			spotPrices := make(map[string]float64)
			for _, prices := range spotPricesByExchange {
				for sym, price := range prices {
					if _, exists := spotPrices[sym]; !exists {
						spotPrices[sym] = price
					}
				}
			}

			// Топ символи по об'єму (однакові для всіх користувачів)
			var topSymbols []string
			if s.topSymbols != nil {
				topSymbols = s.topSymbols.GetTopSymbols()
			}

			var wg sync.WaitGroup
			for userID, clients := range users {
				wg.Add(1)
				go func(userID int64, clients []*Client) {
					defer wg.Done()
					s.broadcastToUser(userID, spotPrices, spotPricesByExchange, topSymbols)
				}(userID, clients)
			}
			wg.Wait()
		}
	}
}

func (s *Server) broadcastToUser(userID int64, spotPrices map[string]float64, spotPricesByExchange map[string]map[string]float64, topSymbols []string) {
	creds, err := s.getUserCredentials(userID)
	if err != nil || len(creds) == 0 {
		return
	}

	// Паралельно отримуємо позиції з кожної біржі
	type result struct {
		positions []LivePosition
	}
	ch := make(chan result, len(creds))

	var wg sync.WaitGroup
	for _, c := range creds {
		wg.Add(1)
		go func(c credKeys) {
			defer wg.Done()
			ex, ok := s.exchanges[c.exchange]
			if !ok {
				return
			}
			rawPositions, err := ex.GetOpenPositions(exchange.Credentials{
				APIKey:     c.apiKey,
				APISecret:  c.apiSecret,
				Passphrase: c.passphrase,
			})
			if err != nil {
				s.logger.Error("ws: positions error", "user_id", userID, "exchange", c.exchange, "error", err)
				return
			}

			var live []LivePosition
			for _, p := range rawPositions {
				// PnL% = (markPrice - entryPrice) / entryPrice × leverage
				// Формула через ціни не залежить від розміру контракту (contracts vs units),
				// тому коректна і для linear (USDT-M), і для inverse (coin-margined) позицій.
				pnlPct := 0.0
				if p.EntryPrice > 0 && p.MarkPrice > 0 {
					priceChange := (p.MarkPrice - p.EntryPrice) / p.EntryPrice
					if strings.EqualFold(p.Side, "short") || strings.EqualFold(p.Side, "sell") {
						priceChange = -priceChange
					}
					lev := p.Leverage
					if lev < 1 {
						lev = 1
					}
					pnlPct = priceChange * float64(lev) * 100
				}
				live = append(live, LivePosition{
					Symbol:     p.Symbol,
					Side:       p.Side,
					Exchange:   c.exchange,
					MarginMode: "cross",
					Leverage:   formatLeverage(p.Leverage),
					EntryPrice: p.EntryPrice,
					MarkPrice:  p.MarkPrice,
					Quantity:   p.Quantity,
					PnL:        p.PnL,
					PnLPct:     roundFloat(pnlPct, 2),
				})
			}
			ch <- result{positions: live}
		}(c)
	}
	wg.Wait()
	close(ch)

	var allPositions []LivePosition
	for r := range ch {
		allPositions = append(allPositions, r.positions...)
	}

	msg := UpdateMessage{
		Type:                 "update",
		Positions:            allPositions,
		SpotPrices:           spotPrices,
		SpotPricesByExchange: spotPricesByExchange,
		TopSymbols:           topSymbols,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.hub.sendToUser(userID, data)
}

// --- helpers ---

type credKeys struct {
	exchange   string
	apiKey     string
	apiSecret  string
	passphrase string
}

func (s *Server) getUserCredentials(userID int64) ([]credKeys, error) {
	type row struct {
		ID                  int64   `db:"id"`
		Exchange            string  `db:"exchange"`
		ApiKeyEncrypted     string  `db:"api_key_encrypted"`
		ApiSecretEncrypted  string  `db:"api_secret_encrypted"`
		PassphraseEncrypted *string `db:"passphrase_encrypted"`
	}
	var rows []row
	err := s.db.Select(&rows,
		"SELECT id, exchange, api_key_encrypted, api_secret_encrypted, passphrase_encrypted FROM external_api_credentials WHERE user_id = ? AND is_active = 1",
		userID,
	)
	if err != nil {
		return nil, err
	}

	result := make([]credKeys, 0, len(rows))
	for _, r := range rows {
		apiKey, err := s.encryption.Decrypt(r.ApiKeyEncrypted)
		if err != nil {
			continue
		}
		apiSecret, err := s.encryption.Decrypt(r.ApiSecretEncrypted)
		if err != nil {
			continue
		}
		passphrase := ""
		if r.PassphraseEncrypted != nil {
			passphrase, _ = s.encryption.Decrypt(*r.PassphraseEncrypted)
		}
		result = append(result, credKeys{
			exchange:   r.Exchange,
			apiKey:     apiKey,
			apiSecret:  apiSecret,
			passphrase: passphrase,
		})
	}
	return result, nil
}

func formatLeverage(lev int) string {
	if lev <= 0 {
		return "1x"
	}
	return fmt.Sprintf("%dx", lev)
}

func roundFloat(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}
