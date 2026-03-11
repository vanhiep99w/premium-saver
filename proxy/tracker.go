package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/hieptran/copilot-proxy/db"
)

// UsageTracker collects usage data from proxy responses and writes to the database.
type UsageTracker struct {
	ch       chan db.UsageRecord
	database *db.DB
	wg       sync.WaitGroup
}

// NewUsageTracker creates a new tracker with a buffered channel and starts the writer goroutine.
func NewUsageTracker(database *db.DB) *UsageTracker {
	t := &UsageTracker{
		ch:       make(chan db.UsageRecord, 1000),
		database: database,
	}
	t.wg.Add(1)
	go t.writer()
	return t
}

// Track sends a usage record to the writer. Non-blocking; drops if channel is full.
func (t *UsageTracker) Track(record db.UsageRecord) {
	select {
	case t.ch <- record:
	default:
		log.Printf("WARNING: usage tracking channel full, dropping record for user %d", record.UserID)
	}
}

func (t *UsageTracker) writer() {
	defer t.wg.Done()
	var backoff time.Duration

	for record := range t.ch {
		err := t.database.InsertUsage(record)
		if err != nil {
			log.Printf("ERROR: failed to write usage log: %v", err)
			if backoff == 0 {
				backoff = 1 * time.Second
			} else {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
			time.Sleep(backoff)
		} else {
			backoff = 0
		}
	}
}

// usageData represents the usage field in an OpenAI-style API response.
type usageData struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// streamResponse represents a chunk of an SSE stream data line.
type streamResponse struct {
	Usage *usageData `json:"usage,omitempty"`
	Model string     `json:"model,omitempty"`
}

// nonStreamResponse represents a non-streaming API response.
type nonStreamResponse struct {
	Usage *usageData `json:"usage,omitempty"`
	Model string     `json:"model,omitempty"`
}

// TrackingReader wraps a response body to intercept and extract usage data from the stream.
type TrackingReader struct {
	inner    io.ReadCloser
	lineBuf  string // buffer for incomplete lines across Read boundaries
	userID   int
	path     string
	tracker  *UsageTracker
	usage    *usageData
	model    string
}

// NewTrackingReader creates a reader that tees data through while scanning for usage info.
func NewTrackingReader(body io.ReadCloser, userID int, path string, tracker *UsageTracker) *TrackingReader {
	return &TrackingReader{
		inner:   body,
		userID:  userID,
		path:    path,
		tracker: tracker,
	}
}

// Read implements io.Reader. Passes data through and scans for usage info.
func (r *TrackingReader) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		r.scanForUsage(p[:n])
	}
	if err == io.EOF {
		r.emitUsage()
	}
	return n, err
}

// Close implements io.Closer.
func (r *TrackingReader) Close() error {
	r.emitUsage()
	return r.inner.Close()
}

func (r *TrackingReader) scanForUsage(data []byte) {
	// Prepend any leftover data from previous Read call
	text := r.lineBuf + string(data)
	lines := strings.Split(text, "\n")

	// Last element may be incomplete — save for next Read
	r.lineBuf = lines[len(lines)-1]
	lines = lines[:len(lines)-1]

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}

		var resp streamResponse
		if err := json.Unmarshal([]byte(payload), &resp); err != nil {
			continue
		}
		if resp.Model != "" {
			r.model = resp.Model
		}
		if resp.Usage != nil {
			r.usage = resp.Usage
		}
	}
}

func (r *TrackingReader) emitUsage() {
	if r.tracker == nil {
		return
	}
	tracker := r.tracker
	r.tracker = nil // emit only once

	record := db.UsageRecord{
		UserID: r.userID,
		Path:   r.path,
		Model:  r.model,
	}
	if r.usage != nil {
		record.PromptTokens = r.usage.PromptTokens
		record.CompletionTokens = r.usage.CompletionTokens
		record.TotalTokens = r.usage.TotalTokens
	}
	tracker.Track(record)
}

// ParseNonStreamUsage parses usage from a non-streaming JSON response body.
// Returns a new ReadCloser with the same content (re-serves the body).
func ParseNonStreamUsage(body io.ReadCloser, userID int, path string, tracker *UsageTracker) io.ReadCloser {
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		log.Printf("ERROR: failed to read response body for usage tracking: %v", err)
		return io.NopCloser(bytes.NewReader(data))
	}

	var resp nonStreamResponse
	if err := json.Unmarshal(data, &resp); err == nil && resp.Usage != nil {
		tracker.Track(db.UsageRecord{
			UserID:           userID,
			Path:             path,
			Model:            resp.Model,
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		})
	}

	return io.NopCloser(bytes.NewReader(data))
}
