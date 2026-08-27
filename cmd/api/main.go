package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"shipr-backend/internal/caddy"
	"shipr-backend/internal/config"
	"shipr-backend/internal/crypto"
	"shipr-backend/internal/database"
	"shipr-backend/internal/docker"
	"shipr-backend/internal/handlers"
	"shipr-backend/internal/middleware"
)

func main() {
	cfg := config.Load()
	log.Printf("[INFO] Starting Shipr Control Plane API on port %s (%s environment)...", cfg.Port, cfg.Environment)

	// 1. Connect to PostgreSQL
	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] Could not connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	if err := database.AutoMigrate(ctx, pool); err != nil {
		log.Printf("[WARN] Auto-migration warning: %v", err)
	}

	db := database.New(pool)
	log.Printf("[INFO] Connected to PostgreSQL successfully.")

	// 2. Connect to Redis
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		redisOpt = &redis.Options{Addr: "localhost:6379"}
	}
	rdb := redis.NewClient(redisOpt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[WARN] Redis ping failed (will retry): %v", err)
	} else {
		log.Printf("[INFO] Connected to Redis successfully.")
	}
	defer rdb.Close()

	// 3. Initialize Asynq Client
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: redisOpt.Addr, Password: redisOpt.Password, DB: redisOpt.DB})
	defer asynqClient.Close()

	// 4. Initialize Crypto, Docker and Caddy Clients
	encryptor, err := crypto.NewEncryptor(cfg.MasterKey)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize AES encryptor: %v", err)
	}

	dockerCli, err := docker.NewClient(cfg.DockerNetwork)
	if err != nil {
		log.Printf("[WARN] Docker client init warning: %v", err)
	}

	caddyCli := caddy.NewClient(cfg.CaddyAdminURL)

	// 5. Initialize Handlers
	authHandler := handlers.NewAuthHandler(cfg, db)
	workspaceHandler := handlers.NewWorkspaceHandler(db)
	projectHandler := handlers.NewProjectHandler(db)
	appHandler := handlers.NewApplicationHandler(cfg, db, dockerCli, caddyCli, encryptor)
	deploymentHandler := handlers.NewDeploymentHandler(db, asynqClient, rdb)
	webhookHandler := handlers.NewWebhookHandler(db, asynqClient)
	domainHandler := handlers.NewDomainHandler(cfg, db)

	// 6. Initialize Fiber App
	app := fiber.New(fiber.Config{
		AppName:      "Shipr Control Plane v1.0",
		ReadTimeout:  10 * time.Minute, // for long SSE connections
		WriteTimeout: 10 * time.Minute,
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000, " + cfg.FrontendURL,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, PATCH, OPTIONS",
		AllowCredentials: true,
	}))

	// Base Healthcheck
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy", "service": "shipr-api", "timestamp": time.Now()})
	})

	// API v1 Routes
	v1 := app.Group("/api/v1")

	// Public Routes
	v1.Post("/auth/register", authHandler.Register)
	v1.Post("/auth/login", authHandler.Login)
	v1.Post("/auth/logout", authHandler.Logout)

	// Caddy TLS verification endpoint
	v1.Get("/domains/check", domainHandler.CheckDomain)

	// Webhooks
	v1.Post("/webhooks/github/:appId", webhookHandler.HandleGithubWebhook)

	// Protected Routes
	protected := v1.Group("/", middleware.AuthRequired(cfg.JWTSecret))

	// Auth profile
	protected.Get("auth/me", authHandler.Me)

	// Workspaces & Projects
	protected.Get("workspaces", workspaceHandler.ListWorkspaces)
	protected.Post("workspaces", workspaceHandler.CreateWorkspace)
	protected.Get("workspaces/:workspaceId", workspaceHandler.GetWorkspace)
	protected.Post("workspaces/:workspaceId/projects", projectHandler.CreateProject)
	protected.Get("projects/:projectId", projectHandler.GetProject)

	// Applications
	protected.Post("projects/:projectId/applications", appHandler.CreateApplication)
	protected.Get("applications/:appId", appHandler.GetApplication)
	protected.Delete("applications/:appId", appHandler.DeleteApplication)
	protected.Get("applications/:appId/metrics", appHandler.GetApplicationMetrics)

	// Environment Variables
	protected.Get("applications/:appId/env", appHandler.ListEnvironmentVariables)
	protected.Put("applications/:appId/env", appHandler.UpsertEnvironmentVariables)

	// Domains
	protected.Post("applications/:appId/domains", domainHandler.AddDomain)
	protected.Delete("applications/:appId/domains/:domainId", domainHandler.DeleteDomain)

	// Deployments & SSE Logs
	protected.Post("applications/:appId/deploy", deploymentHandler.TriggerDeploy)
	protected.Get("applications/:appId/deployments", deploymentHandler.ListDeployments)
	protected.Get("deployments/:deploymentId", deploymentHandler.GetDeployment)
	protected.Get("deployments/:deploymentId/logs/stream", deploymentHandler.StreamLogsSSE)

	// Graceful Shutdown
	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Printf("[INFO] Server closed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Printf("[INFO] Shutting down Control Plane API gracefully...")
	_ = app.Shutdown()
	log.Printf("[INFO] Shipr API stopped.")
}
