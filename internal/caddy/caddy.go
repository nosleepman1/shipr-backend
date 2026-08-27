package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	adminURL   string
	httpClient *http.Client
}

type Upstream struct {
	Dial string `json:"dial"`
}

type ReverseProxyHandler struct {
	Handler   string     `json:"handler"`
	Upstreams []Upstream `json:"upstreams"`
}

type MatchHost struct {
	Host []string `json:"host"`
}

type Route struct {
	ID       string                `json:"@id,omitempty"`
	Match    []MatchHost           `json:"match"`
	Handle   []ReverseProxyHandler `json:"handle"`
	Terminal bool                  `json:"terminal"`
}

func NewClient(adminURL string) *Client {
	return &Client{
		adminURL: adminURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// UpsertAppRoute creates or updates a reverse-proxy route in Caddy for the given application
func (c *Client) UpsertAppRoute(ctx context.Context, appID string, hosts []string, containerIP string, port int32) error {
	routeID := fmt.Sprintf("shipr_app_%s", appID)
	upstreamDial := fmt.Sprintf("%s:%d", containerIP, port)

	route := Route{
		ID: routeID,
		Match: []MatchHost{
			{Host: hosts},
		},
		Handle: []ReverseProxyHandler{
			{
				Handler: "reverse_proxy",
				Upstreams: []Upstream{
					{Dial: upstreamDial},
				},
			},
		},
		Terminal: true,
	}

	payload, err := json.Marshal(route)
	if err != nil {
		return err
	}

	// Try updating the existing route by @id first
	idURL := fmt.Sprintf("%s/id/%s", c.adminURL, routeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, idURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
	}

	// If route didn't exist by @id, append it to the routes list
	routesURL := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", c.adminURL)
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, routesURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := c.httpClient.Do(postReq)
	if err != nil {
		return fmt.Errorf("caddy route post request failed: %w", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode >= 300 {
		body, _ := io.ReadAll(postResp.Body)
		return fmt.Errorf("caddy route creation returned %d: %s", postResp.StatusCode, string(body))
	}

	return nil
}

// DeleteAppRoute removes the reverse-proxy route for the application
func (c *Client) DeleteAppRoute(ctx context.Context, appID string) error {
	routeID := fmt.Sprintf("shipr_app_%s", appID)
	idURL := fmt.Sprintf("%s/id/%s", c.adminURL, routeID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, idURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 200 OK or 404 Not Found are both acceptable
	return nil
}
