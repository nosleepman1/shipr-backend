package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"shipr-backend/internal/caddy"
	"shipr-backend/internal/config"
	"shipr-backend/internal/crypto"
	"shipr-backend/internal/database"
	"shipr-backend/internal/docker"
)

type ApplicationHandler struct {
	cfg       *config.Config
	db        *database.Queries
	dockerCli *docker.Client
	caddyCli  *caddy.Client
	encryptor *crypto.Encryptor
}

func NewApplicationHandler(
	cfg *config.Config,
	db *database.Queries,
	dockerCli *docker.Client,
	caddyCli *caddy.Client,
	encryptor *crypto.Encryptor,
) *ApplicationHandler {
	return &ApplicationHandler{
		cfg:       cfg,
		db:        db,
		dockerCli: dockerCli,
		caddyCli:  caddyCli,
		encryptor: encryptor,
	}
}

type CreateApplicationRequest struct {
	Name             string  `json:"name"`
	GitRepositoryUrl string  `json:"git_repository_url"`
	GitBranch        string  `json:"git_branch"`
	BuildPack        string  `json:"build_pack"` // nixpacks, dockerfile, static
	DockerfilePath   *string `json:"dockerfile_path"`
	BaseDirectory    *string `json:"base_directory"`
	BuildCommand     *string `json:"build_command"`
	StartCommand     *string `json:"start_command"`
	InternalPort     int32   `json:"internal_port"`
	CPULimit         float64 `json:"cpu_limit"`
	MemoryLimitMB    int32   `json:"memory_limit_mb"`
}

func (h *ApplicationHandler) CreateApplication(c *fiber.Ctx) error {
	projectIDStr := c.Params("projectId")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid project ID"})
	}

	var req CreateApplicationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.GitRepositoryUrl == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Application name and Git repository URL are required"})
	}

	branch := req.GitBranch
	if branch == "" {
		branch = "main"
	}

	buildPack := database.BuildPackTypeNixpacks
	if req.BuildPack == "dockerfile" {
		buildPack = database.BuildPackTypeDockerfile
	} else if req.BuildPack == "static" {
		buildPack = database.BuildPackTypeStatic
	}

	internalPort := req.InternalPort
	if internalPort <= 0 {
		internalPort = 3000
	}

	memLimit := req.MemoryLimitMB
	if memLimit <= 0 {
		memLimit = 512
	}

	initialStatus := "idle"

	app, err := h.db.CreateApplication(c.Context(), database.CreateApplicationParams{
		ProjectID:        projectID,
		Name:             req.Name,
		GitRepositoryUrl: req.GitRepositoryUrl,
		GitBranch:        branch,
		BuildPack:        buildPack,
		DockerfilePath:   req.DockerfilePath,
		BaseDirectory:    req.BaseDirectory,
		BuildCommand:     req.BuildCommand,
		StartCommand:     req.StartCommand,
		InternalPort:     internalPort,
		MemoryLimitMb:    &memLimit,
		Status:           &initialStatus,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create application: " + err.Error()})
	}

	// Generate default subdomain
	appSlug := strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	defaultDomain := fmt.Sprintf("%s.%s", appSlug, h.cfg.BaseDomain)
	isSub := true
	sslStatus := "ready"
	_, _ = h.db.CreateDomain(c.Context(), app.ID, defaultDomain, &isSub, &sslStatus)

	return c.Status(fiber.StatusCreated).JSON(app)
}

func (h *ApplicationHandler) GetApplication(c *fiber.Ctx) error {
	appIDStr := c.Params("appId")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid application ID"})
	}

	app, err := h.db.GetApplicationByID(c.Context(), appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Application not found"})
	}

	domains, _ := h.db.ListDomainsByAppID(c.Context(), appID)

	var activeDeployment *database.Deployment
	if app.ActiveDeploymentID != nil {
		dep, err := h.db.GetDeploymentByID(c.Context(), *app.ActiveDeploymentID)
		if err == nil {
			activeDeployment = &dep
		}
	}

	return c.JSON(fiber.Map{
		"application":       app,
		"domains":           domains,
		"active_deployment": activeDeployment,
	})
}

