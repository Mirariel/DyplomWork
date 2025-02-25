package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
)

type AuthHandler struct {
	users     *models.UserRepository
	jwtSecret string
}

func NewAuthHandler(users *models.UserRepository, jwtSecret string) *AuthHandler {
	return &AuthHandler{users: users, jwtSecret: jwtSecret}
}

// POST /api/auth/register
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if len(body.Username) < 3 || body.Email == "" || len(body.Password) < 6 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "username ≥3 symbols, password ≥6 symbols, email required",
		})
	}

	user, err := h.users.Create(body.Username, body.Email, body.Password)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "user already exists"})
	}

	token, err := h.generateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token generation failed"})
	}

	h.setTokenCookie(c, token)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"token":    token,
		"user_id":  user.ID,
		"username": user.Username,
	})
}

// POST /api/auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	user, err := h.users.FindByEmail(body.Email)
	if err != nil || !h.users.CheckPassword(user, body.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	token, err := h.generateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token generation failed"})
	}

	h.setTokenCookie(c, token)
	return c.JSON(fiber.Map{
		"token":    token,
		"user_id":  user.ID,
		"username": user.Username,
	})
}

// POST /api/auth/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:    "token",
		Value:   "",
		Expires: time.Now().Add(-time.Hour),
	})
	return c.JSON(fiber.Map{"status": "ok"})
}

// GET /api/auth/me
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	user, err := h.users.FindByID(userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(fiber.Map{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
	})
}

func (h *AuthHandler) generateToken(user *models.User) (string, error) {
	claims := middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

func (h *AuthHandler) setTokenCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    token,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		HTTPOnly: true,
		SameSite: "Strict",
	})
}
