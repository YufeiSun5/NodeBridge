package syncruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/status"
)

func TestWorkerRunsProcessedAndStopsAtMaxSteps(t *testing.T) {
	store := status.NewRuntimeStore()
	stepper := &sequenceStepper{results: []StepResult{
		{Processed: true, EventID: "evt-001", Action: "applied", DispatchCount: 1},
		{Processed: true, EventID: "evt-002", Action: "applied"},
	}}

	err := Worker{
		Config:  WorkerConfig{Name: "server-ingress", MaxSteps: 2},
		Stepper: stepper,
		Status:  store,
		Sleep:   noSleep,
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	worker := singleWorker(t, store)
	if worker.State != status.WorkerStopped {
		t.Fatalf("expected stopped worker, got %+v", worker)
	}
	if worker.ProcessedCount != 2 || worker.LastEventID != "evt-002" || worker.DispatchCount != 1 {
		t.Fatalf("unexpected worker status %+v", worker)
	}
}

func TestWorkerSleepsWhenIdle(t *testing.T) {
	var slept []time.Duration
	err := Worker{
		Config: WorkerConfig{Name: "edge-upload", IdleInterval: 7 * time.Second, MaxSteps: 1},
		Stepper: &sequenceStepper{results: []StepResult{
			{Action: "empty"},
		}},
		Sleep: func(ctx context.Context, delay time.Duration) error {
			slept = append(slept, delay)
			return nil
		},
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(slept) != 1 || slept[0] != 7*time.Second {
		t.Fatalf("unexpected sleep calls %+v", slept)
	}
}

func TestWorkerRecordsErrorAndContinues(t *testing.T) {
	store := status.NewRuntimeStore()
	stepper := &sequenceStepper{
		errors: map[int]error{0: errors.New("temporary broker error")},
		results: []StepResult{
			{},
			{Processed: true, EventID: "evt-001", Action: "forwarded"},
		},
	}

	err := Worker{
		Config:  WorkerConfig{Name: "edge-upload", ErrorInterval: time.Second, MaxSteps: 2},
		Stepper: stepper,
		Status:  store,
		Sleep:   noSleep,
	}.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	worker := singleWorker(t, store)
	if worker.ErrorCount != 1 || worker.ProcessedCount != 1 {
		t.Fatalf("unexpected counts %+v", worker)
	}
	if worker.LastEventID != "evt-001" {
		t.Fatalf("unexpected last event %+v", worker)
	}
}

func TestWorkerStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Worker{
		Config: WorkerConfig{Name: "edge-upload"},
		Stepper: &sequenceStepper{results: []StepResult{
			{Processed: true, EventID: "evt-001", Action: "forwarded"},
		}},
		Sleep: noSleep,
	}.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

type sequenceStepper struct {
	results []StepResult
	errors  map[int]error
	index   int
}

func (s *sequenceStepper) RunOnce(ctx context.Context) (StepResult, error) {
	index := s.index
	s.index++
	if s.errors != nil && s.errors[index] != nil {
		return StepResult{}, s.errors[index]
	}
	if index >= len(s.results) {
		return StepResult{Action: "empty"}, nil
	}
	return s.results[index], nil
}

func noSleep(ctx context.Context, delay time.Duration) error {
	return nil
}

func singleWorker(t *testing.T, store *status.RuntimeStore) status.WorkerStatus {
	t.Helper()
	snapshot := store.Snapshot()
	if len(snapshot.Workers) != 1 {
		t.Fatalf("expected one worker, got %+v", snapshot)
	}
	return snapshot.Workers[0]
}
