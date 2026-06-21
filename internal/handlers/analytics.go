package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
)

// AnalyticsHandler — HTTP handlers для аналітики портфеля.
type AnalyticsHandler struct {
	analytics *services.AnalyticsService
	snapshots *models.SnapshotRepository
}

func NewAnalyticsHandler(analytics *services.AnalyticsService, snapshots *models.SnapshotRepository) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics, snapshots: snapshots}
}

// GET /api/analytics/summary — загальна статистика торгівлі
//
// Відповідь:
//
//	{
//	  "total_trades": 42, "winning_trades": 28, "losing_trades": 14,
//	  "winrate": 66.67, "total_realized_pnl": 1250.5,
//	  "avg_pnl": 29.77, "best_trade": 380.0, "worst_trade": -95.0,
//	  "avg_win": 67.5, "avg_loss": -32.1, "profit_factor": 2.1
//	}
func (h *AnalyticsHandler) Summary(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	summary, err := h.analytics.GetTradeSummary(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(summary)
}

// GET /api/analytics/coins — статистика по кожній монеті
//
// Відповідь: масив об'єктів, відсортованих за total_pnl DESC:
//
//	[{"symbol":"BTC","exchange":"binance","total_trades":10,"winrate":70,"total_pnl":850.0,...}]
func (h *AnalyticsHandler) Coins(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	coins, err := h.analytics.GetCoinPerformance(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(coins)
}

// GET /api/analytics/snapshots?days=30 — PnL по часу (для графіку)
//
// Параметри:
//   - days=30 (за замовчуванням) — кількість днів назад
//
// Відповідь: масив точок графіку по датах:
//
//	[{"snapshot_date":"2026-06-01","total_value":12500.0,"spot_value":11000.0,"futures_pnl":1500.0}]
func (h *AnalyticsHandler) Snapshots(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	days := c.QueryInt("days", 30)
	if days < 1 || days > 365 {
		days = 30
	}
	snaps, err := h.snapshots.ListByUser(userID, days)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(snaps)
}

// POST /api/analytics/snapshot — зробити snapshot зараз (без чекання scheduler)
func (h *AnalyticsHandler) TakeSnapshot(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if err := h.analytics.TakeSnapshotForUser(userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// GET /api/analytics/arbitrage?min_spread=0.5 — arbitrage scanner між біржами
//
// Параметри:
//   - min_spread=0.5 (default) — мінімальний спред у відсотках
//
// Відповідь: масив можливостей, відсортованих за спредом DESC:
//
//	[{"symbol":"BTC","buy_exchange":"kraken","buy_price":67000,
//	  "sell_exchange":"binance","sell_price":67400,"spread_usd":400,"spread_pct":0.6}]
func (h *AnalyticsHandler) Arbitrage(c *fiber.Ctx) error {
	minSpread := c.QueryFloat("min_spread", 0.5)
	if minSpread < 0 || minSpread > 100 {
		minSpread = 0.5
	}
	opps := h.analytics.GetArbitrage(minSpread)
	return c.JSON(opps)
}
