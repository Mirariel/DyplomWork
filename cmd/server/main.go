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
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberws "github.com/gofiber/websocket/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"
	"github.com/okochadmytro/tradetracker/internal/cache"
	"github.com/okochadmytro/tradetracker/internal/config"
	"github.com/okochadmytro/tradetracker/internal/database"
	"github.com/okochadmytro/tradetracker/internal/handlers"
	"github.com/okochadmytro/tradetracker/internal/metrics"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/notify"
	"github.com/okochadmytro/tradetracker/internal/scheduler"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/ws"
)

func main() {
	cfg := config.Load()

	// Logger: JSON у prod, human-readable text у dev
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
	logger.Info("Database connected")

	enc, err := services.NewEncryptionService(cfg.EncryptionKey)
	if err != nil {
		logger.Error("Encryption service init failed", "error", err)
		os.Exit(1)
	}

	// Cache: автоматично вибирає Redis якщо REDIS_URL задано, інакше in-memory.
	// Щоб увімкнути Redis: додати REDIS_URL=localhost:6379 в .env
	var priceStore cache.PriceStorer
	if cfg.RedisURL != "" {
		redisClient := goredis.NewClient(&goredis.Options{Addr: cfg.RedisURL})
		priceStore = cache.NewRedisPriceStore(redisClient)
		logger.Info("price cache: Redis", "url", cfg.RedisURL)
	} else {
		priceStore = cache.NewMemoryPriceStore()
		logger.Info("price cache: in-memory")
	}

	// Repositories
	userRepo := models.NewUserRepository(db)
	portfolioRepo := models.NewPortfolioRepository(db)
	orderRepo := models.NewOrderRepository(db)
	smartOrderRepo := models.NewSmartOrderRepository(db)
	botRepo := models.NewBotRepository(db)
	snapshotRepo := models.NewSnapshotRepository(db)
	dcaBotRepo := models.NewDCABotRepository(db)

	// Notifier (Telegram) — no-op якщо TELEGRAM_BOT_TOKEN або TELEGRAM_CHAT_ID не задані
	notifier := notify.New(cfg.TelegramToken, cfg.TelegramChatID, logger)
	if notifier.Enabled() {
		logger.Info("telegram notifications: enabled")
	} else {
		logger.Info("telegram notifications: disabled (set TELEGRAM_BOT_TOKEN + TELEGRAM_CHAT_ID to enable)")
	}

	// Services
	syncService := services.NewSyncService(db, enc, logger)
	priceService := services.NewPriceService(db, priceStore, logger)
	smartOrderService := services.NewSmartOrderService(db, smartOrderRepo, orderRepo, portfolioRepo, priceService, enc, notifier, logger)
	botService := services.NewBotService(botRepo, portfolioRepo, enc, logger)
	analyticsService := services.NewAnalyticsService(db, snapshotRepo, priceService, logger)
	dcaService := services.NewDCAService(dcaBotRepo, portfolioRepo, priceService, enc, notifier, logger)

	// WebSocket
	hub := ws.NewHub()
	wsServer := ws.NewServer(hub, db, enc, priceService, nil, cfg.JWTSecret, logger)

	// Контекст для WS broadcast loop і scheduler
	ctx, cancel := context.WithCancel(context.Background())

	go wsServer.Run(ctx)

	// Background scheduler
	sched := scheduler.New(logger)
	sched.Register(
		scheduler.Job{
			Name:     "prices",
			Interval: 30 * time.Second,
			Fn: func(ctx context.Context) {
				if err := priceService.UpdateAllAssets(); err != nil {
					logger.Error("scheduler: update prices failed", "error", err)
				}
			},
		},
		scheduler.Job{
			Name:     "sync-all-users",
			Interval: 15 * time.Minute,
			Fn: func(ctx context.Context) {
				syncService.SyncAllUsers()
			},
		},
		scheduler.Job{
			Name:     "smart-orders",
			Interval: 5 * time.Second,
			Fn:       smartOrderService.CheckAndTrigger,
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
		// Оновлення Prometheus gauge'ів кожні 30 секунд
		scheduler.Job{
			Name:     "metrics-update",
			Interval: 30 * time.Second,
			Fn: func(_ context.Context) {
				metrics.WSClientsConnected.Set(float64(hub.ClientCount()))

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
	sched.Start(ctx)

	// Handlers
	authHandler := handlers.NewAuthHandler(userRepo, cfg.JWTSecret)
	portfolioHandler := handlers.NewPortfolioHandler(portfolioRepo, enc)
	syncHandler := handlers.NewSyncHandler(syncService, priceService, portfolioRepo, analyticsService)
	orderHandler := handlers.NewOrderHandler(orderRepo, portfolioRepo, enc, logger)
	smartOrderHandler := handlers.NewSmartOrderHandler(smartOrderRepo)
	botHandler := handlers.NewBotHandler(botRepo, botService, priceService)
	analyticsHandler := handlers.NewAnalyticsHandler(analyticsService, snapshotRepo)
	dcaHandler := handlers.NewDCAHandler(dcaBotRepo, dcaService)

	// App
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
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000,http://localhost:5173",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	// --- Rate limiter для auth ---
	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "too many requests, try again in a minute",
			})
		},
	})

	// --- Public routes ---
	auth := app.Group("/api/auth")
	auth.Post("/register", authLimiter, authHandler.Register)
	auth.Post("/login", authLimiter, authHandler.Login)
	auth.Post("/logout", authHandler.Logout)

	// --- Protected routes ---
	api := app.Group("/api", middleware.JWTAuth(cfg.JWTSecret))

	api.Get("/auth/me", authHandler.Me)
	api.Patch("/user/profile", authHandler.UpdateProfile)
	api.Patch("/user/password", authHandler.ChangePassword)

	// Portfolio
	portfolio := api.Group("/portfolio")
	portfolio.Get("/", portfolioHandler.GetPortfolio)
	portfolio.Get("/history", portfolioHandler.GetHistory)
	portfolio.Get("/credentials", portfolioHandler.GetCredentials)
	portfolio.Post("/credentials", portfolioHandler.AddCredential)
	portfolio.Delete("/credentials/:id", portfolioHandler.DeleteCredential)

	// Comments
	api.Patch("/positions/:id/comment", portfolioHandler.UpdatePositionComment)
	api.Patch("/history/:id/comment", portfolioHandler.UpdateHistoryComment)

	// Sync
	syncGroup := api.Group("/sync")
	syncGroup.Post("/full", syncHandler.FullSync)
	syncGroup.Post("/positions", syncHandler.SyncPositions)
	syncGroup.Post("/history", syncHandler.SyncHistory)
	syncGroup.Get("/prices", syncHandler.UpdatePrices)

	// Orders
	api.Post("/orders", orderHandler.PlaceOrder)
	api.Get("/orders", orderHandler.ListOrders)
	api.Get("/orders/:id", orderHandler.GetOrder)
	api.Delete("/orders/:id", orderHandler.CancelOrder)

	// Smart Orders
	api.Post("/smart-orders", smartOrderHandler.Create)
	api.Get("/smart-orders", smartOrderHandler.List)
	api.Get("/smart-orders/:id", smartOrderHandler.Get)
	api.Delete("/smart-orders/:id", smartOrderHandler.Cancel)

	// Grid Bots
	api.Post("/bots", botHandler.Create)
	api.Get("/bots", botHandler.List)
	api.Get("/bots/:id", botHandler.Get)
	api.Post("/bots/:id/start", botHandler.Start)
	api.Post("/bots/:id/stop", botHandler.Stop)
	api.Delete("/bots/:id", botHandler.Delete)

	// Analytics
	analyticsGroup := api.Group("/analytics")
	analyticsGroup.Get("/summary",   analyticsHandler.Summary)
	analyticsGroup.Get("/coins",     analyticsHandler.Coins)
	analyticsGroup.Get("/snapshots", analyticsHandler.Snapshots)
	analyticsGroup.Get("/chart",     analyticsHandler.Chart)
	analyticsGroup.Post("/snapshot", analyticsHandler.TakeSnapshot)
	analyticsGroup.Get("/arbitrage", analyticsHandler.Arbitrage)

	// DCA Bots
	api.Post("/dca", dcaHandler.Create)
	api.Get("/dca", dcaHandler.List)
	api.Get("/dca/:id", dcaHandler.Get)
	api.Post("/dca/:id/start", dcaHandler.Start)
	api.Post("/dca/:id/stop", dcaHandler.Stop)
	api.Delete("/dca/:id", dcaHandler.Delete)

	// WebSocket
	app.Use("/ws", func(c *fiber.Ctx) error {
		if fiberws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", ws.Handler(hub, cfg.JWTSecret, logger))

	// Health
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "version": "0.1.0"})
	})

	// Prometheus metrics — виключаємо із збору власних метрик через adaptor
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// --- Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "port", cfg.AppPort, "ws", "ws://localhost:"+cfg.AppPort+"/ws")
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			logger.Error("server error", "error", err)
		}
	}()

	<-quit
	logger.Info("shutting down server...")
	cancel() // зупиняє WS broadcast loop і всі scheduler jobs

	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		logger.Error("shutdown error", "error", err)
	}
	logger.Info("server stopped")
}
