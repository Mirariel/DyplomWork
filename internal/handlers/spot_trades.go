package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/models"
)

type SpotTradesHandler struct {
	repo *models.SpotTradeRepository
}

func NewSpotTradesHandler(repo *models.SpotTradeRepository) *SpotTradesHandler {
	return &SpotTradesHandler{repo: repo}
}

// GET /api/spot-trades?exchange=
func (h *SpotTradesHandler) GetTrades(c *fiber.Ctx) error {
	userID := c.Locals("userID").(int64)
	exFilter := c.Query("exchange", "")

	trades, err := h.repo.GetByUser(userID, exFilter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if trades == nil {
		trades = []models.SpotTrade{}
	}
	return c.JSON(fiber.Map{"trades": trades})
}
