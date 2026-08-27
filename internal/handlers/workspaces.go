package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"shipr-backend/internal/database"
	"shipr-backend/internal/middleware"
)

type WorkspaceHandler struct {
	db *database.Queries
}

func NewWorkspaceHandler(db *database.Queries) *WorkspaceHandler {
	return &WorkspaceHandler{db: db}
}

type CreateWorkspaceRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *WorkspaceHandler) ListWorkspaces(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	workspaces, err := h.db.ListWorkspacesForUser(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list workspaces"})
	}
	return c.JSON(workspaces)
}

func (h *WorkspaceHandler) CreateWorkspace(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req CreateWorkspaceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Workspace name is required"})
	}

	slug := req.Slug
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}

	workspace, err := h.db.CreateWorkspace(c.Context(), database.CreateWorkspaceParams{
		Name:    req.Name,
		Slug:    slug + "-" + uuid.New().String()[:4],
		OwnerID: userID,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create workspace"})
	}

	// Add user as owner member
	_, _ = h.db.AddWorkspaceMember(c.Context(), workspace.ID, userID, database.WorkspaceRoleOwner)

	return c.Status(fiber.StatusCreated).JSON(workspace)
}

func (h *WorkspaceHandler) GetWorkspace(c *fiber.Ctx) error {
	workspaceIDStr := c.Params("workspaceId")
	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid workspace ID"})
	}

	workspace, err := h.db.GetWorkspaceByID(c.Context(), workspaceID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Workspace not found"})
	}

	projects, _ := h.db.ListProjectsByWorkspaceID(c.Context(), workspaceID)

	return c.JSON(fiber.Map{
		"workspace": workspace,
		"projects":  projects,
	})
}
