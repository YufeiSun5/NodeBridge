package cdc

import (
	"errors"
	"testing"
	"time"
)

func TestRecoveryPolicyNextDelay(t *testing.T) {
	policy := RecoveryPolicy{
		InitialDelay: time.Second,
		MaxDelay:     5 * time.Second,
		MaxAttempts:  4,
	}

	cases := []struct {
		attempt int
		want    time.Duration
		ok      bool
	}{
		{attempt: 0, want: time.Second, ok: true},
		{attempt: 1, want: 2 * time.Second, ok: true},
		{attempt: 2, want: 4 * time.Second, ok: true},
		{attempt: 3, want: 5 * time.Second, ok: true},
		{attempt: 4, ok: false},
	}
	for _, tc := range cases {
		got, ok := policy.NextDelay(tc.attempt)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("attempt %d expected %s/%t, got %s/%t", tc.attempt, tc.want, tc.ok, got, ok)
		}
	}
}

func TestDefaultRecoveryPolicy(t *testing.T) {
	delay, ok := DefaultRecoveryPolicy().NextDelay(0)
	if !ok || delay <= 0 {
		t.Fatalf("unexpected default delay %s ok=%t", delay, ok)
	}
}

func TestRecoverable(t *testing.T) {
	if Recoverable(nil) {
		t.Fatal("nil error should not be recoverable")
	}
	if Recoverable(ErrFatalRecovery) {
		t.Fatal("fatal error should not be recoverable")
	}
	if !Recoverable(errors.New("network timeout")) {
		t.Fatal("normal error should be recoverable")
	}
}
