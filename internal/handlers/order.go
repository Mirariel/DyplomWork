package handlers

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
	"github.com/okochadmytro/tradetracker/internal/validator"
)

// OrderHandler — HTTP handlers для торгових ордерів.
// Приймає запит → знаходить credentials користувача → ставить ордер на біржі → зберігає в БД.
type OrderHandler struct {
	orders    *models.OrderRepository
	portfolio *models.PortfolioRepository
	enc       *services.EncryptionService
	exchanges map[string]exchange.Exchange
	logger    *slog.Logger
}

func NewOrderHandler(
	orders *models.OrderRepository,
	portfolio *models.PortfolioRepository,
	enc *services.EncryptionService,
	logger *slog.Logger,
) *OrderHandler {
	return &OrderHandler{
		orders:    orders,
		portfolio: portfolio,
		enc:       enc,
		exchanges: exchange.Registry(),
		logger:    logger,
	}
}

// placeOrderRequest — тіло запиту POST /api/orders
type placeOrderRequest struct {
	Exchange string  `json:"exchange"  validate:"required,oneof=binance okx bybit"`
	Symbol   string  `json:"symbol"    validate:"required,min=2,max=10"`
	Side     string  `json:"side"      validate:"required,oneof=buy sell"`
	Type     string  `json:"type"      validate:"required,oneof=market limit"`
	Category string  `json:"category"  validate:"required,oneof=spot futures"`
	Quantity float64 `json:"quantity"  validate:"required,gt=0"`
	Price    float64 `json:"price"`   // обов'язковий для limit, 0 для market
}

// POST /api/orders — розмістити ордер на біржі
//
// Приклад запиту:
//
//	{
//	  "exchange":  "binance",
//	  "symbol":    "BTC",
//	  "side":      "buy",
//	  "type":      "limit",
//	  "category":  "spot",
//	  "quantity":  0.001,
//	  "price":     65000
//	}
func (h *OrderHandler) PlaceOrder(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req placeOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Type == "limit" && req.Price <= 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "price is required for limit orders",
		})
	}

	// Отримуємо credentials цієї біржі для користувача
	creds, err := h.getUserCreds(userID, req.Exchange)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "no active credentials found for " + req.Exchange,
		})
	}

	// Знаходимо адаптер і перевіряємо що він підтримує торгівлю
	ex, ok := h.exchanges[req.Exchange]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unknown exchange"})
	}
	trader, ok := ex.(exchange.Trader)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": req.Exchange + " does not support trading yet",
		})
	}

	// Зберігаємо ордер в БД до відправки на біржу (статус = "new")
	dbOrder := &models.Order{
		UserID:   userID,
		Exchange: req.Exchange,
		Symbol:   req.Symbol,
		Side:     req.Side,
		Type:     req.Type,
		Category: req.Category,
		Quantity: req.Quantity,
		Price:    req.Price,
		Status:   "new",
	}
	if err := h.orders.Create(dbOrder); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db error"})
	}

	// Ставимо ордер на біржі
	placed, err := trader.PlaceOrder(creds, exchange.PlaceOrderRequest{
		Symbol:   req.Symbol,
		Side:     exchange.OrderSide(req.Side),
		Type:     exchange.OrderType(req.Type),
		Category: exchange.OrderCategory(req.Category),
		Quantity: req.Quantity,
		Price:    req.Price,
	})
	if err != nil {
		h.logger.Error("order: place failed",
			"user_id", userID, "exchange", req.Exchange, "symbol", req.Symbol, "error", err)
		_ = h.orders.MarkFailed(dbOrder.ID, err.Error())
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}

	// Оновлюємо статус і exchange_order_id
	_ = h.orders.UpdateStatus(dbOrder.ID, placed.OrderID, string(placed.Status), placed.FilledQty, placed.AvgPrice)

	h.logger.Info("order: placed",
		"user_id", userID, "exchange", req.Exchange,
		"symbol", req.Symbol, "order_id", placed.OrderID)

	dbOrder.ExchangeOrderID = placed.OrderID
	dbOrder.Status = string(placed.Status)
	dbOrder.FilledQty = placed.FilledQty
	dbOrder.AvgPrice = placed.AvgPrice
	return c.Status(fiber.StatusCreated).JSON(dbOrder)
}

