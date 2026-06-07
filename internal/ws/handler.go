package ws

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	fiberws "github.com/gofiber/websocket/v2"
)

// UpgradeMiddleware перевіряє що запит є WebSocket upgrade.
// Встановлюється перед Handler як окремий middleware.
func UpgradeMiddleware() fiber.Handler {
	return fiberws.New(func(c *fiberws.Conn) {
		// Порожній — реальна логіка в Handler
	})
}

// Handler повертає Fiber WebSocket handler.
// Реєструє клієнта в hub і запускає read/write pumps.
func Handler(hub *Hub, jwtSecret string, logger *slog.Logger) fiber.Handler {
	return fiberws.New(func(c *fiberws.Conn) {
		client := &Client{
			hub:       hub,
			conn:      c.Conn,
			send:      make(chan []byte, 64),
			jwtSecret: jwtSecret,
			logger:    logger,
		}

		hub.register(client)
		logger.Info("ws: new connection", "addr", c.RemoteAddr())

		// writePump у окремій goroutine, readPump — в поточній
		go client.WritePump()
		client.ReadPump()
	})
}