func (h *ApplicationHandler) DeleteApplication(c *fiber.Ctx) error {
	appIDStr := c.Params("appId")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid application ID"})
	}

	app, err := h.db.GetApplicationByID(c.Context(), appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Application not found"})
	}

	// Stop container if active
	if app.ActiveDeploymentID != nil {
		dep, err := h.db.GetDeploymentByID(c.Context(), *app.ActiveDeploymentID)
		if err == nil && dep.ContainerID != nil {
			_ = h.dockerCli.StopAndRemoveContainer(c.Context(), *dep.ContainerID)
		}
	}

	// Remove Caddy Ingress route
	_ = h.caddyCli.DeleteAppRoute(c.Context(), appID.String())

	// Delete from database (cascades to deployments, env vars, domains)
	if err := h.db.DeleteApplication(c.Context(), appID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete application"})
	}

	return c.JSON(fiber.Map{"message": "Application deleted successfully"})
}

type EnvVarItem struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

type UpdateEnvVarsRequest struct {
	Variables []EnvVarItem `json:"variables"`
}

func (h *ApplicationHandler) UpsertEnvironmentVariables(c *fiber.Ctx) error {
	appIDStr := c.Params("appId")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid application ID"})
	}

	var req UpdateEnvVarsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	for _, v := range req.Variables {
		trimmedKey := strings.TrimSpace(v.Key)
		if trimmedKey == "" {
			continue
		}

		encryptedVal, err := h.encryptor.Encrypt(v.Value)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Encryption error"})
		}

		isSecret := v.IsSecret
		_, err = h.db.UpsertEnvironmentVariable(c.Context(), database.UpsertEnvironmentVariableParams{
			ApplicationID:  appID,
			Key:            trimmedKey,
			ValueEncrypted: encryptedVal,
			IsSecret:       &isSecret,
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save env variable: " + err.Error()})
		}
	}

	return c.JSON(fiber.Map{"message": "Environment variables saved successfully"})
}

func (h *ApplicationHandler) ListEnvironmentVariables(c *fiber.Ctx) error {
	appIDStr := c.Params("appId")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid application ID"})
	}

	reveal := c.Query("reveal") == "true"
	vars, err := h.db.ListEnvironmentVariablesByAppID(c.Context(), appID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list env variables"})
	}

	type EnvResponse struct {
		ID        uuid.UUID `json:"id"`
		Key       string    `json:"key"`
		Value     string    `json:"value"`
		IsSecret  bool      `json:"is_secret"`
		CreatedAt string    `json:"created_at"`
	}

	result := make([]EnvResponse, 0, len(vars))
	for _, v := range vars {
		val := "••••••••"
		isSec := v.IsSecret != nil && *v.IsSecret
		if reveal || !isSec {
			decrypted, err := h.encryptor.Decrypt(v.ValueEncrypted)
			if err == nil {
				val = decrypted
			}
		}

		result = append(result, EnvResponse{
			ID:        v.ID,
			Key:       v.Key,
			Value:     val,
			IsSecret:  isSec,
			CreatedAt: v.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return c.JSON(result)
}

func (h *ApplicationHandler) GetApplicationMetrics(c *fiber.Ctx) error {
	appIDStr := c.Params("appId")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid application ID"})
	}

	app, err := h.db.GetApplicationByID(c.Context(), appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Application not found"})
	}

	if app.ActiveDeploymentID == nil {
		return c.JSON(docker.ContainerStats{
			CPUUsagePercent: 0,
			MemoryUsageMB:   0,
			MemoryLimitMB:   512,
			MemoryPercent:   0,
		})
	}

	dep, err := h.db.GetDeploymentByID(c.Context(), *app.ActiveDeploymentID)
	if err != nil || dep.ContainerID == nil || *dep.ContainerID == "" {
		return c.JSON(docker.ContainerStats{
			CPUUsagePercent: 0,
			MemoryUsageMB:   0,
			MemoryLimitMB:   512,
			MemoryPercent:   0,
		})
	}

	stats, err := h.dockerCli.GetContainerStats(context.Background(), *dep.ContainerID)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Could not fetch container stats: " + err.Error()})
	}

	return c.JSON(stats)
}
