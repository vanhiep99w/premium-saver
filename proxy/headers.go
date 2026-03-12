package proxy

import (
	"net/http"

	"github.com/hieptran/copilot-proxy/config"
)

// InjectHeaders sets all required headers for the Copilot API request.
// This replaces any existing Authorization header with the Copilot token,
// and sets X-Initiator based on the current runtime policy.
func InjectHeaders(req *http.Request, copilotToken, initiator string) {
	// Remove headers that could conflict
	req.Header.Del("x-api-key")
	req.Header.Del("authorization")
	req.Header.Del("anthropic-version")
	req.Header.Del("anthropic-beta")
	req.Header.Del("anthropic-dangerous-direct-browser-access")

	// Set Copilot authentication
	req.Header.Set("Authorization", "Bearer "+copilotToken)

	// Match OpenCode-style initiator behavior
	req.Header.Set("X-Initiator", initiator)

	// Match OpenCode-style request headers
	req.Header.Set("User-Agent", config.UserAgent())
	req.Header.Set("Editor-Version", config.EditorVersion)
	req.Header.Set("Editor-Plugin-Version", config.EditorPluginVersion)
	req.Header.Set("Copilot-Integration-Id", config.CopilotIntegrationID)
	req.Header.Set("X-GitHub-Api-Version", "2025-10-01")
	req.Header.Set("X-Interaction-Type", "conversation-panel")
	req.Header.Set("Openai-Intent", config.OpenAIIntent)
}
