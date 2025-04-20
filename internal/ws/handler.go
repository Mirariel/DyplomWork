package ws

import (
	"log"

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
func Handler(hub *Hub, jwtSecret string, logger *log.Logger) fiber.Handler {
	return fiberws.New(func(c *fiberws.Conn) {
		client := &Client{
			hub:       hub,
			conn:      c.Conn, // *gorilla/websocket.Conn під капотом
			send:      make(chan []byte, 64),
			jwtSecret: jwtSecret,
			logger:    logger,
		}

		hub.register(client)
		logger.Printf("[ws] new connection from %s", c.RemoteAddr())

		// writePump у окремій goroutine, readPump — в поточній
		go client.WritePump()
		client.ReadPump()
	})
}
