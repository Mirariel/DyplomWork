// analytics owns trade analytics, portfolio snapshots, and arbitrage scanning.
//
// Responsibilities:
//   - REST handlers for /api/analytics/*
//
// Auth: trusts X-Internal-User-ID header set by api-gateway (InternalAuth middleware).
package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/okochadmytro/tradetracker/internal/config"
	"github.com/okochadmytro/tradetracker/internal/database"
	"github.com/okochadmytro/tradetracker/internal/handlers"
	"github.com/okochadmytro/tradetracker/internal/metrics"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
)

func main() {
	cfg := config.Load()
	if cfg.AppPort == "8080" {
		cfg.AppPort = "8083" // default for analytics
	}

	var logHandler slog.Handler
	if cfg.AppDebug {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	logger := slog.New(logHandler)

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Error("DB connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	snapshotRepo    := models.NewSnapshotRepository(db)
	analyticsService := services.NewAnalyticsService(db, snapshotRepo, logger)
	analyticsHandler := handlers.NewAnalyticsHandler(analyticsService, snapshotRepo)

	// ── Fiber app ─────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(recover.New())
	app.Use(metrics.Middleware())

	api := app.Group("/api", middleware.InternalAuth())
	analyticsGroup := api.Group("/analytics")
	analyticsGroup.Get("/summary",   analyticsHandler.Summary)
	analyticsGroup.Get("/coins",     analyticsHandler.Coins)
	analyticsGroup.Get("/snapshots", analyticsHandler.Snapshots)
	analyticsGroup.Post("/snapshot", analyticsHandler.TakeSnapshot)
	analyticsGroup.Get("/arbitrage", analyticsHandler.Arbitrage)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "analytics"})
	})
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("analytics starting", "port", cfg.AppPort)
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			logger.Error("server error", "error", err)
		}
	}()

	<-quit
	logger.Info("analytics shutting down")
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}
