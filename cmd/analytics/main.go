// analytics owns trade analytics, portfolio snapshots, and arbitrage scanning.
//
// Responsibilities:
//   - REST handlers for /api/analytics/*
//
// Auth: trusts X-Internal-User-ID header set by api-gateway (InternalAuth middleware).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"

	"github.com/okochadmytro/tradetracker/internal/cache"
	"github.com/okochadmytro/tradetracker/internal/config"
	"github.com/okochadmytro/tradetracker/internal/database"
	"github.com/okochadmytro/tradetracker/internal/handlers"
	"github.com/okochadmytro/tradetracker/internal/metrics"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/scheduler"
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

	// PriceService для live цін при розрахунку snapshots (читає з Redis якщо доступний)
	var priceService *services.PriceService
	if cfg.RedisURL != "" {
		redisClient := goredis.NewClient(&goredis.Options{Addr: cfg.RedisURL})
		priceService = services.NewPriceService(db, cache.NewRedisPriceStore(redisClient), logger)
		logger.Info("analytics: price cache via Redis", "url", cfg.RedisURL)
	} else {
		logger.Info("analytics: no Redis — snapshots use DB current_price")
	}

	snapshotRepo     := models.NewSnapshotRepository(db)
	analyticsService := services.NewAnalyticsService(db, snapshotRepo, priceService, logger)
	analyticsHandler := handlers.NewAnalyticsHandler(analyticsService, snapshotRepo)
	aiHandler        := handlers.NewAIHandler(db, cfg.AnthropicKey)

	// ── Snapshot scheduler ────────────────────────────────────────────────────
	// Записує snapshot вартості портфеля кожну годину.
	// При першому запуску — одразу, без очікування першого тіку.
	ctx, cancel := context.WithCancel(context.Background())
	sched := scheduler.New(logger)
	sched.Register(scheduler.Job{
		Name:     "snapshots",
		Interval: 1 * time.Minute,
		Fn: func(c context.Context) {
			analyticsService.TakeAllSnapshots(c)
		},
	})
	sched.Start(ctx)

	// Знімок одразу при старті (щоб графік відображав хоча б поточний стан)
	go analyticsService.TakeAllSnapshots(ctx)

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
	analyticsGroup.Get("/chart",     analyticsHandler.Chart)
	analyticsGroup.Post("/snapshot", analyticsHandler.TakeSnapshot)
	analyticsGroup.Get("/arbitrage", analyticsHandler.Arbitrage)

	api.Post("/ai/ask", aiHandler.Ask)

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
	cancel()
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}
