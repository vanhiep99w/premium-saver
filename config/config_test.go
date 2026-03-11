package config

import "testing"

func TestInitiatorUserEvery_DefaultWhenUnset(t *testing.T) {
	t.Setenv("X_INITIATOR_USER_EVERY", "")

	if got := InitiatorUserEvery(); got != 7 {
		t.Fatalf("InitiatorUserEvery() = %d, want 7", got)
	}
}

func TestInitiatorUserEvery_UsesValidEnv(t *testing.T) {
	t.Setenv("X_INITIATOR_USER_EVERY", "7")

	if got := InitiatorUserEvery(); got != 7 {
		t.Fatalf("InitiatorUserEvery() = %d, want 7", got)
	}
}

func TestInitiatorUserEvery_FallsBackForInvalidEnv(t *testing.T) {
	t.Setenv("X_INITIATOR_USER_EVERY", "0")

	if got := InitiatorUserEvery(); got != 7 {
		t.Fatalf("InitiatorUserEvery() with zero = %d, want 7", got)
	}

	t.Setenv("X_INITIATOR_USER_EVERY", "abc")

	if got := InitiatorUserEvery(); got != 7 {
		t.Fatalf("InitiatorUserEvery() with invalid string = %d, want 7", got)
	}
}
