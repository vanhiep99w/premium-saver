package proxy

import "testing"

func TestFormatOutgoingRequestLog(t *testing.T) {
	got := formatOutgoingRequestLog("agent", "POST", "/chat/completions")
	want := "-> agent POST /chat/completions"

	if got != want {
		t.Fatalf("formatOutgoingRequestLog() = %q, want %q", got, want)
	}
}