// DELETE /api/orders/:id — скасувати ордер
//
// :id — наш DB ID (не exchange order ID)
func (h *OrderHandler) CancelOrder(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	dbOrder, err := h.orders.GetByID(int64(id), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
	}
	if dbOrder.Status == "filled" || dbOrder.Status == "cancelled" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "order is already " + dbOrder.Status,
		})
	}
	if dbOrder.ExchangeOrderID == "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "order has no exchange order ID (may still be pending)",
		})
	}

	creds, err := h.getUserCreds(userID, dbOrder.Exchange)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "credentials not found"})
	}

	ex := h.exchanges[dbOrder.Exchange]
	trader, ok := ex.(exchange.Trader)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "exchange does not support trading"})
	}

	if err := trader.CancelOrder(creds, exchange.CancelOrderRequest{
		OrderID:  dbOrder.ExchangeOrderID,
		Symbol:   dbOrder.Symbol,
		Category: exchange.OrderCategory(dbOrder.Category),
	}); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}

	_ = h.orders.UpdateStatus(dbOrder.ID, dbOrder.ExchangeOrderID, "cancelled", dbOrder.FilledQty, dbOrder.AvgPrice)
	return c.JSON(fiber.Map{"status": "cancelled", "id": dbOrder.ID})
}

// GET /api/orders/:id — статус конкретного ордеру (живий запит до біржі)
func (h *OrderHandler) GetOrder(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	dbOrder, err := h.orders.GetByID(int64(id), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
	}
	if dbOrder.ExchangeOrderID == "" {
		return c.JSON(dbOrder)
	}

	creds, err := h.getUserCreds(userID, dbOrder.Exchange)
	if err != nil {
		return c.JSON(dbOrder)
	}

	ex := h.exchanges[dbOrder.Exchange]
	trader, ok := ex.(exchange.Trader)
	if !ok {
		return c.JSON(dbOrder)
	}

	live, err := trader.GetOrderStatus(creds, exchange.CancelOrderRequest{
		OrderID:  dbOrder.ExchangeOrderID,
		Symbol:   dbOrder.Symbol,
		Category: exchange.OrderCategory(dbOrder.Category),
	})
	if err != nil {
		// Повертаємо DB-версію якщо біржа недоступна
		return c.JSON(dbOrder)
	}

	// Оновлюємо БД свіжими даними
	_ = h.orders.UpdateStatus(dbOrder.ID, dbOrder.ExchangeOrderID, string(live.Status), live.FilledQty, live.AvgPrice)
	dbOrder.Status = string(live.Status)
	dbOrder.FilledQty = live.FilledQty
	dbOrder.AvgPrice = live.AvgPrice
	return c.JSON(dbOrder)
}

// GET /api/orders?status=new — список ордерів користувача
func (h *OrderHandler) ListOrders(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	status := c.Query("status", "")

	orders, err := h.orders.ListByUser(userID, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(orders)
}

// --- internal ---

// getUserCreds розшифровує credentials для вказаної біржі.
func (h *OrderHandler) getUserCreds(userID int64, exchangeName string) (exchange.Credentials, error) {
	creds, err := h.portfolio.GetCredentials(userID)
	if err != nil {
		return exchange.Credentials{}, err
	}
	for _, c := range creds {
		if c.Exchange != exchangeName || !c.IsActive {
			continue
		}
		apiKey, err := h.enc.Decrypt(c.ApiKeyEncrypted)
		if err != nil {
			return exchange.Credentials{}, err
		}
		apiSecret, err := h.enc.Decrypt(c.ApiSecretEncrypted)
		if err != nil {
			return exchange.Credentials{}, err
		}
		passphrase := ""
		if c.PassphraseEncrypted != nil {
			passphrase, _ = h.enc.Decrypt(*c.PassphraseEncrypted)
		}
		return exchange.Credentials{
			APIKey:     apiKey,
			APISecret:  apiSecret,
			Passphrase: passphrase,
		}, nil
	}
	return exchange.Credentials{}, fiber.ErrNotFound
}
