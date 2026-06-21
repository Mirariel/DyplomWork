package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/validator"
)

// DCAHandler — HTTP handlers для DCA ботів.
type DCAHandler struct {
	dcaBots    *models.DCABotRepository
	dcaService *services.DCAService
}

func NewDCAHandler(dcaBots *models.DCABotRepository, dcaService *services.DCAService) *DCAHandler {
	return &DCAHandler{dcaBots: dcaBots, dcaService: dcaService}
}

// createDCARequest — тіло POST /api/dca
type createDCARequest struct {
	Exchange      string  `json:"exchange"       validate:"required,oneof=binance okx bybit"`
	Symbol        string  `json:"symbol"         validate:"required,min=2,max=10"`
	Category      string  `json:"category"       validate:"required,oneof=spot futures"`
	AmountUSD     float64 `json:"amount_usd"     validate:"required,gt=0"`
	IntervalHours int     `json:"interval_hours" validate:"required,min=1,max=8760"` // 1h – 1 рік
}

// POST /api/dca — створити DCA бота
//
// Приклад: купувати BTC на 50 USD кожні 24 години:
//
//	{"exchange":"binance","symbol":"BTC","category":"spot","amount_usd":50,"interval_hours":24}
func (h *DCAHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req createDCARequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	b := &models.DCABot{
		UserID:        userID,
		Exchange:      req.Exchange,
		Symbol:        req.Symbol,
		Category:      req.Category,
		AmountUSD:     req.AmountUSD,
		IntervalHours: req.IntervalHours,
		NextBuyAt:     time.Now().Add(time.Duration(req.IntervalHours) * time.Hour),
	}
	if err := h.dcaBots.Create(b); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db error"})
	}
	return c.Status(fiber.StatusCreated).JSON(b)
}

// GET /api/dca — список DCA ботів
func (h *DCAHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	bots, err := h.dcaBots.ListByUser(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(bots)
}

// GET /api/dca/:id — один DCA бот
func (h *DCAHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	bot, err := h.dcaBots.GetByID(int64(id), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	return c.JSON(bot)
}

// POST /api/dca/:id/start — запустити DCA бота
//
// Query: ?buy_now=true — виконати першу покупку негайно (default: false)
func (h *DCAHandler) Start(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	buyNow := c.QueryBool("buy_now", false)

	if err := h.dcaService.Start(int64(id), userID, buyNow); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "running", "id": id})
}

// POST /api/dca/:id/stop — зупинити DCA бота
func (h *DCAHandler) Stop(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.dcaService.Stop(int64(id), userID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "stopped", "id": id})
}

// DELETE /api/dca/:id — видалити зупиненого бота
func (h *DCAHandler) Delete(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	bot, err := h.dcaBots.GetByID(int64(id), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	if bot.Status == "running" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "stop the bot before deleting"})
	}
	errMsg := "deleted"
	_ = h.dcaBots.SetStatus(int64(id), "stopped", &errMsg)
	return c.JSON(fiber.Map{"status": "deleted", "id": id})
}
