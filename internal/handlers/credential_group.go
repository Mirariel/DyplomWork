package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/okochadmytro/tradetracker/internal/middleware"
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/validator"
)

// CredentialGroupHandler — CRUD для груп API ключів.
type CredentialGroupHandler struct {
	groups *models.CredentialGroupRepository
}

func NewCredentialGroupHandler(groups *models.CredentialGroupRepository) *CredentialGroupHandler {
	return &CredentialGroupHandler{groups: groups}
}

type createGroupRequest struct {
	Name      string  `json:"name"       validate:"required,min=1,max=100"`
	MemberIDs []int64 `json:"member_ids"`
}

// POST /api/credential-groups
func (h *CredentialGroupHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req createGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := validator.Validate(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	g := &models.CredentialGroup{UserID: userID, Name: req.Name}
	if err := h.groups.Create(g); err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "group name already exists"})
	}

	if len(req.MemberIDs) > 0 {
		if err := h.groups.SetMembers(g.ID, req.MemberIDs); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         g.ID,
		"name":       g.Name,
		"member_ids": req.MemberIDs,
	})
}

// GET /api/credential-groups
func (h *CredentialGroupHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	groups, err := h.groups.ListByUser(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(groups)
}

// DELETE /api/credential-groups/:id
func (h *CredentialGroupHandler) Delete(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.groups.Delete(int64(id), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

type updateMembersRequest struct {
	MemberIDs []int64 `json:"member_ids" validate:"required"`
}

// PUT /api/credential-groups/:id/members
func (h *CredentialGroupHandler) SetMembers(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var req updateMembersRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if err := h.groups.SetMembers(int64(id), req.MemberIDs); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}
