// trading owns all trading operations: portfolio, orders, smart orders, bots, DCA.
//
// Responsibilities:
//   - REST handlers for portfolio / orders / smart-orders / bots / dca
//   - Schedulers: grid-bots (10 s), DCA (5 min), portfolio snapshots (1 h)
//   - Subscribes to NATS market.prices.updated → triggers smart-order checks
//
// Auth: trusts X-Internal-User-ID header set by api-gateway (InternalAuth middleware).
package main

import (
	"context"
	"encoding/json"
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
	"github.com/okochadmytro/tradetracker/internal/notify"
	"github.com/okochadmytro/tradetracker/internal/scheduler"
	"github.com/okochadmytro/tradetracker/internal/services"
)

func main() {
	cfg := config.Load()
	if cfg.AppPort == "8080" {
		cfg.AppPort = "8082" // default for trading
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
	} else {
		priceStore = cache.NewMemoryPriceStore()
	}

	// Repositories
	portfolioRepo    := models.NewPortfolioRepository(db)
	orderRepo        := models.NewOrderRepository(db)
	smartOrderRepo   := models.NewSmartOrderRepository(db)
	botRepo          := models.NewBotRepository(db)
	snapshotRepo     := models.NewSnapshotRepository(db)
	dcaBotRepo       := models.NewDCABotRepository(db)
	futuresRepo      := models.NewFuturesPositionRepository(db)
	spotTradeRepo    := models.NewSpotTradeRepository(db)

	notifier    := notify.New(cfg.TelegramToken, cfg.TelegramChatID, logger)
	priceService := services.NewPriceService(db, priceStore, logger)

	smartOrderService := services.NewSmartOrderService(db, smartOrderRepo, orderRepo, portfolioRepo, priceService, enc, notifier, logger)
	botService        := services.NewBotService(botRepo, portfolioRepo, enc, logger)
	analyticsService  := services.NewAnalyticsService(db, snapshotRepo, priceService, logger)
	dcaService        := services.NewDCAService(dcaBotRepo, portfolioRepo, priceService, enc, notifier, logger)

	syncService := services.NewSyncService(db, enc, logger)

	// Handlers
	portfolioHandler  := handlers.NewPortfolioHandler(portfolioRepo, enc)
	portfolioHandler.SetCredentialHook(func(userID int64) {
		if err := syncService.SyncFuturesForUser(userID); err != nil {
			logger.Error("auto-discovery: futures sync failed", "user_id", userID, "error", err)
		}
	})
	orderHandler      := handlers.NewOrderHandler(orderRepo, portfolioRepo, enc, logger)
	smartOrderHandler := handlers.NewSmartOrderHandler(smartOrderRepo)
	botHandler        := handlers.NewBotHandler(botRepo, botService, priceService)
	dcaHandler        := handlers.NewDCAHandler(dcaBotRepo, dcaService)
	futuresHandler    := handlers.NewFuturesHandler(futuresRepo)
	spotTradesHandler := handlers.NewSpotTradesHandler(spotTradeRepo)

	// ── NATS: subscribe to price updates → trigger smart-order checks ─────────
	bus, err := natspkg.Connect(cfg.NatsURL, logger)
	if err != nil {
		// Non-fatal: fall back to timer-based check
		logger.Warn("NATS unavailable, smart-order checks will use fallback timer", "error", err)
		bus = nil
	} else {
		defer bus.Close()
	}

	// ── Scheduler ─────────────────────────────────────────────────────────────
	sched := scheduler.New(logger)

	// Smart orders: event-driven via NATS; timer is a safety fallback
	sched.Register(scheduler.Job{
		Name:     "smart-orders-fallback",
		Interval: 30 * time.Second,
		Fn:       smartOrderService.CheckAndTrigger,
	})

	sched.Register(
		scheduler.Job{
			Name:     "futures-sync",
			Interval: 30 * time.Second,
			Fn: func(ctx context.Context) {
				syncService.SyncFuturesAllUsers()
			},
		},
		scheduler.Job{
			Name:     "grid-bots",
			Interval: 10 * time.Second,
			Fn:       botService.CheckBots,
		},
		scheduler.Job{
			Name:     "portfolio-snapshots",
			Interval: 1 * time.Hour,
			Fn:       analyticsService.TakeAllSnapshots,
		},
		scheduler.Job{
			Name:     "dca-bots",
			Interval: 5 * time.Minute,
			Fn:       dcaService.CheckAndBuy,
		},
		scheduler.Job{
			Name:     "metrics-update",
			Interval: 30 * time.Second,
			Fn: func(_ context.Context) {
				var runningBots, runningDCA, activeSO int64
				_ = db.Get(&runningBots, `SELECT COUNT(*) FROM bots WHERE status = 'running'`)
				_ = db.Get(&runningDCA, `SELECT COUNT(*) FROM dca_bots WHERE status = 'running'`)
				_ = db.Get(&activeSO, `SELECT COUNT(*) FROM smart_orders WHERE status = 'active'`)
				metrics.ActiveGridBots.Set(float64(runningBots))
				metrics.ActiveDCABots.Set(float64(runningDCA))
				metrics.ActiveSmartOrders.Set(float64(activeSO))
			},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	sched.Start(ctx)

	// Subscribe to NATS price updates for instant smart-order evaluation
	if bus != nil {
		if _, err := bus.Subscribe(natspkg.SubjPricesUpdated, func(data []byte) {
			var msg natspkg.PricesMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				return
			}
			logger.Debug("trading: prices updated, checking smart orders",
				"symbols", len(msg.Prices))
			smartOrderService.CheckAndTrigger(ctx)
		}); err != nil {
			logger.Warn("NATS subscribe failed", "error", err)
		}
	}

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

	// Portfolio
	portfolio := api.Group("/portfolio")
	portfolio.Get("/",            portfolioHandler.GetPortfolio)
	portfolio.Get("/history",     portfolioHandler.GetHistory)
	portfolio.Get("/credentials", portfolioHandler.GetCredentials)
	portfolio.Post("/credentials", portfolioHandler.AddCredential)
	portfolio.Delete("/credentials/:id", portfolioHandler.DeleteCredential)

	api.Patch("/positions/:id/comment",         portfolioHandler.UpdatePositionComment)
	api.Patch("/history/:id/comment",           portfolioHandler.UpdateHistoryComment)
	api.Patch("/portfolio/assets/:id/price",    portfolioHandler.UpdateAssetPrice)

	// Orders
	api.Post("/orders",      orderHandler.PlaceOrder)
	api.Get("/orders",       orderHandler.ListOrders)
	api.Get("/orders/:id",   orderHandler.GetOrder)
	api.Delete("/orders/:id", orderHandler.CancelOrder)

	// Smart Orders
	api.Post("/smart-orders",      smartOrderHandler.Create)
	api.Get("/smart-orders",       smartOrderHandler.List)
	api.Get("/smart-orders/:id",   smartOrderHandler.Get)
	api.Delete("/smart-orders/:id", smartOrderHandler.Cancel)

	// Grid Bots
	api.Post("/bots",          botHandler.Create)
	api.Get("/bots",           botHandler.List)
	api.Get("/bots/:id",       botHandler.Get)
	api.Post("/bots/:id/start", botHandler.Start)
	api.Post("/bots/:id/stop",  botHandler.Stop)
	api.Delete("/bots/:id",    botHandler.Delete)

	// DCA Bots
	api.Post("/dca",           dcaHandler.Create)
	api.Get("/dca",            dcaHandler.List)
	api.Get("/dca/:id",        dcaHandler.Get)
	api.Post("/dca/:id/start", dcaHandler.Start)
	api.Post("/dca/:id/stop",  dcaHandler.Stop)
	api.Delete("/dca/:id",     dcaHandler.Delete)

	// Futures Positions
	api.Get("/futures/positions", futuresHandler.GetPositions)

	// Spot Trades
	api.Get("/spot-trades", spotTradesHandler.GetTrades)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "trading"})
	})
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("trading starting", "port", cfg.AppPort)
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			logger.Error("server error", "error", err)
		}
	}()

	<-quit
	logger.Info("trading shutting down")
	cancel()
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}
