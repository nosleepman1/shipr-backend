package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"shipr-backend/internal/database"
)

type ProjectHandler struct {
	db *database.Queries
}

func NewProjectHandler(db *database.Queries) *ProjectHandler {
	return &ProjectHandler{db: db}
}

type CreateProjectRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *ProjectHandler) CreateProject(c *fiber.Ctx) error {
	workspaceIDStr := c.Params("workspaceId")
	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid workspace ID"})
	}

	var req CreateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Project name is required"})
	}

	slug := req.Slug
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}

	project, err := h.db.CreateProject(c.Context(), workspaceID, req.Name, slug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create project: " + err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(project)
}

func (h *ProjectHandler) GetProject(c *fiber.Ctx) error {
	projectIDStr := c.Params("projectId")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid project ID"})
	}

	project, err := h.db.GetProjectByID(c.Context(), projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Project not found"})
	}

	apps, _ := h.db.ListApplicationsByProjectID(c.Context(), projectID)

	return c.JSON(fiber.Map{
		"project":      project,
		"applications": apps,
	})
}
