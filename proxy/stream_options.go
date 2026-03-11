package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// injectStreamOptions modifies the request body to add stream_options.include_usage=true
// so the API returns token usage data in the final SSE chunk of streaming responses.
func injectStreamOptions(req *http.Request) {
	if req.Body == nil {
		return
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil || len(body) == 0 {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return
	}

	// Only inject if stream is true
	stream, ok := payload["stream"].(bool)
	if !ok || !stream {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return
	}

	// Don't overwrite if already set
	if _, exists := payload["stream_options"]; exists {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return
	}

	payload["stream_options"] = map[string]any{"include_usage": true}

	modified, err := json.Marshal(payload)
	if err != nil {
		log.Printf("WARNING: failed to marshal stream_options injection: %v", err)
		req.Body = io.NopCloser(bytes.NewReader(body))
		return
	}

	req.Body = io.NopCloser(bytes.NewReader(modified))
	req.ContentLength = int64(len(modified))
}
