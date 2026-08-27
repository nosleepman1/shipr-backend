package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"shipr-backend/internal/config"
	"shipr-backend/internal/database"
)

type DomainHandler struct {
	cfg *config.Config
	db  *database.Queries
}

func NewDomainHandler(cfg *config.Config, db *database.Queries) *DomainHandler {
	return &DomainHandler{cfg: cfg, db: db}
}

// CheckDomain is the Caddy On-Demand TLS verification endpoint
// Caddy calls GET /api/v1/domains/check?domain=subdomain.example.com
func (h *DomainHandler) CheckDomain(c *fiber.Ctx) error {
	domain := strings.TrimSpace(strings.ToLower(c.Query("domain")))
	if domain == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Always allow base domain subdomains
	if strings.HasSuffix(domain, "."+h.cfg.BaseDomain) || domain == h.cfg.BaseDomain {
		return c.SendStatus(fiber.StatusOK)
	}

	// Check custom domains in database
	_, err := h.db.GetDomainByName(c.Context(), domain)
	if err != nil {
		return c.SendStatus(fiber.StatusNotFound)
	}

	return c.SendStatus(fiber.StatusOK)
}

type AddCustomDomainRequest struct {
	Domain string `json:"domain"`
}

func (h *DomainHandler) AddDomain(c *fiber.Ctx) error {
	appIDStr := c.Params("appId")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid app ID"})
	}

	var req AddCustomDomainRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	domainStr := strings.TrimSpace(strings.ToLower(req.Domain))
	if domainStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Domain is required"})
	}

	isSub := false
	sslStatus := "pending"
	domain, err := h.db.CreateDomain(c.Context(), appID, domainStr, &isSub, &sslStatus)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Domain already registered: " + err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(domain)
}

func (h *DomainHandler) DeleteDomain(c *fiber.Ctx) error {
	domainIDStr := c.Params("domainId")
	domainID, err := uuid.Parse(domainIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid domain ID"})
	}

	if err := h.db.DeleteDomain(c.Context(), domainID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete domain"})
	}

	return c.JSON(fiber.Map{"message": "Domain removed successfully"})
}
