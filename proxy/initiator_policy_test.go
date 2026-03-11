package proxy

import "testing"

func TestInitiatorPolicy_DefaultsToAgentUntilNthRequest(t *testing.T) {
	policy := NewInitiatorPolicy(3)

	if got := policy.NextInitiator(); got != "agent" {
		t.Fatalf("first request = %q, want agent", got)
	}
	if got := policy.NextInitiator(); got != "agent" {
		t.Fatalf("second request = %q, want agent", got)
	}
	if got := policy.NextInitiator(); got != "agent" {
		t.Fatalf("third request = %q, want agent", got)
	}
	if got := policy.NextInitiator(); got != "user" {
		t.Fatalf("fourth request = %q, want user", got)
	}
}

func TestInitiatorPolicy_UpdateOnlyAffectsFutureRequests(t *testing.T) {
	policy := NewInitiatorPolicy(5)

	if got := policy.NextInitiator(); got != "agent" {
		t.Fatalf("first request = %q, want agent", got)
	}

	policy.SetUserEvery(2)

	if got := policy.NextInitiator(); got != "agent" {
		t.Fatalf("second request after update = %q, want agent", got)
	}
	if got := policy.NextInitiator(); got != "user" {
		t.Fatalf("third request after update = %q, want user", got)
	}
}

func TestInitiatorPolicy_GetUserEvery(t *testing.T) {
	policy := NewInitiatorPolicy(7)

	if got := policy.GetUserEvery(); got != 7 {
		t.Fatalf("GetUserEvery() = %d, want 7", got)
	}
}
