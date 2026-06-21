package services

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
)

// BotService керує grid-ботами.
// Start — ініціалізує сітку і виставляє ордери.
// Stop — скасовує всі відкриті ордери.
// CheckBots — перевіряє виконання ордерів і поновлює сітку (викликати кожні 10с).
type BotService struct {
	bots      *models.BotRepository
	portfolio *models.PortfolioRepository
	enc       *EncryptionService
	exchanges map[string]exchange.Exchange
	logger    *slog.Logger
}

func NewBotService(
	bots *models.BotRepository,
	portfolio *models.PortfolioRepository,
	enc *EncryptionService,
	logger *slog.Logger,
) *BotService {
	return &BotService{
		bots:      bots,
		portfolio: portfolio,
		enc:       enc,
		exchanges: exchange.Registry(),
		logger:    logger,
	}
}

// Start запускає бота: розраховує сітку, виставляє ордери, ставить статус running.
// Потокобезпечно — кожен виклик ізольований по botID.
func (s *BotService) Start(botID, userID int64, currentPrice float64) error {
	bot, err := s.bots.GetByID(botID, userID)
	if err != nil {
		return fmt.Errorf("bot not found: %w", err)
	}
	if bot.Status == "running" {
		return fmt.Errorf("bot is already running")
	}
	if bot.PriceLow >= bot.PriceHigh {
		return fmt.Errorf("price_low must be less than price_high")
	}
	if bot.Grids < 2 {
		return fmt.Errorf("grids must be at least 2")
	}

	creds, err := s.getUserCreds(userID, bot.Exchange)
	if err != nil {
		return fmt.Errorf("no credentials for %s: %w", bot.Exchange, err)
	}

	ex, ok := s.exchanges[bot.Exchange]
	if !ok {
		return fmt.Errorf("unknown exchange: %s", bot.Exchange)
	}
	trader, ok := ex.(exchange.Trader)
	if !ok {
		return fmt.Errorf("%s does not support trading", bot.Exchange)
	}

	// Очищаємо старі рівні якщо бот перезапускається
	_ = s.bots.CancelAllGrids(botID)

	// Розраховуємо рівні сітки
	levels := s.calcLevels(bot.PriceLow, bot.PriceHigh, bot.Grids)
	gridSpacing := (bot.PriceHigh - bot.PriceLow) / float64(bot.Grids)

	grids := make([]models.BotGrid, 0, len(levels))
	for i, price := range levels {
		side := "buy"
		if price >= currentPrice {
			side = "sell"
		}
		grids = append(grids, models.BotGrid{
			BotID:  botID,
			Level:  i,
			Price:  price,
			Side:   side,
			Status: "pending",
		})
	}

	if err := s.bots.CreateGrids(grids); err != nil {
		return fmt.Errorf("create grids: %w", err)
	}

	// Виставляємо ордери
	placed := 0
	for i := range grids {
		g := &grids[i]
		// Пропускаємо рівень що співпадає з поточною ціною
		if math.Abs(g.Price-currentPrice) < gridSpacing*0.01 {
			continue
		}

		result, err := trader.PlaceOrder(creds, exchange.PlaceOrderRequest{
			Symbol:   bot.Symbol,
			Side:     exchange.OrderSide(g.Side),
			Type:     exchange.OrderTypeLimit,
			Category: exchange.OrderCategory(bot.Category),
			Quantity: bot.QtyPerGrid,
			Price:    g.Price,
		})
		if err != nil {
			s.logger.Error("bot: place order failed",
				"bot_id", botID, "level", g.Level, "price", g.Price, "error", err)
			continue
		}

		orderID := result.OrderID
		_ = s.bots.UpdateGrid(g.ID, orderID, "open")
		g.ExchangeOrderID = &orderID
		g.Status = "open"
		placed++
	}

	s.logger.Info("bot: started", "bot_id", botID, "levels", len(levels), "placed", placed)

	return s.bots.SetStatus(botID, "running", nil)
}

// Stop скасовує всі відкриті ордери і зупиняє бота.
func (s *BotService) Stop(botID, userID int64) error {
	bot, err := s.bots.GetByID(botID, userID)
	if err != nil {
		return fmt.Errorf("bot not found: %w", err)
	}
	if bot.Status != "running" {
		return fmt.Errorf("bot is not running")
	}

	creds, err := s.getUserCreds(userID, bot.Exchange)
	if err != nil {
		// Навіть без credentials — зупиняємо бота в БД
		s.logger.Warn("bot: stop without credentials", "bot_id", botID, "error", err)
		_ = s.bots.CancelAllGrids(botID)
		return s.bots.SetStatus(botID, "stopped", nil)
	}

	ex, ok := s.exchanges[bot.Exchange]
	if !ok {
		_ = s.bots.CancelAllGrids(botID)
		return s.bots.SetStatus(botID, "stopped", nil)
	}
	trader, ok := ex.(exchange.Trader)
	if !ok {
		_ = s.bots.CancelAllGrids(botID)
		return s.bots.SetStatus(botID, "stopped", nil)
	}

	openGrids, err := s.bots.ListOpenGrids(botID)
	if err == nil {
		for _, g := range openGrids {
			if g.ExchangeOrderID == nil {
				continue
			}
			err := trader.CancelOrder(creds, exchange.CancelOrderRequest{
				OrderID:  *g.ExchangeOrderID,
				Symbol:   bot.Symbol,
				Category: exchange.OrderCategory(bot.Category),
			})
			if err != nil {
				s.logger.Warn("bot: cancel order failed",
					"bot_id", botID, "order_id", *g.ExchangeOrderID, "error", err)
			}
		}
	}

	_ = s.bots.CancelAllGrids(botID)
	s.logger.Info("bot: stopped", "bot_id", botID)
	return s.bots.SetStatus(botID, "stopped", nil)
}

