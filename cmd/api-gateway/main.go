// api-gateway is the single external entry point.
//
// Responsibilities:
//   - JWT validation + rate limiting
//   - Auth / user-profile handlers (direct, no proxy)
//   - WebSocket hub (real-time positions & prices)
//   - HTTP reverse-proxy to market-data, trading, analytics
//   - Prometheus metrics & CORS
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	natspkg "github.com/okochadmytro/tradetracker/internal/nats"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/ws"
)

func main() {
	cfg := config.Load()

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

	// Price cache (Redis) — shared with market-data which writes to it.
	var redisClient *goredis.Client
	var priceStore cache.PriceStorer
	if cfg.RedisURL != "" {
		redisClient = goredis.NewClient(&goredis.Options{Addr: cfg.RedisURL})
		priceStore = cache.NewRedisPriceStore(redisClient)
		logger.Info("price cache: Redis", "url", cfg.RedisURL)
	} else {
		priceStore = cache.NewMemoryPriceStore()
		logger.Info("price cache: in-memory")
	}

	userRepo     := models.NewUserRepository(db)
	priceService := services.NewPriceService(db, priceStore, logger)
	topSvc       := services.NewTopSymbolsService(redisClient, logger)

	// ── Handlers served directly by the gateway ──────────────────────────────
	authHandler := handlers.NewAuthHandler(userRepo, cfg.JWTSecret)

	// ── WebSocket ─────────────────────────────────────────────────────────────
	hub      := ws.NewHub()
	wsServer := ws.NewServer(hub, db, enc, priceService, topSvc, cfg.JWTSecret, logger)

	ctx, cancel := context.WithCancel(context.Background())
	go wsServer.Run(ctx)

	// ── NATS — heartbeat active users + listen for portfolio.synced ───────────
	bus, err := natspkg.Connect(cfg.NatsURL, logger)
	if err != nil {
		logger.Error("NATS connection failed", "error", err)
		os.Exit(1)
	}
	defer bus.Close()

	// Publish active user IDs every 10 seconds so market-data knows who to sync frequently.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ids := hub.ActiveUserIDs()
				if err := bus.Publish(natspkg.SubjActiveUsers, natspkg.ActiveUsersMsg{
					UserIDs: ids,
					At:      time.Now(),
				}); err != nil {
					logger.Warn("nats: publish active-users failed", "error", err)
				}
			}
		}
	}()

	// Subscribe to portfolio.synced — push {type:"synced"} to user's WS.
	if _, err := bus.Subscribe(natspkg.SubjPortfolioSynced, func(data []byte) {
		var msg natspkg.PortfolioSyncedMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}
		payload, _ := json.Marshal(map[string]string{"type": "synced", "kind": msg.Kind})
		hub.SendToUser(msg.UserID, payload)
	}); err != nil {
		logger.Warn("nats: subscribe portfolio.synced failed", "error", err)
	}

	// ── HTTP reverse-proxy helper ──────────────────────────────────────────────
	proxyClient := &http.Client{Timeout: 30 * time.Second}
	makeProxy := func(targetBase string) fiber.Handler {
		return func(c *fiber.Ctx) error {
			url := targetBase + c.OriginalURL()

			req, err := http.NewRequestWithContext(
				context.Background(),
				c.Method(),
				url,
				bytes.NewReader(c.Body()),
			)
			if err != nil {
				return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "proxy error"})
			}

			// Forward request headers (skip Host to avoid routing issues)
			c.Request().Header.VisitAll(func(key, val []byte) {
				if !strings.EqualFold(string(key), "Host") {
					req.Header.Set(string(key), string(val))
				}
			})

			// Inject authenticated user ID so downstream services don't need JWT
			if uid := middleware.GetUserID(c); uid > 0 {
				req.Header.Set("X-Internal-User-ID", strconv.FormatInt(uid, 10))
			}

			resp, err := proxyClient.Do(req)
			if err != nil {
				return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "upstream unavailable"})
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			c.Status(resp.StatusCode)
			if ct := resp.Header.Get("Content-Type"); ct != "" {
				c.Set("Content-Type", ct)
			}
			return c.Send(body)
		}
	}

	mdProxy := makeProxy(cfg.MarketDataURL)
	trProxy := makeProxy(cfg.TradingURL)
	anProxy := makeProxy(cfg.AnalyticsURL)

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
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000,http://localhost:5173",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "too many requests, try again in a minute",
			})
		},
	})

	// ── Public auth routes ────────────────────────────────────────────────────
	auth := app.Group("/api/auth")
	auth.Post("/register", authLimiter, authHandler.Register)
	auth.Post("/login", authLimiter, authHandler.Login)
	auth.Post("/logout", authHandler.Logout)

	// ── Protected routes ──────────────────────────────────────────────────────
	api := app.Group("/api", middleware.JWTAuth(cfg.JWTSecret))

	// Direct: auth & profile
	api.Get("/auth/me", authHandler.Me)
	api.Patch("/user/profile", authHandler.UpdateProfile)
	api.Patch("/user/password", authHandler.ChangePassword)

	// Proxy: market-data
	api.All("/sync/*", mdProxy)
	api.Get("/market/top-symbols", mdProxy)

	// Proxy: trading
	api.All("/portfolio*",    trProxy)
	api.All("/positions/*",   trProxy)
	api.All("/history/*",     trProxy)
	api.All("/orders*",       trProxy)
	api.All("/smart-orders*",       trProxy)
	api.All("/credential-groups*", trProxy)
	api.All("/bots*",              trProxy)
	api.All("/dca*",          trProxy)
	api.All("/futures*",      trProxy)
	api.All("/spot-trades*",  trProxy)

	// Proxy: analytics
	api.All("/analytics*", anProxy)
	api.All("/ai/*",       anProxy)

	// ── WebSocket ─────────────────────────────────────────────────────────────
	app.Use("/ws", func(c *fiber.Ctx) error {
		if fiberws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", ws.Handler(hub, cfg.JWTSecret, logger))

	// ── Infra endpoints ───────────────────────────────────────────────────────
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "api-gateway"})
	})
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("api-gateway starting", "port", cfg.AppPort)
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			logger.Error("server error", "error", err)
		}
	}()

	<-quit
	logger.Info("api-gateway shutting down")
	cancel()
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}
