// market-data owns exchange integration: price fetching and portfolio sync.
//
// Responsibilities:
//   - REST handlers for /api/sync/* (proxied from api-gateway)
//   - 30-second price scheduler → updates Redis + publishes NATS market.prices.updated
//   - 15-minute full user sync scheduler
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
	natspkg "github.com/okochadmytro/tradetracker/internal/nats"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/scheduler"
	"github.com/okochadmytro/tradetracker/internal/services"
)

func main() {
	cfg := config.Load()
	if cfg.AppPort == "8080" {
		cfg.AppPort = "8081" // default for market-data
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

	enc, err := services.NewEncryptionService(cfg.EncryptionKey)
	if err != nil {
		logger.Error("Encryption init failed", "error", err)
		os.Exit(1)
	}

	var priceStore cache.PriceStorer
	if cfg.RedisURL != "" {
		priceStore = cache.NewRedisPriceStore(goredis.NewClient(&goredis.Options{Addr: cfg.RedisURL}))
		logger.Info("price cache: Redis", "url", cfg.RedisURL)
	} else {
		priceStore = cache.NewMemoryPriceStore()
		logger.Info("price cache: in-memory (prices won't reach api-gateway WS without Redis)")
	}

	portfolioRepo := models.NewPortfolioRepository(db)
	syncService   := services.NewSyncService(db, enc, logger)
	priceService  := services.NewPriceService(db, priceStore, logger)

	// NATS — publish price updates so trading can trigger smart-order checks
	bus, err := natspkg.Connect(cfg.NatsURL, logger)
	if err != nil {
		logger.Error("NATS connection failed", "error", err)
		os.Exit(1)
	}
	defer bus.Close()

	// ── Handlers ──────────────────────────────────────────────────────────────
	syncHandler := handlers.NewSyncHandler(syncService, priceService, portfolioRepo)

	// ── Scheduler ─────────────────────────────────────────────────────────────
	sched := scheduler.New(logger)
	sched.Register(
		scheduler.Job{
			Name:     "prices",
			Interval: 30 * time.Second,
			Fn: func(ctx context.Context) {
				if err := priceService.UpdateAllAssets(); err != nil {
					logger.Error("scheduler: update prices failed", "error", err)
					return
				}
				// Publish NATS event so trading service knows prices are fresh
				prices := priceService.GetLivePrices(nil)
				if err := bus.Publish(natspkg.SubjPricesUpdated, natspkg.PricesMsg{
					Prices: prices,
					At:     time.Now(),
				}); err != nil {
					logger.Warn("nats: publish prices failed", "error", err)
				}
			},
		},
		scheduler.Job{
			Name:     "sync-all-users",
			Interval: 15 * time.Minute,
			Fn: func(_ context.Context) {
				syncService.SyncAllUsers()
			},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	sched.Start(ctx)

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
	syncGroup := api.Group("/sync")
	syncGroup.Post("/full",      syncHandler.FullSync)
	syncGroup.Post("/positions", syncHandler.SyncPositions)
	syncGroup.Post("/history",   syncHandler.SyncHistory)
	syncGroup.Get("/prices",     syncHandler.UpdatePrices)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "market-data"})
	})
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("market-data starting", "port", cfg.AppPort)
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			logger.Error("server error", "error", err)
		}
	}()

	<-quit
	logger.Info("market-data shutting down")
	cancel()
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}
