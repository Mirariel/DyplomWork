package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services"
	"github.com/okochadmytro/tradetracker/internal/validator"
)

type PortfolioHandler struct {
	repo               *models.PortfolioRepository
	encryption         *services.EncryptionService
	onCredentialAdded  func(userID int64) // опціональний хук для auto-discovery
}

func NewPortfolioHandler(repo *models.PortfolioRepository, enc *services.EncryptionService) *PortfolioHandler {
	return &PortfolioHandler{repo: repo, encryption: enc}
}

// SetCredentialHook задає функцію, що викликається асинхронно після додавання credentials.
func (h *PortfolioHandler) SetCredentialHook(fn func(userID int64)) {
	h.onCredentialAdded = fn
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

type addCredentialRequest struct {
	Exchange   string `json:"exchange"   validate:"required"`
	Label      string `json:"label"`
	APIKey     string `json:"api_key"    validate:"required,min=16"`
	APISecret  string `json:"api_secret" validate:"required,min=16"`
	Passphrase string `json:"passphrase"`
}

// POST /api/portfolio/credentials
func (h *PortfolioHandler) AddCredential(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var body addCredentialRequest
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&body); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	encKey, err := h.encryption.Encrypt(body.APIKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "encryption failed"})
	}
	encSecret, err := h.encryption.Encrypt(body.APISecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "encryption failed"})
	}

	// Зберігаємо лише перші 8 символів ключа як hint (безпечно показувати в UI)
	hint := body.APIKey
	if len(hint) > 8 {
		hint = hint[:4] + "••••" + hint[len(hint)-4:]
	}

	cred := &models.ExternalApiCredential{
		UserID:             userID,
		Exchange:           body.Exchange,
		Label:              body.Label,
		ApiKeyHint:         hint,
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
		cred.HasPassphrase = true
	}

	if err := h.repo.UpsertCredential(cred); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Auto-discovery: асинхронно синкуємо ф'ючерсні позиції для нового ключа
	if h.onCredentialAdded != nil {
		go h.onCredentialAdded(userID)
	}

	creds, err := h.repo.GetCredentials(userID)
	if err != nil || len(creds) == 0 {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "ok"})
	}
	// Повертаємо щойно додану credential (остання за created_at)
	return c.Status(fiber.StatusCreated).JSON(creds[len(creds)-1])
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

// PATCH /api/portfolio/assets/:id/price
func (h *PortfolioHandler) UpdateAssetPrice(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var body struct {
		AvgBuyPrice float64 `json:"avg_buy_price"`
		ManuallySet bool    `json:"manually_set"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if err := h.repo.UpdateAssetPrice(int64(id), userID, body.AvgBuyPrice, body.ManuallySet); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}
