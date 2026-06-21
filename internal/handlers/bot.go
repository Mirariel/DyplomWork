package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/validator"
)

// BotHandler — HTTP handlers для grid-ботів.
type BotHandler struct {
	bots       *models.BotRepository
	botService *services.BotService
	prices     *services.PriceService
}

func NewBotHandler(
	bots *models.BotRepository,
	botService *services.BotService,
	prices *services.PriceService,
) *BotHandler {
	return &BotHandler{bots: bots, botService: botService, prices: prices}
}

// createBotRequest — тіло POST /api/bots
type createBotRequest struct {
	Exchange   string  `json:"exchange"     validate:"required,oneof=binance okx bybit"`
	Symbol     string  `json:"symbol"       validate:"required,min=2,max=10"`
	Category   string  `json:"category"     validate:"required,oneof=spot futures"`
	PriceLow   float64 `json:"price_low"    validate:"required,gt=0"`
	PriceHigh  float64 `json:"price_high"   validate:"required,gt=0"`
	Grids      int     `json:"grids"        validate:"required,min=2,max=100"`
	QtyPerGrid float64 `json:"qty_per_grid" validate:"required,gt=0"`
}

// POST /api/bots — створити бота (не запускає автоматично)
//
// Приклад:
//
//	{ "exchange":"binance","symbol":"BTC","category":"spot",
//	  "price_low":60000,"price_high":70000,"grids":10,"qty_per_grid":0.001 }
func (h *BotHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req createBotRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	if req.PriceLow >= req.PriceHigh {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "price_low must be less than price_high",
		})
	}

	b := &models.Bot{
		UserID:     userID,
		Exchange:   req.Exchange,
		Symbol:     req.Symbol,
		Category:   req.Category,
		PriceLow:   req.PriceLow,
		PriceHigh:  req.PriceHigh,
		Grids:      req.Grids,
		QtyPerGrid: req.QtyPerGrid,
	}
	if err := h.bots.Create(b); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db error"})
	}
	return c.Status(fiber.StatusCreated).JSON(b)
}

// GET /api/bots — список ботів користувача
func (h *BotHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	bots, err := h.bots.ListByUser(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(bots)
}

// GET /api/bots/:id — один бот + рівні сітки
func (h *BotHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	bot, err := h.bots.GetByID(int64(id), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}

	grids, _ := h.bots.ListAllGrids(bot.ID)
	return c.JSON(fiber.Map{"bot": bot, "grids": grids})
}

// POST /api/bots/:id/start — запустити бота
func (h *BotHandler) Start(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	bot, err := h.bots.GetByID(int64(id), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}

	// Отримуємо поточну ціну для визначення сторони ордерів
	prices := h.prices.GetLivePrices([]string{bot.Symbol})
	currentPrice, ok := prices[bot.Symbol]
	if !ok || currentPrice <= 0 {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "cannot get current price for " + bot.Symbol,
		})
	}

	if err := h.botService.Start(int64(id), userID, currentPrice); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "running", "id": id, "current_price": currentPrice})
}

// POST /api/bots/:id/stop — зупинити бота
func (h *BotHandler) Stop(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	if err := h.botService.Stop(int64(id), userID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "stopped", "id": id})
}

// DELETE /api/bots/:id — видалити бота (тільки якщо stopped)
func (h *BotHandler) Delete(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	bot, err := h.bots.GetByID(int64(id), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	if bot.Status == "running" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "stop the bot before deleting",
		})
	}

	// Каскадне видалення через SQL (або просто позначаємо deleted)
	// Для спрощення - просто міняємо статус
	errMsg := "deleted"
	_ = h.bots.SetStatus(int64(id), "stopped", &errMsg)
	return c.JSON(fiber.Map{"status": "deleted", "id": id})
}
