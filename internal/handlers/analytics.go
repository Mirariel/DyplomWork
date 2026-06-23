package handlers

import (
	"time"

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
	days := c.QueryInt("days", 0)
	exchange := c.Query("exchange", "")
	from := c.Query("from")
	to := c.Query("to")
	summary, err := h.analytics.GetTradeSummary(userID, days, exchange, from, to)
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

// GET /api/analytics/snapshots — погодинні/денні дані для графіку Portfolio Value.
//
// Параметри (пріоритет: from+to > hours > days):
//   - hours=24          — останні N годин (для 1d view)
//   - days=30           — останні N днів (для 7d/30d/90d view)
//   - from=RFC3339&to=RFC3339 — довільний діапазон
func (h *AnalyticsHandler) Snapshots(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	fromStr := c.Query("from")
	toStr   := c.Query("to")
	hours   := c.QueryInt("hours", 0)
	days    := c.QueryInt("days", 30)

	var snaps []models.PortfolioSnapshot
	var err error

	switch {
	case fromStr != "" && toStr != "":
		from, errF := time.Parse(time.RFC3339, fromStr)
		to, errT   := time.Parse(time.RFC3339, toStr)
		if errF != nil || errT != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from/to format, use RFC3339"})
		}
		snaps, err = h.snapshots.ListByUserRange(userID, from, to)
	case hours > 0:
		if hours > 24*365 {
			hours = 24 * 365
		}
		snaps, err = h.snapshots.ListByUserHours(userID, hours)
	default:
		if days < 1 || days > 365 {
			days = 30
		}
		snaps, err = h.snapshots.ListByUser(userID, days)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(snaps)
}

// GET /api/analytics/chart?range=7d&exchange=all — точки для графіку Portfolio Value.
//
// Параметри:
//   - range=1d|7d|30d|90d|all (default: 7d) — попередньо визначений діапазон
//   - exchange=all|binance|bybit|...  (default: all) — конкретна біржа або агрегована сума
//   - from=RFC3339&to=RFC3339 — довільний діапазон (має пріоритет над range)
//
// Відповідь: [{recorded_at: "...", value: 12345.67}, ...]
func (h *AnalyticsHandler) Chart(c *fiber.Ctx) error {
	userID  := middleware.GetUserID(c)
	exchange := c.Query("exchange", "all")
	rangeKey := c.Query("range", "7d")
	fromStr  := c.Query("from")
	toStr    := c.Query("to")

	var (
		points []models.ChartPoint
		err    error
	)

	if fromStr != "" && toStr != "" {
		from, errF := time.Parse(time.RFC3339, fromStr)
		to, errT   := time.Parse(time.RFC3339, toStr)
		if errF != nil || errT != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from/to, use RFC3339"})
		}
		if exchange == "all" || exchange == "" {
			points, err = h.snapshots.ChartAllRange(userID, from, to)
		} else {
			points, err = h.snapshots.ChartByExchangeRange(userID, exchange, from, to)
		}
	} else {
		if exchange == "all" || exchange == "" {
			points, err = h.snapshots.ChartAll(userID, rangeKey)
		} else {
			points, err = h.snapshots.ChartByExchange(userID, exchange, rangeKey)
		}
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(points)
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
