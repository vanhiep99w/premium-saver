package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hieptran/copilot-proxy/config"
)

// DeviceCodeResponse is the response from GitHub's device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// OAuthTokenResponse is the response from GitHub's OAuth token endpoint.
type OAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// CopilotTokenResponse is the response from the Copilot internal token endpoint.
type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// Authenticator handles the GitHub OAuth device flow and Copilot token management.
type Authenticator struct {
	store  *Store
	client *http.Client
}

// NewAuthenticator creates a new Authenticator with the given store.
func NewAuthenticator(store *Store) *Authenticator {
	return &Authenticator{
		store:  store,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// RequestDeviceCode initiates the OAuth device flow.
func (a *Authenticator) RequestDeviceCode() (*DeviceCodeResponse, error) {
	body := map[string]string{
		"client_id": config.OAuthClientID,
		"scope":     config.OAuthScope,
	}
	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", config.GitHubDeviceCodeURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", config.UserAgent())

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode device code response: %w", err)
	}
	return &result, nil
}

// PollForToken polls GitHub for the OAuth access token after user authorization.
func (a *Authenticator) PollForToken(deviceCode string, interval int) (string, error) {
	body := map[string]string{
		"client_id":   config.OAuthClientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}
	jsonBody, _ := json.Marshal(body)

	pollInterval := time.Duration(interval) * time.Second
	if pollInterval < 5*time.Second {
		pollInterval = 5 * time.Second
	}

	for {
		time.Sleep(pollInterval)

		req, err := http.NewRequest("POST", config.GitHubOAuthTokenURL, bytes.NewReader(jsonBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", config.UserAgent())

		resp, err := a.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to poll for token: %w", err)
		}

		var result OAuthTokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("failed to decode token response: %w", err)
		}
		resp.Body.Close()

		switch result.Error {
		case "":
			// Success
			if result.AccessToken == "" {
				return "", fmt.Errorf("received empty access token")
			}
			return result.AccessToken, nil
		case "authorization_pending":
			// User hasn't authorized yet, keep polling
			continue
		case "slow_down":
			// Increase interval
			pollInterval += 5 * time.Second
			continue
		case "expired_token":
			return "", fmt.Errorf("device code expired, please try again")
		case "access_denied":
			return "", fmt.Errorf("authorization was denied by the user")
		default:
			return "", fmt.Errorf("OAuth error: %s - %s", result.Error, result.ErrorDesc)
		}
	}
}

// Login performs the full OAuth device flow login.
func (a *Authenticator) Login() error {
	// Step 1: Request device code
	deviceCode, err := a.RequestDeviceCode()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("To authenticate with GitHub Copilot:")
	fmt.Printf("  1. Open: %s\n", deviceCode.VerificationURI)
	fmt.Printf("  2. Enter code: %s\n", deviceCode.UserCode)
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	// Step 2: Poll for access token
	accessToken, err := a.PollForToken(deviceCode.DeviceCode, deviceCode.Interval)
	if err != nil {
		return err
	}

	// Step 3: Store the OAuth token
	if err := a.store.SetOAuthToken(accessToken); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	fmt.Println("Successfully authenticated with GitHub Copilot!")

	// Step 4: Pre-fetch a Copilot API token
	_, err = a.GetCopilotToken()
	if err != nil {
		fmt.Printf("Warning: could not pre-fetch Copilot token: %v\n", err)
	}

	return nil
}

// RefreshCopilotToken fetches a new short-lived Copilot API token using the OAuth token.
func (a *Authenticator) RefreshCopilotToken() (*CopilotTokenResponse, error) {
	tokenData := a.store.Get()
	if tokenData.OAuthToken == "" {
		return nil, fmt.Errorf("not authenticated - run 'copilot-proxy login' first")
	}

	req, err := http.NewRequest("GET", config.CopilotTokenURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenData.OAuthToken)
	req.Header.Set("User-Agent", config.UserAgent())
	req.Header.Set("Editor-Version", config.EditorVersion)
	req.Header.Set("Editor-Plugin-Version", config.EditorPluginVersion)
	req.Header.Set("Copilot-Integration-Id", config.CopilotIntegrationID)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Copilot token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("OAuth token expired or revoked - run 'copilot-proxy login' to re-authenticate")
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Copilot token refresh failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result CopilotTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode Copilot token response: %w", err)
	}

	// Store with 5-minute early buffer
	expiresAt := result.ExpiresAt*1000 - config.TokenRefreshBufferMs
	if err := a.store.SetCopilotToken(result.Token, expiresAt); err != nil {
		return nil, fmt.Errorf("failed to store Copilot token: %w", err)
	}

	return &result, nil
}

// GetCopilotToken returns a valid Copilot API token, refreshing if necessary.
func (a *Authenticator) GetCopilotToken() (string, error) {
	tokenData := a.store.Get()

	// Check if current token is still valid
	if tokenData.CopilotToken != "" && tokenData.ExpiresAt > time.Now().UnixMilli() {
		return tokenData.CopilotToken, nil
	}

	// Token expired or missing, refresh
	result, err := a.RefreshCopilotToken()
	if err != nil {
		return "", err
	}

	return result.Token, nil
}

// Logout clears all stored tokens.
func (a *Authenticator) Logout() error {
	return a.store.Clear()
}

// Status returns the current authentication status.
func (a *Authenticator) Status() (authenticated bool, tokenExpiry time.Time) {
	tokenData := a.store.Get()
	if tokenData.OAuthToken == "" {
		return false, time.Time{}
	}
	if tokenData.ExpiresAt > 0 {
		return true, time.UnixMilli(tokenData.ExpiresAt)
	}
	return true, time.Time{}
}
