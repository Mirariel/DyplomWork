package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
)

type FuturesHandler struct {
	repo *models.FuturesPositionRepository
}

func NewFuturesHandler(repo *models.FuturesPositionRepository) *FuturesHandler {
	return &FuturesHandler{repo: repo}
}

// GET /api/futures/positions?exchange=binance
// Без query param — агрегує з усіх бірж.
func (h *FuturesHandler) GetPositions(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	exchange := c.Query("exchange", "")

	positions, err := h.repo.GetByUser(userID, exchange)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	totalPnl := 0.0
	for _, p := range positions {
		totalPnl += p.UnrealizedPnl
	}

	return c.JSON(fiber.Map{
		"positions":            positions,
		"total_unrealized_pnl": totalPnl,
	})
}