// CheckBots перевіряє всі запущені боти і оновлює сітку при виконанні ордерів.
// Викликати кожні 10с через scheduler.
func (s *BotService) CheckBots(ctx context.Context) {
	running, err := s.bots.ListRunning()
	if err != nil || len(running) == 0 {
		return
	}

	for _, bot := range running {
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.checkBot(bot)
	}
}

func (s *BotService) checkBot(bot models.Bot) {
	creds, err := s.getUserCreds(bot.UserID, bot.Exchange)
	if err != nil {
		return
	}
	ex, ok := s.exchanges[bot.Exchange]
	if !ok {
		return
	}
	trader, ok := ex.(exchange.Trader)
	if !ok {
		return
	}

	openGrids, err := s.bots.ListOpenGrids(bot.ID)
	if err != nil || len(openGrids) == 0 {
		return
	}

	// Розраховуємо крок сітки для визначення протилежного рівня
	gridSpacing := (bot.PriceHigh - bot.PriceLow) / float64(bot.Grids)

	for _, g := range openGrids {
		if g.ExchangeOrderID == nil {
			continue
		}

		live, err := trader.GetOrderStatus(creds, exchange.CancelOrderRequest{
			OrderID:  *g.ExchangeOrderID,
			Symbol:   bot.Symbol,
			Category: exchange.OrderCategory(bot.Category),
		})
		if err != nil {
			s.logger.Debug("bot: get order status failed",
				"bot_id", bot.ID, "order_id", *g.ExchangeOrderID, "error", err)
			continue
		}

		if live.Status != exchange.OrderStatusFilled {
			continue
		}

		// Ордер виконано — оновлюємо БД
		_ = s.bots.UpdateGrid(g.ID, *g.ExchangeOrderID, "filled")

		// Рахуємо profit від round-trip (одна сторона = половина спреду)
		profit := gridSpacing * bot.QtyPerGrid
		_ = s.bots.AddProfit(bot.ID, profit)

		// Виставляємо протилежний ордер на сусідньому рівні
		s.placeCounterOrder(bot, g, creds, trader, gridSpacing)

		s.logger.Info("bot: grid filled",
			"bot_id", bot.ID, "level", g.Level,
			"side", g.Side, "price", g.Price, "profit", profit)
	}
}

// placeCounterOrder виставляє протилежний ордер після виконання grid-ордеру.
// buy заповнився → sell на рівень вище; sell заповнився → buy на рівень нижче.
func (s *BotService) placeCounterOrder(
	bot models.Bot, filled models.BotGrid,
	creds exchange.Credentials, trader exchange.Trader,
	gridSpacing float64,
) {
	var counterSide string
	var counterPrice float64

	if filled.Side == "buy" {
		counterSide = "sell"
		counterPrice = filled.Price + gridSpacing
	} else {
		counterSide = "buy"
		counterPrice = filled.Price - gridSpacing
	}

	// Перевіряємо що ціна в межах сітки
	if counterPrice < bot.PriceLow || counterPrice > bot.PriceHigh {
		return
	}

	result, err := trader.PlaceOrder(creds, exchange.PlaceOrderRequest{
		Symbol:   bot.Symbol,
		Side:     exchange.OrderSide(counterSide),
		Type:     exchange.OrderTypeLimit,
		Category: exchange.OrderCategory(bot.Category),
		Quantity: bot.QtyPerGrid,
		Price:    counterPrice,
	})
	if err != nil {
		s.logger.Error("bot: place counter order failed",
			"bot_id", bot.ID, "side", counterSide, "price", counterPrice, "error", err)
		return
	}

	// Рівень вже існує, додаємо новий grid-запис для counter order
	newGrid := models.BotGrid{
		BotID:  bot.ID,
		Level:  filled.Level, // перевикористовуємо рівень
		Price:  counterPrice,
		Side:   counterSide,
		Status: "open",
	}
	orderID := result.OrderID
	newGrid.ExchangeOrderID = &orderID
	_ = s.bots.CreateGrids([]models.BotGrid{newGrid})

	s.logger.Info("bot: counter order placed",
		"bot_id", bot.ID, "side", counterSide, "price", counterPrice, "order_id", result.OrderID)
}

// calcLevels рівномірно розподіляє N+1 цінових рівнів між low і high.
func (s *BotService) calcLevels(low, high float64, grids int) []float64 {
	levels := make([]float64, grids+1)
	step := (high - low) / float64(grids)
	for i := range levels {
		// Округлення до 8 знаків після коми
		levels[i] = math.Round((low+float64(i)*step)*1e8) / 1e8
	}
	return levels
}

// getUserCreds розшифровує credentials для вказаної біржі.
func (s *BotService) getUserCreds(userID int64, exchangeName string) (exchange.Credentials, error) {
	creds, err := s.portfolio.GetCredentials(userID)
	if err != nil {
		return exchange.Credentials{}, err
	}
	for _, c := range creds {
		if c.Exchange != exchangeName || !c.IsActive {
			continue
		}
		apiKey, err := s.enc.Decrypt(c.ApiKeyEncrypted)
		if err != nil {
			return exchange.Credentials{}, err
		}
		apiSecret, err := s.enc.Decrypt(c.ApiSecretEncrypted)
		if err != nil {
			return exchange.Credentials{}, err
		}
		passphrase := ""
		if c.PassphraseEncrypted != nil {
			passphrase, _ = s.enc.Decrypt(*c.PassphraseEncrypted)
		}
		return exchange.Credentials{
			APIKey:     apiKey,
			APISecret:  apiSecret,
			Passphrase: passphrase,
		}, nil
	}
	return exchange.Credentials{}, exchange.ErrNoCredentials
}
