package agentstate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAgentLeaseLifecycle(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.yaml")
	state, err := Read(config)
	if err != nil || state != nil {
		t.Fatalf("initial: %v %v", state, err)
	}
	release, err := Acquire(config, "stop")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	state, err = Read(config)
	if err != nil || state == nil || state.PID != os.Getpid() || state.StopFile != "stop" {
		t.Fatalf("active: %v %v", state, err)
	}
	if other, err := Acquire(config, "stop"); !errors.Is(err, ErrRunning) {
		if other != nil {
			other()
		}
		t.Fatalf("duplicate lease: %v", err)
	}
	release()
	state, err = Read(config)
	if err != nil || state != nil {
		t.Fatalf("released: %v %v", state, err)
	}
	_, statePath := paths(config)
	if err := os.WriteFile(statePath, []byte(`{"pid":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err = Read(config)
	if err != nil || state != nil {
		t.Fatalf("stale state trusted: %v %v", state, err)
	}
}
