package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestInjectStreamOptions_SkipsAnthropicMessagesRequests(t *testing.T) {
	req, err := http.NewRequest(
		"POST",
		"http://localhost/messages",
		strings.NewReader(`{"model":"claude-sonnet-4","stream":true}`),
	)
	if err != nil {
		t.Fatal(err)
	}

	injectStreamOptions(req)

	var payload map[string]any
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatalf("decode modified body: %v", err)
	}

	if _, exists := payload["stream_options"]; exists {
		t.Fatalf("stream_options should not be injected for /messages requests")
	}
}

func TestInjectStreamOptions_AddsUsageForChatCompletions(t *testing.T) {
	req, err := http.NewRequest(
		"POST",
		"http://localhost/chat/completions",
		strings.NewReader(`{"model":"claude-sonnet-4","stream":true}`),
	)
	if err != nil {
		t.Fatal(err)
	}

	injectStreamOptions(req)

	var payload map[string]any
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatalf("decode modified body: %v", err)
	}

	streamOptions, ok := payload["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options = %#v, want object", payload["stream_options"])
	}
	if got := streamOptions["include_usage"]; got != true {
		t.Fatalf("stream_options.include_usage = %#v, want true", got)
	}
}
