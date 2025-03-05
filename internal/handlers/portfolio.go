package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
)

type PortfolioHandler struct {
	repo       *models.PortfolioRepository
	encryption *services.EncryptionService
}

func NewPortfolioHandler(repo *models.PortfolioRepository, enc *services.EncryptionService) *PortfolioHandler {
	return &PortfolioHandler{repo: repo, encryption: enc}
}

// GET /api/portfolio
func (h *PortfolioHandler) GetPortfolio(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	assets, err := h.repo.GetUserAssets(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	positions, err := h.repo.GetOpenPositions(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	history, err := h.repo.GetHistory(userID, 50, 0)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	totalValue := 0.0
	for _, a := range assets {
		totalValue += a.Quantity * a.CurrentPrice
	}

	return c.JSON(fiber.Map{
		"assets":      assets,
		"positions":   positions,
		"history":     history,
		"total_value": totalValue,
	})
}

// GET /api/portfolio/history?limit=15&offset=0
func (h *PortfolioHandler) GetHistory(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 15)
	offset := c.QueryInt("offset", 0)

	history, err := h.repo.GetHistory(userID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(history)
}

// GET /api/portfolio/credentials
func (h *PortfolioHandler) GetCredentials(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	creds, err := h.repo.GetCredentials(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(creds)
}

// POST /api/portfolio/credentials
func (h *PortfolioHandler) AddCredential(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var body struct {
		Exchange   string `json:"exchange"`
		APIKey     string `json:"api_key"`
		APISecret  string `json:"api_secret"`
		Passphrase string `json:"passphrase"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Exchange == "" || len(body.APIKey) < 16 || len(body.APISecret) < 16 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "exchange, api_key (≥16), api_secret (≥16) are required",
		})
	}

	encKey, err := h.encryption.Encrypt(body.APIKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "encryption failed"})
	}
	encSecret, err := h.encryption.Encrypt(body.APISecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "encryption failed"})
	}

	cred := &models.ExternalApiCredential{
		UserID:             userID,
		Exchange:           body.Exchange,
		ApiKeyEncrypted:    encKey,
		ApiSecretEncrypted: encSecret,
		IsActive:           true,
	}

	if body.Passphrase != "" {
		encPass, err := h.encryption.Encrypt(body.Passphrase)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "encryption failed"})
		}
		cred.PassphraseEncrypted = &encPass
	}

	if err := h.repo.UpsertCredential(cred); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "ok"})
}

// DELETE /api/portfolio/credentials/:id
func (h *PortfolioHandler) DeleteCredential(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	if err := h.repo.DeleteCredential(int64(id), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// PATCH /api/positions/:id/comment
func (h *PortfolioHandler) UpdatePositionComment(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var body struct {
		Comment *string `json:"comment"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if err := h.repo.UpdatePositionComment(int64(id), userID, body.Comment); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// PATCH /api/history/:id/comment
func (h *PortfolioHandler) UpdateHistoryComment(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var body struct {
		Comment *string `json:"comment"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if err := h.repo.UpdateHistoryComment(int64(id), userID, body.Comment); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}
