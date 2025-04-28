package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberws "github.com/gofiber/websocket/v2"
	"github.com/okochadmytro/tradetracker/internal/config"
	"github.com/okochadmytro/tradetracker/internal/database"
	"github.com/okochadmytro/tradetracker/internal/handlers"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/ws"
)

func main() {
	cfg := config.Load()
	logger := log.New(os.Stdout, "", log.LstdFlags)

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer db.Close()
	logger.Println("Database connected")

	enc, err := services.NewEncryptionService(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Encryption service: %v", err)
	}

	// Services
	syncService := services.NewSyncService(db, enc, logger)
	priceService := services.NewPriceService(db, logger)

	// Repositories
	userRepo := models.NewUserRepository(db)
	portfolioRepo := models.NewPortfolioRepository(db)

	// WebSocket
	hub := ws.NewHub()
	wsServer := ws.NewServer(hub, db, enc, priceService, cfg.JWTSecret, logger)
	go wsServer.Run() // broadcast loop

	// Handlers
	authHandler := handlers.NewAuthHandler(userRepo, cfg.JWTSecret)
	portfolioHandler := handlers.NewPortfolioHandler(portfolioRepo, enc)
	syncHandler := handlers.NewSyncHandler(syncService, priceService, portfolioRepo)

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
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000,http://localhost:5173",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	// --- Public routes ---
	auth := app.Group("/api/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/logout", authHandler.Logout)

	// --- Protected routes ---
	api := app.Group("/api", middleware.JWTAuth(cfg.JWTSecret))

	api.Get("/auth/me", authHandler.Me)

	// Portfolio (read)
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

	// WebSocket — upgrade check перед handler
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

	logger.Printf("Server starting on :%s (WebSocket: ws://localhost:%s/ws)", cfg.AppPort, cfg.AppPort)
	if err := app.Listen(":" + cfg.AppPort); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
