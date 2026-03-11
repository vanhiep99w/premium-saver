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

	InjectHeaders(req, "new-token", "user")

	if got := req.Header.Get("Authorization"); got != "Bearer new-token" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer new-token")
	}
	if got := req.Header.Get("X-Initiator"); got != "user" {
		t.Fatalf("X-Initiator = %q, want user", got)
	}
	if got := req.Header.Get("User-Agent"); got != "opencode/1.2.3" {
		t.Fatalf("User-Agent = %q, want opencode/1.2.3", got)
	}
	if got := req.Header.Get("Openai-Intent"); got != "conversation-edits" {
		t.Fatalf("Openai-Intent = %q, want conversation-edits", got)
	}
	if got := req.Header.Get("x-api-key"); got != "" {
		t.Fatalf("x-api-key = %q, want empty", got)
	}
}
