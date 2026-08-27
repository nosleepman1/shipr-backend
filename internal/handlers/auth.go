package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"shipr-backend/internal/config"
	"shipr-backend/internal/database"
	"shipr-backend/internal/middleware"
)

type AuthHandler struct {
	cfg *config.Config
	db  *database.Queries
}

func NewAuthHandler(cfg *config.Config, db *database.Queries) *AuthHandler {
	return &AuthHandler{cfg: cfg, db: db}
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || len(req.Password) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email required and password must be at least 6 characters"})
	}

	// Check if user already exists
	_, err := h.db.GetUserByEmail(c.Context(), req.Email)
	if err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "A user with this email already exists"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	pwStr := string(hashedPassword)
	nameStr := req.Name
	if nameStr == "" {
		parts := strings.Split(req.Email, "@")
		nameStr = parts[0]
	}
	defaultRole := database.UserRoleUser

	user, err := h.db.CreateUser(c.Context(), database.CreateUserParams{
		Email:        req.Email,
		PasswordHash: &pwStr,
		Name:         &nameStr,
		SystemRole:   &defaultRole,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	// Create default personal workspace for the user
	workspaceSlug := strings.ToLower(strings.ReplaceAll(nameStr, " ", "-")) + "-workspace"
	workspace, err := h.db.CreateWorkspace(c.Context(), database.CreateWorkspaceParams{
		Name:    nameStr + "'s Workspace",
		Slug:    workspaceSlug + "-" + user.ID.String()[:4],
		OwnerID: user.ID,
	})
	if err == nil {
		_, _ = h.db.AddWorkspaceMember(c.Context(), workspace.ID, user.ID, database.WorkspaceRoleOwner)
	}

	token, err := middleware.GenerateToken(user.ID.String(), user.Email, string(defaultRole), h.cfg.JWTSecret, h.cfg.JWTExpiresIn)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	// Set HttpOnly cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    token,
		Expires:  time.Now().Add(time.Duration(h.cfg.JWTExpiresIn) * time.Second),
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   h.cfg.Environment == "production",
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"token": token,
		"user":  user,
	})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	user, err := h.db.GetUserByEmail(c.Context(), req.Email)
	if err != nil || user.PasswordHash == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid email or password"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid email or password"})
	}

	role := "user"
	if user.SystemRole != nil {
		role = string(*user.SystemRole)
	}

	token, err := middleware.GenerateToken(user.ID.String(), user.Email, role, h.cfg.JWTSecret, h.cfg.JWTExpiresIn)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	// Set HttpOnly cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    token,
		Expires:  time.Now().Add(time.Duration(h.cfg.JWTExpiresIn) * time.Second),
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   h.cfg.Environment == "production",
	})

	return c.JSON(fiber.Map{
		"token": token,
		"user":  user,
	})
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: true,
	})
	return c.JSON(fiber.Map{"message": "Logged out successfully"})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	user, err := h.db.GetUserByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	workspaces, _ := h.db.ListWorkspacesForUser(context.Background(), userID)

	return c.JSON(fiber.Map{
		"user":       user,
		"workspaces": workspaces,
	})
}
