package syncruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/status"
)

type stepperFunc func(context.Context) (StepResult, error)

func (f stepperFunc) RunOnce(ctx context.Context) (StepResult, error) { return f(ctx) }

func TestWorkerLogsErrorContextRecoveryAndSummary(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	step := 0
	w := Worker{Config: WorkerConfig{Name: "server-ingress", MaxSteps: 2, ErrorInterval: 3 * time.Second, SummaryInterval: time.Nanosecond}, Logger: logger, Sleep: noSleep, Stepper: stepperFunc(func(ctx context.Context) (StepResult, error) {
		defer measurePhase(ctx, "mysql_apply")()
		step++
		if step == 1 {
			return StepResult{Processed: true, EventID: "evt-failed", Action: "failed", Count: 50}, errors.New("apply update: Error 1205 (HY000): timeout")
		}
		time.Sleep(2 * time.Millisecond)
		return StepResult{Processed: true, EventID: "evt-ok", Action: "applied", Count: 50}, nil
	})}
	if err := w.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var failure, recovery, summary bool
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
		var r map[string]any
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatal(err)
		}
		switch r["msg"] {
		case "worker step failed; retry scheduled":
			failure = true
			if r["event_id"] != "evt-failed" || r["batch_count"] != float64(50) || r["retry_after_ms"] != float64(3000) || r["phase_ms"] == nil || r["error"] == nil {
				t.Fatalf("incomplete context: %s", line)
			}
		case "worker step recovered":
			recovery = true
		case "worker interval":
			summary = true
		}
	}
	if !failure || !recovery || !summary {
		t.Fatalf("missing records: %s", output.String())
	}
}

type noticeWriter struct {
	bytes.Buffer
	notice chan struct{}
}

func (w *noticeWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if bytes.Contains(p, []byte("worker step still running")) {
		select {
		case w.notice <- struct{}{}:
		default:
		}
	}
	return n, err
}

func TestWorkerLogsWhileCallIsStillBlocked(t *testing.T) {
	output := &noticeWriter{notice: make(chan struct{}, 1)}
	logger := slog.New(slog.NewJSONHandler(output, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w := Worker{Config: WorkerConfig{Name: "edge-downlink", MaxSteps: 1, SlowStepThreshold: time.Millisecond}, Logger: logger, Stepper: stepperFunc(func(ctx context.Context) (StepResult, error) {
		defer measurePhase(ctx, "mysql_apply")()
		select {
		case <-output.notice:
			return StepResult{Processed: true, Action: "applied", EventID: "a", Count: 1}, nil
		case <-ctx.Done():
			return StepResult{}, ctx.Err()
		}
	})}
	if err := w.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "mysql_apply") || !strings.Contains(output.String(), "slow worker step") {
		t.Fatalf("missing timing: %s", output.String())
	}
}

func TestWorkerCancellationDoesNotCreateFalseError(t *testing.T) {
	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	store := status.NewRuntimeStore()
	w := Worker{Logger: slog.New(slog.NewJSONHandler(&output, nil)), Status: store, Stepper: stepperFunc(func(context.Context) (StepResult, error) { cancel(); return StepResult{}, context.Canceled })}
	if err := w.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), `"level":"ERROR"`) {
		t.Fatalf("false error: %s", output.String())
	}
	if store.Snapshot().Workers[0].ErrorCount != 0 {
		t.Fatal("cancellation counted as failure")
	}
}

type failingCleanupStepper struct{}

func (failingCleanupStepper) RunOnce(context.Context) (StepResult, error) { return StepResult{}, nil }
func (failingCleanupStepper) Stop(ctx context.Context) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("cleanup has no deadline")
	}
	return errors.New("cleanup failed")
}
func TestWorkerReportsCleanupFailure(t *testing.T) {
	var output bytes.Buffer
	err := (Worker{Config: WorkerConfig{MaxSteps: 1}, Stepper: failingCleanupStepper{}, Sleep: noSleep, Logger: slog.New(slog.NewJSONHandler(&output, nil))}).Run(context.Background())
	if err == nil || !strings.Contains(output.String(), "worker cleanup failed") {
		t.Fatalf("cleanup lost: %v %s", err, output.String())
	}
}
