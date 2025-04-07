package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
)

type SyncHandler struct {
	sync      *services.SyncService
	prices    *services.PriceService
	portfolio *models.PortfolioRepository
}

func NewSyncHandler(sync *services.SyncService, prices *services.PriceService, portfolio *models.PortfolioRepository) *SyncHandler {
	return &SyncHandler{sync: sync, prices: prices, portfolio: portfolio}
}

// POST /api/sync/full — повний синк з усіма біржами
func (h *SyncHandler) FullSync(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	if err := h.sync.SyncUser(userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return h.portfolioResponse(c, userID)
}

// POST /api/sync/positions — синк лише відкритих позицій
func (h *SyncHandler) SyncPositions(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	if err := h.sync.SyncPositions(userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	positions, err := h.portfolio.GetOpenPositions(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok", "positions": positions})
}

// POST /api/sync/history?days=7 — легкий синк закритих угод за N днів
func (h *SyncHandler) SyncHistory(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	days := c.QueryInt("days", 7)

	if err := h.sync.SyncRecentHistory(userID, days); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	history, err := h.portfolio.GetHistory(userID, 50, 0)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok", "history": history})
}

// GET /api/sync/prices — оновлення цін активів користувача
func (h *SyncHandler) UpdatePrices(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	assets, err := h.portfolio.GetUserAssets(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	symbols := make([]string, 0, len(assets))
	seen := make(map[string]bool)
	for _, a := range assets {
		if !seen[a.Symbol] {
			symbols = append(symbols, a.Symbol)
			seen[a.Symbol] = true
		}
	}

	if err := h.prices.UpdatePrices(symbols); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return h.portfolioResponse(c, userID)
}

// portfolioResponse — повертає повний стан портфеля після синку
func (h *SyncHandler) portfolioResponse(c *fiber.Ctx, userID int64) error {
	assets, _ := h.portfolio.GetUserAssets(userID)
	positions, _ := h.portfolio.GetOpenPositions(userID)
	history, _ := h.portfolio.GetHistory(userID, 50, 0)

	totalValue := 0.0
	for _, a := range assets {
		totalValue += a.Quantity * a.CurrentPrice
	}

	return c.JSON(fiber.Map{
		"status":      "ok",
		"assets":      assets,
		"positions":   positions,
		"history":     history,
		"total_value": totalValue,
	})
}
