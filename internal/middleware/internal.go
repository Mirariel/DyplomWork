package middleware

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// InternalAuth reads the user ID from the X-Internal-User-ID header that
// api-gateway injects after validating the JWT. Downstream services (market-data,
// trading, analytics) use this instead of parsing JWT themselves.
//
// The header is set by the gateway proxy and must never be trusted from the
// external network — downstream services should not be reachable externally.
func InternalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Get("X-Internal-User-ID")
		if raw == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || userID == 0 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid user id"})
		}
		c.Locals("user_id", userID)
		return c.Next()
	}
}
