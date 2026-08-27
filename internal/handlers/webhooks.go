package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"shipr-backend/internal/database"
	"shipr-backend/internal/tasks"
)

type WebhookHandler struct {
	db          *database.Queries
	asynqClient *asynq.Client
}

func NewWebhookHandler(db *database.Queries, asynqClient *asynq.Client) *WebhookHandler {
	return &WebhookHandler{
		db:          db,
		asynqClient: asynqClient,
	}
}

func (h *WebhookHandler) HandleGithubWebhook(c *fiber.Ctx) error {
	appIDStr := c.Params("appId")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid app ID"})
	}

	app, err := h.db.GetApplicationByID(c.Context(), appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Application not found"})
	}

	event := c.Get("X-GitHub-Event")
	if event != "push" && event != "ping" {
		return c.JSON(fiber.Map{"message": "Event ignored (only push triggers deploy)"})
	}

	if event == "ping" {
		return c.JSON(fiber.Map{"message": "Webhook ping received successfully"})
	}

	log.Printf("[WEBHOOK] GitHub push received for app: %s (%s)", app.Name, app.ID)

	// Create new deployment record
	deployment, err := h.db.CreateDeployment(c.Context(), database.CreateDeploymentParams{
		ApplicationID: app.ID,
		TriggerSource: database.TriggerSourceWebhook,
		Status:        database.DeploymentStatusQueued,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create deployment"})
	}

	// Enqueue Asynq task
	task, err := tasks.NewDeployApplicationTask(deployment.ID.String(), app.ID.String(), string(database.TriggerSourceWebhook))
	if err == nil {
		_, _ = h.asynqClient.Enqueue(task)
	}

	return c.JSON(fiber.Map{
		"message":       "Deployment triggered via GitHub webhook",
		"deployment_id": deployment.ID,
	})
}
