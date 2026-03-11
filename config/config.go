package config

import (
	"os"
	"path/filepath"
	"strconv"
)

const (
	// GitHub OAuth client ID (VS Code Copilot Chat)
	OAuthClientID = "Iv1.b507a08c87ecfe98"

	// GitHub endpoints
	GitHubDeviceCodeURL   = "https://github.com/login/device/code"
	GitHubOAuthTokenURL   = "https://github.com/login/oauth/access_token"
	GitHubDeviceVerifyURL = "https://github.com/login/device"
	CopilotTokenURL       = "https://api.github.com/copilot_internal/v2/token"

	// Copilot API
	CopilotAPIBaseURL = "https://api.githubcopilot.com"

	// Headers to impersonate OpenCode/Copilot behavior
	UserAgentPrefix         = "opencode"
	DefaultUserAgentVersion = "dev"
	EditorVersion           = "vscode/1.107.0"
	EditorPluginVersion     = "copilot-chat/0.35.0"
	CopilotIntegrationID    = "vscode-chat"
	OpenAIIntent            = "conversation-edits"

	// OAuth scope
	OAuthScope = "read:user"

	// Token refresh buffer (5 minutes before expiry)
	TokenRefreshBufferMs = 5 * 60 * 1000

	// Default proxy port
	DefaultPort               = 8787
	DefaultInitiatorUserEvery = 7
)

// AuthFilePath returns the path to the auth storage file.
func AuthFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}
	dir := filepath.Join(configDir, "copilot-proxy")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "auth.json"), nil
}

// DBPath returns the SQLite database file path.
// Uses DB_PATH env var, or defaults to ~/.config/copilot-proxy/copilot-proxy.db
func DBPath() (string, error) {
	if p := os.Getenv("DB_PATH"); p != "" {
		dir := filepath.Dir(p)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", err
		}
		return p, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}
	dir := filepath.Join(configDir, "copilot-proxy")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "copilot-proxy.db"), nil
}

// AdminUsername returns the admin username from ADMIN_USERNAME env var (default: "admin").
func AdminUsername() string {
	if u := os.Getenv("ADMIN_USERNAME"); u != "" {
		return u
	}
	return "admin"
}

// AdminPassword returns the admin password from ADMIN_PASSWORD env var.
// Returns empty string if not set.
func AdminPassword() string {
	return os.Getenv("ADMIN_PASSWORD")
}

// InitiatorUserEvery returns how many agent requests to send before one user request.
// Example: 7 means 7 agent requests, then 1 user request.
func InitiatorUserEvery() int {
	raw := os.Getenv("X_INITIATOR_USER_EVERY")
	if raw == "" {
		return DefaultInitiatorUserEvery
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return DefaultInitiatorUserEvery
	}

	return value
}

// UserAgent returns the OpenCode-style user agent string.
func UserAgent() string {
	version := os.Getenv("OPENCODE_VERSION")
	if version == "" {
		version = DefaultUserAgentVersion
	}
	return UserAgentPrefix + "/" + version
}
