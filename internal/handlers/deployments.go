package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"shipr-backend/internal/database"
	"shipr-backend/internal/middleware"
	"shipr-backend/internal/tasks"
)

type DeploymentHandler struct {
	db          *database.Queries
	asynqClient *asynq.Client
	rdb         *redis.Client
}

func NewDeploymentHandler(db *database.Queries, asynqClient *asynq.Client, rdb *redis.Client) *DeploymentHandler {
	return &DeploymentHandler{
		db:          db,
		asynqClient: asynqClient,
		rdb:         rdb,
	}
}

func (h *DeploymentHandler) TriggerDeploy(c *fiber.Ctx) error {
	appIDStr := c.Params("appId")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid application ID"})
	}

	app, err := h.db.GetApplicationByID(c.Context(), appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Application not found"})
	}

	userID := middleware.GetUserID(c)
	var userPtr *uuid.UUID
	if userID != uuid.Nil {
		userPtr = &userID
	}

	// Create new deployment in database with status 'queued'
	deployment, err := h.db.CreateDeployment(c.Context(), database.CreateDeploymentParams{
		ApplicationID: app.ID,
		TriggeredBy:   userPtr,
		TriggerSource: database.TriggerSourceManual,
		Status:        database.DeploymentStatusQueued,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create deployment record: " + err.Error()})
	}

	// Update app status
	buildingStatus := "queued"
	_, _ = h.db.UpdateApplicationStatus(c.Context(), appID, &buildingStatus)

	// Enqueue Asynq Task
	task, err := tasks.NewDeployApplicationTask(deployment.ID.String(), appID.String(), string(database.TriggerSourceManual))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate build task"})
	}

	_, err = h.asynqClient.Enqueue(task)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to enqueue deployment job: " + err.Error()})
	}

	return c.Status(fiber.StatusAccepted).JSON(deployment)
}

func (h *DeploymentHandler) ListDeployments(c *fiber.Ctx) error {
	appIDStr := c.Params("appId")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid application ID"})
	}

	limit := int32(20)
	offset := int32(0)
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = int32(val)
		}
	}
	if o := c.Query("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			offset = int32(val)
		}
	}

	deployments, err := h.db.ListDeploymentsByAppID(c.Context(), appID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list deployments"})
	}

	return c.JSON(deployments)
}

func (h *DeploymentHandler) GetDeployment(c *fiber.Ctx) error {
	depIDStr := c.Params("deploymentId")
	depID, err := uuid.Parse(depIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid deployment ID"})
	}

	deployment, err := h.db.GetDeploymentByID(c.Context(), depID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Deployment not found"})
	}

	return c.JSON(deployment)
}

// StreamLogsSSE streams live deployment logs in real-time using Server-Sent Events (SSE)
func (h *DeploymentHandler) StreamLogsSSE(c *fiber.Ctx) error {
	depIDStr := c.Params("deploymentId")
	depID, err := uuid.Parse(depIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid deployment ID"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("Access-Control-Allow-Origin", "*")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 1. Send all existing logs from DB first
		existingLogs, err := h.db.ListDeploymentLogsByDeploymentID(ctx, depID)
		if err == nil {
			for _, l := range existingLogs {
				stream := "stdout"
				if l.Stream != nil {
					stream = *l.Stream
				}
				data, _ := json.Marshal(map[string]interface{}{
					"stream":    stream,
					"message":   l.Message,
					"timestamp": l.CreatedAt.Format(time.RFC3339),
				})
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			}
			_ = w.Flush()
		}

		// Check if deployment is already finished
		dep, err := h.db.GetDeploymentByID(ctx, depID)
		if err == nil && (dep.Status == database.DeploymentStatusRunning || dep.Status == database.DeploymentStatusFailed || dep.Status == database.DeploymentStatusCancelled) {
			_, _ = fmt.Fprintf(w, "event: end\ndata: %s\n\n", dep.Status)
			_ = w.Flush()
			return
		}

		// 2. Subscribe to Redis Pub/Sub for live real-time stream
		channel := fmt.Sprintf("logs:%s", depID.String())
		pubsub := h.rdb.Subscribe(ctx, channel)
		defer pubsub.Close()

		ch := pubsub.Channel()
		ticker := time.NewTicker(25 * time.Second) // Keepalive heartbeat
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", msg.Payload)
				if err := w.Flush(); err != nil {
					return
				}

			case <-ticker.C:
				// SSE Heartbeat comment to keep connection alive
				_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
				if err := w.Flush(); err != nil {
					return
				}

			case <-time.After(2 * time.Second):
				// Periodically check if deployment is done
				curDep, err := h.db.GetDeploymentByID(ctx, depID)
				if err == nil && (curDep.Status == database.DeploymentStatusRunning || curDep.Status == database.DeploymentStatusFailed || curDep.Status == database.DeploymentStatusCancelled) {
					_, _ = fmt.Fprintf(w, "event: end\ndata: %s\n\n", curDep.Status)
					_ = w.Flush()
					return
				}
			}
		}
	})

	return nil
}
