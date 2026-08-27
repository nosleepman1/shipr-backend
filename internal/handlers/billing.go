package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"shipr-backend/internal/billing"
	"shipr-backend/internal/config"
	"shipr-backend/internal/database"
)

type BillingHandler struct {
	cfg            *config.Config
	db             *database.Queries
	paydunyaClient *billing.PaydunyaClient
}

func NewBillingHandler(cfg *config.Config, db *database.Queries, paydunyaClient *billing.PaydunyaClient) *BillingHandler {
	return &BillingHandler{
		cfg:            cfg,
		db:             db,
		paydunyaClient: paydunyaClient,
	}
}

func (h *BillingHandler) ListPlans(c *fiber.Ctx) error {
	plans, err := h.db.ListPlans(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load subscription plans"})
	}
	return c.JSON(plans)
}

func (h *BillingHandler) GetWorkspaceSubscription(c *fiber.Ctx) error {
	wsIDStr := c.Params("workspaceId")
	wsID, err := uuid.Parse(wsIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid workspace ID"})
	}

	sub, err := h.db.GetWorkspaceSubscription(c.Context(), wsID)
	if err != nil {
		// Return free plan fallback
		freePlan, _ := h.db.GetPlanBySlug(c.Context(), "free")
		return c.JSON(fiber.Map{
			"plan_name":              freePlan.Name,
			"plan_slug":              freePlan.Slug,
			"status":                 "active",
			"max_applications":       freePlan.MaxApplications,
			"max_cpus":               freePlan.MaxCpus,
			"max_memory_mb":          freePlan.MaxMemoryMb,
			"custom_domains_allowed": freePlan.CustomDomainsAllowed,
		})
	}

	return c.JSON(sub)
}

type SubscribeRequest struct {
	PlanSlug string `json:"plan_slug"` // "pro" or "enterprise"
}

func (h *BillingHandler) CreateSubscriptionCheckout(c *fiber.Ctx) error {
	wsIDStr := c.Params("workspaceId")
	wsID, err := uuid.Parse(wsIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid workspace ID"})
	}

	var req SubscribeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	plan, err := h.db.GetPlanBySlug(c.Context(), req.PlanSlug)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Plan not found"})
	}

	if plan.PriceXof <= 0 {
		// Free plan: immediately apply
		now := time.Now()
		sub, err := h.db.UpsertSubscription(c.Context(), database.UpsertSubscriptionParams{
			WorkspaceID:          wsID,
			PlanID:               plan.ID,
			Status:               database.SubscriptionStatusActive,
			PaydunyaInvoiceToken: nil,
			CurrentPeriodStart:   now,
			CurrentPeriodEnd:     now.AddDate(1, 0, 0),
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update subscription"})
		}
		return c.JSON(fiber.Map{"message": "Switched to free plan successfully", "subscription": sub})
	}

	returnURL := fmt.Sprintf("%s/dashboard?payment=success", h.cfg.FrontendURL)
	cancelURL := fmt.Sprintf("%s/dashboard?payment=canceled", h.cfg.FrontendURL)
	callbackURL := fmt.Sprintf("%s/api/v1/billing/webhook/paydunya", h.cfg.APIBaseURL)

	invoiceResp, err := h.paydunyaClient.CreateCheckoutInvoice(
		c.Context(),
		plan.Name,
		plan.PriceXof,
		wsID,
		returnURL,
		cancelURL,
		callbackURL,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create PayDunya invoice: " + err.Error()})
	}

	// Create pending payment record
	_, _ = h.db.CreatePayment(c.Context(), wsID, nil, invoiceResp.Token, plan.PriceXof, database.PaymentStatusPending)

	return c.JSON(fiber.Map{
		"checkout_url":  invoiceResp.ResponseText,
		"invoice_token": invoiceResp.Token,
		"amount_xof":    plan.PriceXof,
		"plan":          plan.Name,
	})
}

// HandlePaydunyaIPN handles instant payment notification from PayDunya
func (h *BillingHandler) HandlePaydunyaIPN(c *fiber.Ctx) error {
	var payload billing.IPNPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid IPN payload"})
	}

	token := payload.Data.Invoice.Token
	status := payload.Data.Status
	wsIDStr := payload.Data.CustomData.WorkspaceID
	receiptURL := payload.Data.Invoice.ReceiptURL
	customerPhone := payload.Data.Customer.Phone
	method := "mobile_money"

	log.Printf("[PAYDUNYA IPN] Received webhook for token %s with status: %s", token, status)

	paymentStatus := database.PaymentStatusPending
	if status == "completed" {
		paymentStatus = database.PaymentStatusCompleted
	} else if status == "failed" {
		paymentStatus = database.PaymentStatusFailed
	}

	// Update payment record in database
	_, err := h.db.UpdatePaymentStatus(
		c.Context(),
		token,
		paymentStatus,
		&receiptURL,
		&method,
		&customerPhone,
	)
	if err != nil {
		log.Printf("[PAYDUNYA IPN ERROR] Failed to update payment %s: %v", token, err)
	}

	// If payment is completed, activate the workspace subscription and upgrade quotas
	if paymentStatus == database.PaymentStatusCompleted && wsIDStr != "" {
		if wsID, err := uuid.Parse(wsIDStr); err == nil {
			// Find pro or enterprise plan based on amount
			planSlug := "pro"
			if payload.Data.Invoice.TotalAmount >= 50000 {
				planSlug = "enterprise"
			}
			plan, err := h.db.GetPlanBySlug(c.Context(), planSlug)
			if err == nil {
				now := time.Now()
				tokenPtr := &token
				_, _ = h.db.UpsertSubscription(c.Context(), database.UpsertSubscriptionParams{
					WorkspaceID:          wsID,
					PlanID:               plan.ID,
					Status:               database.SubscriptionStatusActive,
					PaydunyaInvoiceToken: tokenPtr,
					CurrentPeriodStart:   now,
					CurrentPeriodEnd:     now.AddDate(0, 1, 0), // +30 days
				})
				log.Printf("[BILLING] Workspace %s successfully upgraded to plan %s via PayDunya!", wsID, plan.Name)
			}
		}
	}

	return c.JSON(fiber.Map{"status": "IPN received successfully"})
}
