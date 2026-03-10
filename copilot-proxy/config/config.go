package config

import (
	"os"
	"path/filepath"
)

const (
	// GitHub OAuth client ID (VS Code Copilot Chat)
	OAuthClientID = "Iv1.b507a08c87ecfe98"

	// GitHub endpoints
	GitHubDeviceCodeURL    = "https://github.com/login/device/code"
	GitHubOAuthTokenURL    = "https://github.com/login/oauth/access_token"
	GitHubDeviceVerifyURL  = "https://github.com/login/device"
	CopilotTokenURL        = "https://api.github.com/copilot_internal/v2/token"

	// Copilot API
	CopilotAPIBaseURL = "https://api.githubcopilot.com"

	// Headers to impersonate VS Code Copilot Chat
	UserAgent           = "GitHubCopilotChat/0.35.0"
	EditorVersion       = "vscode/1.107.0"
	EditorPluginVersion = "copilot-chat/0.35.0"
	CopilotIntegrationID = "vscode-chat"
	OpenAIIntent        = "conversation-edits"

	// OAuth scope
	OAuthScope = "read:user"

	// Token refresh buffer (5 minutes before expiry)
	TokenRefreshBufferMs = 5 * 60 * 1000

	// Default proxy port
	DefaultPort = 8787
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
