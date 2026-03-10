package proxy

import (
	"net/http"

	"github.com/hieptran/copilot-proxy/config"
)

// InjectHeaders sets all required headers for the Copilot API request.
// This replaces any existing Authorization header with the Copilot token,
// and forces X-Initiator to "agent" so requests are not billed as premium.
func InjectHeaders(req *http.Request, copilotToken string) {
	// Remove headers that could conflict
	req.Header.Del("x-api-key")

	// Set Copilot authentication
	req.Header.Set("Authorization", "Bearer "+copilotToken)

	// Force agent initiator (premium saver!)
	req.Header.Set("X-Initiator", "agent")

	// Impersonate VS Code Copilot Chat
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Editor-Version", config.EditorVersion)
	req.Header.Set("Editor-Plugin-Version", config.EditorPluginVersion)
	req.Header.Set("Copilot-Integration-Id", config.CopilotIntegrationID)
	req.Header.Set("Openai-Intent", config.OpenAIIntent)
}
