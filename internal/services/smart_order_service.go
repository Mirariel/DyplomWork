package services

import (
	"context"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/notify"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
)

// SmartOrderService перевіряє активні умовні ордери і тригерить market-ордери.
// Викликати CheckAndTrigger через scheduler кожні 5 секунд.
type SmartOrderService struct {
	smartOrders *models.SmartOrderRepository
	orders      *models.OrderRepository
	portfolio   *models.PortfolioRepository
	price       *PriceService
	enc         *EncryptionService
	exchanges   map[string]exchange.Exchange
	notifier    *notify.Notifier
	logger      *slog.Logger
}

func NewSmartOrderService(
	db *sqlx.DB,
	smartOrders *models.SmartOrderRepository,
	orders *models.OrderRepository,
	portfolio *models.PortfolioRepository,
	price *PriceService,
	enc *EncryptionService,
	notifier *notify.Notifier,
	logger *slog.Logger,
) *SmartOrderService {
	return &SmartOrderService{
		smartOrders: smartOrders,
		orders:      orders,
		portfolio:   portfolio,
		price:       price,
		enc:         enc,
		exchanges:   exchange.Registry(),
		notifier:    notifier,
		logger:      logger,
	}
}

// CheckAndTrigger — перевіряє всі активні smart orders і тригерить ті що спрацювали.
// Використовується фоновим scheduler'ом.
func (s *SmartOrderService) CheckAndTrigger(ctx context.Context) {
	active, err := s.smartOrders.ListActive()
	if err != nil || len(active) == 0 {
		return
	}

	// Отримуємо всі ціни одним батчем
	symbols := make([]string, 0, len(active))
	seen := map[string]bool{}
	for _, o := range active {
		if !seen[o.Symbol] {
			symbols = append(symbols, o.Symbol)
			seen[o.Symbol] = true
		}
	}
	prices := s.price.GetLivePrices(symbols)

	for _, o := range active {
		select {
		case <-ctx.Done():
			return
		default:
		}

		currentPrice, ok := prices[o.Symbol]
		if !ok || currentPrice <= 0 {
			continue
		}

		triggered, newPeak := s.checkTrigger(o, currentPrice)

		// Оновлюємо peak для trailing stop (навіть якщо ще не тригернули)
		if o.Type == "trailing_stop" && newPeak != o.PeakPrice {
			if err := s.smartOrders.UpdatePeak(o.ID, newPeak); err != nil {
				s.logger.Error("smart_order: update peak failed", "id", o.ID, "error", err)
			}
			o.PeakPrice = newPeak
		}

		if !triggered {
			continue
		}

		s.logger.Info("smart_order: triggered",
			"id", o.ID, "type", o.Type, "symbol", o.Symbol,
			"side", o.Side, "price", currentPrice)

		s.executeOrder(o)
	}
}

// checkTrigger повертає (triggered bool, newPeakPrice float64).
func (s *SmartOrderService) checkTrigger(o models.SmartOrder, price float64) (bool, float64) {
	switch o.Type {
	case "stop_loss":
		// sell stop-loss (захист лонгу): тригер коли ціна <= trigger_price
		if o.Side == "sell" {
			return price <= o.TriggerPrice, o.PeakPrice
		}
		// buy stop-loss (захист шорту): тригер коли ціна >= trigger_price
		return price >= o.TriggerPrice, o.PeakPrice

	case "take_profit":
		// sell take-profit (фіксація прибутку лонгу): тригер коли ціна >= trigger_price
		if o.Side == "sell" {
			return price >= o.TriggerPrice, o.PeakPrice
		}
		// buy take-profit (фіксація прибутку шорту): тригер коли ціна <= trigger_price
		return price <= o.TriggerPrice, o.PeakPrice

	case "trailing_stop":
		return s.checkTrailing(o, price)
	}
	return false, o.PeakPrice
}

// checkTrailing обраховує trailing stop логіку і повертає (triggered, newPeak).
func (s *SmartOrderService) checkTrailing(o models.SmartOrder, price float64) (bool, float64) {
	if o.CallbackRate <= 0 {
		return false, o.PeakPrice
	}

	if o.Side == "sell" {
		// Захист лонгу: відстежуємо максимум, тригер коли падіння >= callback_rate%
		peak := o.PeakPrice
		if price > peak {
			peak = price
		}
		if peak > 0 {
			dropPct := (peak - price) / peak * 100
			if dropPct >= o.CallbackRate {
				return true, peak
			}
		}
		return false, peak
	}

	// Захист шорту: відстежуємо мінімум, тригер коли зростання >= callback_rate%
	peak := o.PeakPrice
	if peak == 0 || price < peak {
		peak = price
	}
	if peak > 0 {
		risePct := (price - peak) / peak * 100
		if risePct >= o.CallbackRate {
			return true, peak
		}
	}
	return false, peak
}

// executeOrder розміщує market ордер на біржі і оновлює статус smart order.
func (s *SmartOrderService) executeOrder(o models.SmartOrder) {
	creds, err := s.getUserCreds(o.UserID, o.Exchange)
	if err != nil {
		s.logger.Error("smart_order: no creds", "id", o.ID, "exchange", o.Exchange, "error", err)
		_ = s.smartOrders.MarkFailed(o.ID, "no credentials: "+err.Error())
		return
	}

	ex, ok := s.exchanges[o.Exchange]
	if !ok {
		_ = s.smartOrders.MarkFailed(o.ID, "unknown exchange: "+o.Exchange)
		return
	}
	trader, ok := ex.(exchange.Trader)
	if !ok {
		_ = s.smartOrders.MarkFailed(o.ID, o.Exchange+" does not support trading")
		return
	}

	dbOrder := &models.Order{
		UserID:   o.UserID,
		Exchange: o.Exchange,
		Symbol:   o.Symbol,
		Side:     o.Side,
		Type:     "market",
		Category: o.Category,
		Quantity: o.Quantity,
		Status:   "new",
	}
	if err := s.orders.Create(dbOrder); err != nil {
		s.logger.Error("smart_order: db create failed", "id", o.ID, "error", err)
		_ = s.smartOrders.MarkFailed(o.ID, "db error: "+err.Error())
		return
	}

	placed, err := trader.PlaceOrder(creds, exchange.PlaceOrderRequest{
		Symbol:   o.Symbol,
		Side:     exchange.OrderSide(o.Side),
		Type:     exchange.OrderTypeMarket,
		Category: exchange.OrderCategory(o.Category),
		Quantity: o.Quantity,
	})
	if err != nil {
		s.logger.Error("smart_order: place failed", "id", o.ID, "error", err)
		_ = s.orders.MarkFailed(dbOrder.ID, err.Error())
		_ = s.smartOrders.MarkFailed(o.ID, err.Error())
		s.notifier.SmartOrderFailed(o.ID, o.Symbol, o.Exchange, err.Error())
		return
	}

	_ = s.orders.UpdateStatus(dbOrder.ID, placed.OrderID, string(placed.Status), placed.FilledQty, placed.AvgPrice)
	_ = s.smartOrders.MarkTriggered(o.ID, dbOrder.ID)

	s.logger.Info("smart_order: executed",
		"smart_id", o.ID, "order_id", dbOrder.ID,
		"exchange", o.Exchange, "symbol", o.Symbol, "side", o.Side)

	s.notifier.SmartOrderTriggered(o.Type, o.Symbol, o.Exchange, placed.AvgPrice)
}

// getUserCreds розшифровує credentials для вказаної біржі.
func (s *SmartOrderService) getUserCreds(userID int64, exchangeName string) (exchange.Credentials, error) {
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
