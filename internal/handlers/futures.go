package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
)

type FuturesHandler struct {
	repo *models.PortfolioRepository
}

func NewFuturesHandler(repo *models.PortfolioRepository) *FuturesHandler {
	return &FuturesHandler{repo: repo}
}

// GET /api/futures/positions?exchange=binance
// Reads from open_positions (the single source of truth for live positions).
// Response format is kept identical to the old futures_positions-based handler.
func (h *FuturesHandler) GetPositions(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	exchange := c.Query("exchange", "")

	positions, err := h.repo.GetOpenPositions(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Filter by exchange if requested + convert to response format
	type posResp struct {
		ID            int64   `json:"id"`
		UserID        int64   `json:"user_id"`
		Exchange      string  `json:"exchange"`
		Symbol        string  `json:"symbol"`
		Side          string  `json:"side"`
		Size          float64 `json:"size"`
		EntryPrice    float64 `json:"entry_price"`
		MarkPrice     float64 `json:"mark_price"`
		UnrealizedPnl float64 `json:"unrealized_pnl"`
		Leverage      int     `json:"leverage"`
		MarginType    string  `json:"margin_type"`
		Margin        float64 `json:"margin"`
		MarginMode    string  `json:"margin_mode"`
		TradeSize     float64 `json:"trade_size"`
		Quantity      float64 `json:"quantity"`
		SyncedAt      string  `json:"synced_at"`
	}

	var out []posResp
	totalPnl := 0.0
	for _, p := range positions {
		if exchange != "" && p.Exchange != exchange {
			continue
		}
		lev := parseLeverageInt(p.Leverage)
		out = append(out, posResp{
			ID:            p.ID,
			UserID:        p.UserID,
			Exchange:      p.Exchange,
			Symbol:        p.Symbol,
			Side:          p.Side,
			Size:          p.Quantity,
			EntryPrice:    p.EntryPrice,
			MarkPrice:     p.MarkPrice,
			UnrealizedPnl: p.PnL,
			Leverage:      lev,
			MarginType:    p.MarginMode,
			Margin:        p.Margin,
			MarginMode:    p.MarginMode,
			TradeSize:     p.TradeSize,
			Quantity:      p.Quantity,
			SyncedAt:      p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
		totalPnl += p.PnL
	}

	if out == nil {
		out = []posResp{}
	}

	return c.JSON(fiber.Map{
		"positions":            out,
		"total_unrealized_pnl": totalPnl,
	})
}

// parseLeverageInt parses "10x" → 10, "0x" → 0
func parseLeverageInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
