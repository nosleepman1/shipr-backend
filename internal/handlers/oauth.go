package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"shipr-backend/internal/config"
	"shipr-backend/internal/database"
	"shipr-backend/internal/middleware"
	"shipr-backend/internal/oauth"
)

type OAuthHandler struct {
	cfg          *config.Config
	db           *database.Queries
	oauthService *oauth.OAuthService
}

func NewOAuthHandler(cfg *config.Config, db *database.Queries, oauthService *oauth.OAuthService) *OAuthHandler {
	return &OAuthHandler{
		cfg:          cfg,
		db:           db,
		oauthService: oauthService,
	}
}

func (h *OAuthHandler) InitiateOAuth(c *fiber.Ctx) error {
	provider := strings.ToLower(c.Params("provider"))

	// Generate random CSRF state
	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	// Set state cookie for verification
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HTTPOnly: true,
		SameSite: "Lax",
	})

	authURL, err := h.oauthService.GetAuthURL(provider, state)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Redirect(authURL, fiber.StatusTemporaryRedirect)
}

func (h *OAuthHandler) OAuthCallback(c *fiber.Ctx) error {
	provider := strings.ToLower(c.Params("provider"))
	code := c.Query("code")
	if code == "" {
		return c.Redirect(fmt.Sprintf("%s/auth?error=missing_code", h.cfg.FrontendURL), fiber.StatusTemporaryRedirect)
	}

	profile, err := h.oauthService.ExchangeCode(c.Context(), provider, code)
	if err != nil {
		log.Printf("[OAUTH ERROR] Provider %s exchange failed: %v", provider, err)
		return c.Redirect(fmt.Sprintf("%s/auth?error=oauth_exchange_failed", h.cfg.FrontendURL), fiber.StatusTemporaryRedirect)
	}

	// 1. Check if user already exists with this email
	user, err := h.db.GetUserByEmail(c.Context(), profile.Email)
	if err != nil {
		// User does not exist, create user
		nameStr := profile.Name
		if nameStr == "" {
			nameStr = strings.Split(profile.Email, "@")[0]
		}
		avatarURL := profile.AvatarURL
		defaultRole := database.UserRoleUser

		newUser, err := h.db.CreateUser(c.Context(), database.CreateUserParams{
			Email:      profile.Email,
			Name:       &nameStr,
			AvatarUrl:  &avatarURL,
			SystemRole: &defaultRole,
		})
		if err != nil {
			log.Printf("[OAUTH ERROR] Failed to create user: %v", err)
			return c.Redirect(fmt.Sprintf("%s/auth?error=user_creation_failed", h.cfg.FrontendURL), fiber.StatusTemporaryRedirect)
		}
		user = newUser

		// Create default personal workspace
		workspaceSlug := strings.ToLower(strings.ReplaceAll(nameStr, " ", "-")) + "-workspace"
		workspace, err := h.db.CreateWorkspace(c.Context(), database.CreateWorkspaceParams{
			Name:    nameStr + "'s Workspace",
			Slug:    workspaceSlug + "-" + user.ID.String()[:4],
			OwnerID: user.ID,
		})
		if err == nil {
			_, _ = h.db.AddWorkspaceMember(c.Context(), workspace.ID, user.ID, database.WorkspaceRoleOwner)
		}
	}

	// 2. Link Account in accounts table
	authProvider := database.AuthProvider(provider)
	_, _ = h.db.UpsertAccount(c.Context(), database.UpsertAccountParams{
		UserID:            user.ID,
		Provider:          authProvider,
		ProviderAccountID: profile.ID,
	})

	// 3. Generate JWT Token
	role := "user"
	if user.SystemRole != nil {
		role = string(*user.SystemRole)
	}

	token, err := middleware.GenerateToken(user.ID.String(), user.Email, role, h.cfg.JWTSecret, h.cfg.JWTExpiresIn)
	if err != nil {
		return c.Redirect(fmt.Sprintf("%s/auth?error=token_generation_failed", h.cfg.FrontendURL), fiber.StatusTemporaryRedirect)
	}

	// 4. Set HttpOnly Cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    token,
		Expires:  time.Now().Add(time.Duration(h.cfg.JWTExpiresIn) * time.Second),
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   h.cfg.Environment == "production",
	})

	// 5. Redirect back to frontend dashboard with token param for localStorage sync
	return c.Redirect(fmt.Sprintf("%s/dashboard?token=%s", h.cfg.FrontendURL, token), fiber.StatusTemporaryRedirect)
}
