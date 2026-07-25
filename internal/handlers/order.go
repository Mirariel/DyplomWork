package handlers

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
	"github.com/okochadmytro/tradetracker/internal/validator"
)

// OrderHandler — HTTP handlers для торгових ордерів.
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
	CredentialID int64   `json:"credential_id" validate:"required,gt=0"`
	Symbol       string  `json:"symbol"        validate:"required,min=2"`
	Side         string  `json:"side"          validate:"required,oneof=buy sell"`
	Type         string  `json:"type"          validate:"required,oneof=market limit"`
	Category     string  `json:"category"      validate:"required,oneof=spot futures"`
	Leverage     string  `json:"leverage"`
	Quantity     float64 `json:"quantity"`
	AmountPct    float64 `json:"amount_pct"`
	Price        float64 `json:"price"`
}

// POST /api/orders — розмістити ордер на біржі
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
	if req.Quantity <= 0 && req.AmountPct <= 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "quantity or amount_pct is required",
		})
	}

	// Знаходимо credentials за ID
	creds, exchangeName, err := services.GetUserCredsByID(h.portfolio, h.enc, userID, req.CredentialID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "credentials not found or inactive",
		})
	}

	ex, ok := h.exchanges[exchangeName]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unknown exchange: " + exchangeName})
	}
	trader, ok := ex.(exchange.Trader)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": exchangeName + " does not support trading yet",
		})
	}

	// Визначаємо кількість
	quantity := req.Quantity
	if req.AmountPct > 0 {
		balances, err := ex.GetBalances(creds)
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to fetch balance: " + err.Error()})
		}
		// Вирахувати загальний баланс у USD через ціни
		symbols := make([]string, 0, len(balances))
		for _, b := range balances {
			symbols = append(symbols, b.Symbol)
		}
		prices, _ := ex.GetPrices(symbols)
		totalUSD := 0.0
		for _, b := range balances {
			if p, ok := prices[b.Symbol]; ok {
				totalUSD += b.Quantity * p
			}
		}
		if totalUSD <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "balance is zero"})
		}
		amountUSD := totalUSD * req.AmountPct / 100
		// Визначаємо ціну для конвертації USD → quantity
		targetPrice := req.Price
		if targetPrice <= 0 {
			if p, ok := prices[req.Symbol]; ok && p > 0 {
				targetPrice = p
			} else {
				symPrices, _ := ex.GetPrices([]string{req.Symbol})
				if p, ok := symPrices[req.Symbol]; ok && p > 0 {
					targetPrice = p
				}
			}
		}
		if targetPrice <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "cannot determine current price for % calculation",
			})
		}
		quantity = amountUSD / targetPrice
	}

	leverage := req.Leverage
	if req.Category == "spot" {
		leverage = ""
	}

	credID := req.CredentialID
	dbOrder := &models.Order{
		UserID:       userID,
		CredentialID: &credID,
		Exchange:     exchangeName,
		Symbol:       req.Symbol,
		Side:         req.Side,
		Type:         req.Type,
		Category:     req.Category,
		Leverage:     leverage,
		Quantity:     quantity,
		Price:        req.Price,
		Status:       "new",
	}
	if err := h.orders.Create(dbOrder); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db error"})
	}

	placed, err := trader.PlaceOrder(creds, exchange.PlaceOrderRequest{
		Symbol:   req.Symbol,
		Side:     exchange.OrderSide(req.Side),
		Type:     exchange.OrderType(req.Type),
		Category: exchange.OrderCategory(req.Category),
		Quantity: quantity,
		Price:    req.Price,
	})
	if err != nil {
		h.logger.Error("order: place failed",
			"user_id", userID, "exchange", exchangeName, "symbol", req.Symbol, "error", err)
		_ = h.orders.MarkFailed(dbOrder.ID, err.Error())
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}

	_ = h.orders.UpdateStatus(dbOrder.ID, placed.OrderID, string(placed.Status), placed.FilledQty, placed.AvgPrice)

	h.logger.Info("order: placed",
		"user_id", userID, "exchange", exchangeName,
		"symbol", req.Symbol, "order_id", placed.OrderID)

	dbOrder.ExchangeOrderID = placed.OrderID
	dbOrder.Status = string(placed.Status)
	dbOrder.FilledQty = placed.FilledQty
	dbOrder.AvgPrice = placed.AvgPrice
	dbOrder.Quantity = quantity
	return c.Status(fiber.StatusCreated).JSON(dbOrder)
}

// DELETE /api/orders/:id — скасувати ордер
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

	// Отримуємо credentials: спочатку по credential_id, потім fallback по exchange
	var creds exchange.Credentials
	if dbOrder.CredentialID != nil {
		creds, _, err = services.GetUserCredsByID(h.portfolio, h.enc, userID, *dbOrder.CredentialID)
	} else {
		creds, err = services.GetUserCreds(h.portfolio, h.enc, userID, dbOrder.Exchange)
	}
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

// GET /api/orders/:id — статус конкретного ордеру
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

	var creds exchange.Credentials
	if dbOrder.CredentialID != nil {
		creds, _, err = services.GetUserCredsByID(h.portfolio, h.enc, userID, *dbOrder.CredentialID)
	} else {
		creds, err = services.GetUserCreds(h.portfolio, h.enc, userID, dbOrder.Exchange)
	}
	if err != nil {
		return c.JSON(dbOrder)
	}

	ex, ok := h.exchanges[dbOrder.Exchange]
	if !ok {
		return c.JSON(dbOrder)
	}
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
		return c.JSON(dbOrder)
	}

	_ = h.orders.UpdateStatus(dbOrder.ID, dbOrder.ExchangeOrderID, string(live.Status), live.FilledQty, live.AvgPrice)
	dbOrder.Status = string(live.Status)
	dbOrder.FilledQty = live.FilledQty
	dbOrder.AvgPrice = live.AvgPrice
	return c.JSON(dbOrder)
}

// GET /api/orders?status=new&credential_id=5 — список ордерів користувача
func (h *OrderHandler) ListOrders(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	status := c.Query("status", "")

	// Фільтрація по credential_id (одному або кільком через кому)
	credFilter := c.Query("credential_ids", "")
	if credFilter != "" {
		parts := strings.Split(credFilter, ",")
		credIDs := make([]int64, 0, len(parts))
		for _, p := range parts {
			if id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64); err == nil {
				credIDs = append(credIDs, id)
			}
		}
		if len(credIDs) > 0 {
			orders, err := h.orders.ListByCredentialIDs(userID, credIDs)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(orders)
		}
	}

	orders, err := h.orders.ListByUser(userID, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(orders)
}
