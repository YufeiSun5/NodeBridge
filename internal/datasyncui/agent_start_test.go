package datasyncui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
)

func TestStartWaitRequiresReadyAndStableProcess(t *testing.T) {
	for _, scenario := range []string{"ready", "exit", "exit_after_ready", "never_ready", "wrong_pid"} {
		t.Run(scenario, func(t *testing.T) {
			config := filepath.Join(t.TempDir(), "config.yaml")
			release, err := agentstate.Acquire(config, "stop")
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			done := make(chan error, 1)
			pid := os.Getpid()
			if scenario == "ready" || scenario == "exit_after_ready" || scenario == "wrong_pid" {
				if err := agentstate.PublishReady(config); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "exit" {
				done <- errors.New("alignment_topology_pending")
			}
			if scenario == "exit_after_ready" {
				time.AfterFunc(100*time.Millisecond, func() { done <- errors.New("startup failed") })
			}
			if scenario == "wrong_pid" {
				pid++
			}
			err = waitAgentReady(context.Background(), config, pid, done, 500*time.Millisecond)
			if (err == nil) != (scenario == "ready") {
				t.Fatal(scenario, err)
			}
		})
	}
}

func TestStartupTailExcludesEarlierAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.log")
	if err := os.WriteFile(path, []byte("old failure\nnew alignment_topology_pending\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tail := startupLogTail(path, int64(len("old failure\n")))
	if strings.Contains(tail, "old failure") || !strings.Contains(tail, "alignment_topology_pending") {
		t.Fatal(tail)
	}
}
