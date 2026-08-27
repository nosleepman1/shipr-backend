package billing

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type PaydunyaClient struct {
	masterKey  string
	privateKey string
	token      string
	mode       string // "test" or "live"
	httpClient *http.Client
}

func NewPaydunyaClient(masterKey, privateKey, token, mode string) *PaydunyaClient {
	return &PaydunyaClient{
		masterKey:  masterKey,
		privateKey: privateKey,
		token:      token,
		mode:       mode,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type CreateInvoiceRequest struct {
	Invoice struct {
		TotalAmount int32  `json:"total_amount"`
		Description string `json:"description"`
	} `json:"invoice"`
	Store struct {
		Name    string `json:"name"`
		Tagline string `json:"tagline"`
	} `json:"store"`
	Actions struct {
		CancelURL   string `json:"cancel_url"`
		ReturnURL   string `json:"return_url"`
		CallbackURL string `json:"callback_url"`
	} `json:"actions"`
	CustomData map[string]interface{} `json:"custom_data,omitempty"`
}

type CreateInvoiceResponse struct {
	ResponseCode string `json:"response_code"`
	ResponseText string `json:"response_text"` // URL of checkout redirect
	Token        string `json:"token"`
	Description  string `json:"description"`
}

func (c *PaydunyaClient) CreateCheckoutInvoice(
	ctx context.Context,
	planName string,
	amountXOF int32,
	workspaceID uuid.UUID,
	returnURL, cancelURL, callbackURL string,
) (*CreateInvoiceResponse, error) {
	baseURL := "https://app.paydunya.com/sandbox-api/v1/checkout-invoice/create"
	if c.mode == "live" {
		baseURL = "https://app.paydunya.com/api/v1/checkout-invoice/create"
	}

	var reqBody CreateInvoiceRequest
	reqBody.Invoice.TotalAmount = amountXOF
	reqBody.Invoice.Description = fmt.Sprintf("Shipr PaaS — Subscription: %s", planName)
	reqBody.Store.Name = "Shipr Cloud Platform"
	reqBody.Store.Tagline = "Self-Hosted Multi-Tenant PaaS"
	reqBody.Actions.ReturnURL = returnURL
	reqBody.Actions.CancelURL = cancelURL
	reqBody.Actions.CallbackURL = callbackURL
	reqBody.CustomData = map[string]interface{}{
		"workspace_id": workspaceID.String(),
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PAYDUNYA-MASTER-KEY", c.masterKey)
	req.Header.Set("PAYDUNYA-PRIVATE-KEY", c.privateKey)
	req.Header.Set("PAYDUNYA-TOKEN", c.token)

	// If keys are not yet configured in dev, generate an instant simulated checkout URL
	if c.masterKey == "" || c.token == "" {
		simulatedToken := fmt.Sprintf("sim_token_%s", uuid.New().String()[:12])
		return &CreateInvoiceResponse{
			ResponseCode: "00",
			ResponseText: fmt.Sprintf("%s?paydunya_token=%s&simulated=true", returnURL, simulatedToken),
			Token:        simulatedToken,
			Description:  "Simulated PayDunya Dev Mode",
		}, nil
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paydunya api error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res CreateInvoiceResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to parse paydunya response: %s", string(body))
	}

	if res.ResponseCode != "00" {
		return nil, fmt.Errorf("paydunya error [%s]: %s", res.ResponseCode, res.ResponseText)
	}

	return &res, nil
}

type IPNPayload struct {
	Data struct {
		Status  string `json:"status"` // "completed", "pending", "failed"
		Invoice struct {
			Token      string `json:"token"`
			ReceiptURL string `json:"receipt_url"`
			TotalAmount float64 `json:"total_amount"`
		} `json:"invoice"`
		Customer struct {
			Name  string `json:"name"`
			Phone string `json:"phone"`
			Email string `json:"email"`
		} `json:"customer"`
		CustomData struct {
			WorkspaceID string `json:"workspace_id"`
		} `json:"custom_data"`
	} `json:"data"`
}

func (c *PaydunyaClient) VerifyIPNHash(masterKey, token, receivedHash string) bool {
	if c.masterKey == "" {
		return true // Dev fallback
	}
	h := sha512.New()
	h.Write([]byte(masterKey))
	expected := hex.EncodeToString(h.Sum(nil))
	return expected == receivedHash
}
