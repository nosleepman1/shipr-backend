package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"shipr-backend/internal/config"
)

type OAuthService struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewOAuthService(cfg *config.Config) *OAuthService {
	return &OAuthService{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type UserProfile struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Provider  string `json:"provider"`
}

func (s *OAuthService) GetAuthURL(provider, state string) (string, error) {
	redirectURI := fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", s.cfg.APIBaseURL, provider)

	switch provider {
	case "github":
		clientID := s.cfg.GithubClientID
		if clientID == "" {
			clientID = "demo_github_client_id"
		}
		return fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email&state=%s",
			clientID, url.QueryEscape(redirectURI), state,
		), nil

	case "google":
		clientID := s.cfg.GoogleClientID
		if clientID == "" {
			clientID = "demo_google_client_id"
		}
		return fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=openid%%20email%%20profile&state=%s",
			clientID, url.QueryEscape(redirectURI), state,
		), nil

	case "gitlab":
		clientID := s.cfg.GitlabClientID
		if clientID == "" {
			clientID = "demo_gitlab_client_id"
		}
		return fmt.Sprintf(
			"https://gitlab.com/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=read_user&state=%s",
			clientID, url.QueryEscape(redirectURI), state,
		), nil

	default:
		return "", fmt.Errorf("unsupported oauth provider: %s", provider)
	}
}

func (s *OAuthService) ExchangeCode(ctx context.Context, provider, code string) (*UserProfile, error) {
	redirectURI := fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", s.cfg.APIBaseURL, provider)

	switch provider {
	case "github":
		return s.exchangeGitHub(ctx, code, redirectURI)
	case "google":
		return s.exchangeGoogle(ctx, code, redirectURI)
	case "gitlab":
		return s.exchangeGitLab(ctx, code, redirectURI)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (s *OAuthService) exchangeGitHub(ctx context.Context, code, redirectURI string) (*UserProfile, error) {
	data := url.Values{}
	data.Set("client_id", s.cfg.GithubClientID)
	data.Set("client_secret", s.cfg.GithubClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tokenResp)
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("github oauth error: %s", tokenResp.Error)
	}

	// Fetch User info
	userReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	userResp, err := s.httpClient.Do(userReq)
	if err != nil {
		return nil, err
	}
	defer userResp.Body.Close()

	var ghUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	_ = json.NewDecoder(userResp.Body).Decode(&ghUser)

	userEmail := ghUser.Email
	if userEmail == "" {
		userEmail = fmt.Sprintf("%s@users.noreply.github.com", ghUser.Login)
	}

	return &UserProfile{
		ID:        fmt.Sprintf("%d", ghUser.ID),
		Email:     userEmail,
		Name:      ghUser.Name,
		AvatarURL: ghUser.AvatarURL,
		Provider:  "github",
	}, nil
}

func (s *OAuthService) exchangeGoogle(ctx context.Context, code, redirectURI string) (*UserProfile, error) {
	data := url.Values{}
	data.Set("client_id", s.cfg.GoogleClientID)
	data.Set("client_secret", s.cfg.GoogleClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tokenResp)

	// Fetch user info
	uReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	uReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	uResp, err := s.httpClient.Do(uReq)
	if err != nil {
		return nil, err
	}
	defer uResp.Body.Close()

	var gUser struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	_ = json.NewDecoder(uResp.Body).Decode(&gUser)

	return &UserProfile{
		ID:        gUser.ID,
		Email:     gUser.Email,
		Name:      gUser.Name,
		AvatarURL: gUser.Picture,
		Provider:  "google",
	}, nil
}

func (s *OAuthService) exchangeGitLab(ctx context.Context, code, redirectURI string) (*UserProfile, error) {
	data := url.Values{}
	data.Set("client_id", s.cfg.GitlabClientID)
	data.Set("client_secret", s.cfg.GitlabClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://gitlab.com/oauth/token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tokenResp)

	uReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://gitlab.com/api/v4/user", nil)
	uReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	uResp, err := s.httpClient.Do(uReq)
	if err != nil {
		return nil, err
	}
	defer uResp.Body.Close()

	var glUser struct {
		ID        int64  `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	_ = json.NewDecoder(uResp.Body).Decode(&glUser)

	return &UserProfile{
		ID:        fmt.Sprintf("%d", glUser.ID),
		Email:     glUser.Email,
		Name:      glUser.Name,
		AvatarURL: glUser.AvatarURL,
		Provider:  "gitlab",
	}, nil
}
