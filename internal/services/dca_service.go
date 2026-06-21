package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/notify"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
)

// DCAService виконує Dollar Cost Averaging стратегію.
// CheckAndBuy — викликати кожні 5 хвилин через scheduler.
// При кожному спрацюванні: купує на amount_usd USD по ринковій ціні,
// потім планує наступну покупку через interval_hours годин.
type DCAService struct {
	dcaBots   *models.DCABotRepository
	portfolio *models.PortfolioRepository
	price     *PriceService
	enc       *EncryptionService
	exchanges map[string]exchange.Exchange
	notifier  *notify.Notifier
	logger    *slog.Logger
}

func NewDCAService(
	dcaBots *models.DCABotRepository,
	portfolio *models.PortfolioRepository,
	price *PriceService,
	enc *EncryptionService,
	notifier *notify.Notifier,
	logger *slog.Logger,
) *DCAService {
	return &DCAService{
		dcaBots:   dcaBots,
		portfolio: portfolio,
		price:     price,
		enc:       enc,
		exchanges: exchange.Registry(),
		notifier:  notifier,
		logger:    logger,
	}
}

// Start активує DCA бота і планує першу покупку — негайно або через interval.
// buyNow=true → перша покупка відразу при старті.
func (s *DCAService) Start(botID, userID int64, buyNow bool) error {
	bot, err := s.dcaBots.GetByID(botID, userID)
	if err != nil {
		return fmt.Errorf("bot not found: %w", err)
	}
	if bot.Status == "running" {
		return fmt.Errorf("bot is already running")
	}

	var nextBuyAt time.Time
	if buyNow {
		nextBuyAt = time.Now()
	} else {
		nextBuyAt = time.Now().Add(time.Duration(bot.IntervalHours) * time.Hour)
	}

	if err := s.dcaBots.RecordBuy(botID, 0, 0, nextBuyAt); err != nil {
		return err
	}
	return s.dcaBots.SetStatus(botID, "running", nil)
}

// Stop зупиняє DCA бота.
func (s *DCAService) Stop(botID, userID int64) error {
	bot, err := s.dcaBots.GetByID(botID, userID)
	if err != nil {
		return fmt.Errorf("bot not found: %w", err)
	}
	if bot.Status != "running" {
		return fmt.Errorf("bot is not running")
	}
	return s.dcaBots.SetStatus(botID, "stopped", nil)
}

// CheckAndBuy перевіряє всі DCA боти і виконує покупки для тих що настали.
// Викликається scheduler'ом кожні 5 хвилин.
func (s *DCAService) CheckAndBuy(ctx context.Context) {
	due, err := s.dcaBots.ListDue()
	if err != nil || len(due) == 0 {
		return
	}

	for _, bot := range due {
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.executeBuy(bot)
	}
}

func (s *DCAService) executeBuy(bot models.DCABot) {
	// Отримуємо поточну ціну для розрахунку кількості
	prices := s.price.GetLivePrices([]string{bot.Symbol})
	currentPrice, ok := prices[bot.Symbol]
	if !ok || currentPrice <= 0 {
		s.logger.Error("dca: cannot get price", "bot_id", bot.ID, "symbol", bot.Symbol)
		errMsg := "cannot get current price for " + bot.Symbol
		_ = s.dcaBots.SetStatus(bot.ID, "stopped", &errMsg)
		return
	}

	// Кількість = сума USD / ціна
	qty := bot.AmountUSD / currentPrice

	creds, err := GetUserCreds(s.portfolio, s.enc, bot.UserID, bot.Exchange)
	if err != nil {
		s.logger.Error("dca: no credentials", "bot_id", bot.ID, "exchange", bot.Exchange)
		errMsg := "no credentials: " + err.Error()
		_ = s.dcaBots.SetStatus(bot.ID, "stopped", &errMsg)
		return
	}

	ex, ok := s.exchanges[bot.Exchange]
	if !ok {
		errMsg := "unknown exchange: " + bot.Exchange
		_ = s.dcaBots.SetStatus(bot.ID, "stopped", &errMsg)
		return
	}
	trader, ok := ex.(exchange.Trader)
	if !ok {
		errMsg := bot.Exchange + " does not support trading"
		_ = s.dcaBots.SetStatus(bot.ID, "stopped", &errMsg)
		return
	}

	placed, err := trader.PlaceOrder(creds, exchange.PlaceOrderRequest{
		Symbol:   bot.Symbol,
		Side:     exchange.OrderSideBuy,
		Type:     exchange.OrderTypeMarket,
		Category: exchange.OrderCategory(bot.Category),
		Quantity: qty,
	})
	if err != nil {
		s.logger.Error("dca: place order failed",
			"bot_id", bot.ID, "symbol", bot.Symbol, "qty", qty, "error", err)
		errMsg := err.Error()
		_ = s.dcaBots.SetStatus(bot.ID, "stopped", &errMsg)
		return
	}

	// Плануємо наступну покупку
	nextBuyAt := time.Now().Add(time.Duration(bot.IntervalHours) * time.Hour)
	actualQty := placed.FilledQty
	if actualQty == 0 {
		actualQty = qty
	}

	if err := s.dcaBots.RecordBuy(bot.ID, actualQty, bot.AmountUSD, nextBuyAt); err != nil {
		s.logger.Error("dca: record buy failed", "bot_id", bot.ID, "error", err)
	}

	s.logger.Info("dca: buy executed",
		"bot_id", bot.ID, "symbol", bot.Symbol,
		"qty", actualQty, "price", currentPrice,
		"amount_usd", bot.AmountUSD,
		"next_buy_at", nextBuyAt.Format(time.RFC3339))

	s.notifier.DCABought(bot.Symbol, bot.Exchange, actualQty, bot.AmountUSD, currentPrice)
}

