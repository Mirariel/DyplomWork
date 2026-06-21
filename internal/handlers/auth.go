package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/validator"
)

type AuthHandler struct {
	users     *models.UserRepository
	jwtSecret string
}

func NewAuthHandler(users *models.UserRepository, jwtSecret string) *AuthHandler {
	return &AuthHandler{users: users, jwtSecret: jwtSecret}
}

type registerRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type loginRequest struct {
	Login    string `json:"login"    validate:"required"`
	Password string `json:"password" validate:"required"`
}

// POST /api/auth/register
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	user, err := h.users.Create(req.Username, req.Email, req.Password)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "user already exists"})
	}

	token, err := h.generateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token generation failed"})
	}

	h.setTokenCookie(c, token)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"token": token,
		"user":  user,
	})
}

// POST /api/auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	user, err := h.users.FindByEmailOrUsername(req.Login)
	if err != nil || !h.users.CheckPassword(user, req.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	token, err := h.generateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token generation failed"})
	}

	h.setTokenCookie(c, token)
	return c.JSON(fiber.Map{
		"token": token,
		"user":  user,
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
	return c.JSON(user)
}

type updateProfileRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email"    validate:"required,email"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password"     validate:"required,min=6"`
}

// PATCH /api/user/profile
func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req updateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	user, err := h.users.UpdateProfile(userID, req.Username, req.Email)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "username or email already taken"})
	}
	return c.JSON(user)
}

// PATCH /api/user/password
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req changePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	user, err := h.users.FindByID(userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	if !h.users.CheckPassword(user, req.CurrentPassword) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "incorrect current password"})
	}
	if err := h.users.UpdatePassword(userID, req.NewPassword); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update password"})
	}
	return c.JSON(fiber.Map{"status": "ok"})
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
