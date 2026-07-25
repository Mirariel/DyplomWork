package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/validator"
)

// SmartOrderHandler — HTTP handlers для умовних ордерів (SL/TP/Trailing).
type SmartOrderHandler struct {
	smartOrders *models.SmartOrderRepository
	portfolio   *models.PortfolioRepository
	enc         *services.EncryptionService
}

func NewSmartOrderHandler(
	smartOrders *models.SmartOrderRepository,
	portfolio *models.PortfolioRepository,
	enc *services.EncryptionService,
) *SmartOrderHandler {
	return &SmartOrderHandler{
		smartOrders: smartOrders,
		portfolio:   portfolio,
		enc:         enc,
	}
}

// createSmartOrderRequest — тіло POST /api/smart-orders
type createSmartOrderRequest struct {
	CredentialID int64   `json:"credential_id" validate:"required,gt=0"`
	Symbol       string  `json:"symbol"        validate:"required,min=2"`
	Side         string  `json:"side"          validate:"required,oneof=buy sell"`
	Category     string  `json:"category"      validate:"required,oneof=spot futures"`
	Leverage     string  `json:"leverage"`
	Quantity     float64 `json:"quantity"      validate:"required,gt=0"`
	Type         string  `json:"type"          validate:"required,oneof=stop_loss take_profit trailing_stop"`
	TriggerPrice float64 `json:"trigger_price"`
	CallbackRate float64 `json:"callback_rate"`
}

// POST /api/smart-orders — створити умовний ордер
func (h *SmartOrderHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req createSmartOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	// Валідація специфічна для типу
	switch req.Type {
	case "stop_loss", "take_profit":
		if req.TriggerPrice <= 0 {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "trigger_price is required for " + req.Type,
			})
		}
	case "trailing_stop":
		if req.CallbackRate <= 0 || req.CallbackRate > 50 {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error": "callback_rate must be between 0.01 and 50 (percent)",
			})
		}
	}

	// Знаходимо credentials за ID щоб визначити біржу
	cred, err := h.portfolio.GetCredentialByID(req.CredentialID, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "credentials not found or inactive",
		})
	}

	leverage := req.Leverage
	if req.Category == "spot" {
		leverage = ""
	}

	credID := req.CredentialID
	o := &models.SmartOrder{
		UserID:       userID,
		CredentialID: &credID,
		Exchange:     cred.Exchange,
		Symbol:       req.Symbol,
		Side:         req.Side,
		Category:     req.Category,
		Leverage:     leverage,
		Quantity:     req.Quantity,
		Type:         req.Type,
		TriggerPrice: req.TriggerPrice,
		CallbackRate: req.CallbackRate,
	}
	if err := h.smartOrders.Create(o); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "db error"})
	}
	return c.Status(fiber.StatusCreated).JSON(o)
}

// GET /api/smart-orders — список умовних ордерів
func (h *SmartOrderHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	orders, err := h.smartOrders.ListByUser(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(orders)
}

// GET /api/smart-orders/:id — один умовний ордер
func (h *SmartOrderHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	o, err := h.smartOrders.GetByID(int64(id), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	return c.JSON(o)
}

// DELETE /api/smart-orders/:id — скасувати умовний ордер
func (h *SmartOrderHandler) Cancel(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.smartOrders.Cancel(int64(id), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "cancelled", "id": id})
}
