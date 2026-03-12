package proxy

import (
	"net/http"
	"testing"
)

func TestInjectHeaders_UsesOpenCodeStyleHeaders(t *testing.T) {
	t.Setenv("OPENCODE_VERSION", "1.2.3")

	req, err := http.NewRequest("POST", "http://localhost/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer old-token")
	req.Header.Set("x-api-key", "secret")
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req.Header.Set("Anthropic-Beta", "test-flag")
	req.Header.Set("Anthropic-Dangerous-Direct-Browser-Access", "true")

	InjectHeaders(req, "new-token", "user")

	if got := req.Header.Get("Authorization"); got != "Bearer new-token" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer new-token")
	}
	if got := req.Header.Get("X-Initiator"); got != "user" {
		t.Fatalf("X-Initiator = %q, want user", got)
	}
	if got := req.Header.Get("User-Agent"); got != "GitHubCopilotChat/1.2.3" {
		t.Fatalf("User-Agent = %q, want GitHubCopilotChat/1.2.3", got)
	}
	if got := req.Header.Get("Openai-Intent"); got != "conversation-edits" {
		t.Fatalf("Openai-Intent = %q, want conversation-edits", got)
	}
	if got := req.Header.Get("X-GitHub-Api-Version"); got != "2025-10-01" {
		t.Fatalf("X-GitHub-Api-Version = %q, want 2025-10-01", got)
	}
	if got := req.Header.Get("X-Interaction-Type"); got != "conversation-panel" {
		t.Fatalf("X-Interaction-Type = %q, want conversation-panel", got)
	}
	if got := req.Header.Get("x-api-key"); got != "" {
		t.Fatalf("x-api-key = %q, want empty", got)
	}
	if got := req.Header.Get("Anthropic-Version"); got != "" {
		t.Fatalf("Anthropic-Version = %q, want empty", got)
	}
	if got := req.Header.Get("Anthropic-Beta"); got != "" {
		t.Fatalf("Anthropic-Beta = %q, want empty", got)
	}
	if got := req.Header.Get("Anthropic-Dangerous-Direct-Browser-Access"); got != "" {
		t.Fatalf("Anthropic-Dangerous-Direct-Browser-Access = %q, want empty", got)
	}
}
