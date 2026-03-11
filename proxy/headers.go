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

	// Set Copilot authentication
	req.Header.Set("Authorization", "Bearer "+copilotToken)

	// Match OpenCode-style initiator behavior
	req.Header.Set("X-Initiator", initiator)

	// Match OpenCode-style request headers
	req.Header.Set("User-Agent", config.UserAgent())
	req.Header.Set("Editor-Version", config.EditorVersion)
	req.Header.Set("Editor-Plugin-Version", config.EditorPluginVersion)
	req.Header.Set("Copilot-Integration-Id", config.CopilotIntegrationID)
	req.Header.Set("Openai-Intent", config.OpenAIIntent)
}
