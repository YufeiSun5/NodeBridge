package syncruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/status"
)

type repairFunc func(context.Context) (bool, error)

func (f repairFunc) RunOnce(ctx context.Context) (bool, error) { return f(ctx) }

func TestConflictRepairRuntimeResults(t *testing.T) {
	failed := errors.New("repair failed")
	for _, tc := range []struct {
		name string
		done bool
		err  error
	}{
		{"idle", false, nil}, {"repaired", true, nil}, {"failed", false, failed}, {"ambiguous_commit", true, failed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := ConflictRepairRuntime{Repairer: repairFunc(func(context.Context) (bool, error) { return tc.done, tc.err })}
			got, err := r.RunOnce(context.Background())
			wantDone := tc.done && tc.err == nil
			if !errors.Is(err, tc.err) || got.Processed != wantDone || got.Action != "conflict_repair" || (got.Count == 1) != wantDone || got.EventID != "" {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
	if _, err := (ConflictRepairRuntime{}).RunOnce(context.Background()); err == nil {
		t.Fatal("missing dependency accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := ConflictRepairRuntime{Repairer: repairFunc(func(context.Context) (bool, error) { t.Fatal("called after cancellation"); return false, nil })}
	if _, err := r.RunOnce(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestConflictRepairWorkerRetriesAndReportsRecovery(t *testing.T) {
	store := status.NewRuntimeStore()
	calls := 0
	r := ConflictRepairRuntime{Repairer: repairFunc(func(context.Context) (bool, error) {
		calls++
		if calls == 1 {
			return false, errors.New("repair transaction rolled back")
		}
		return calls == 2, nil
	})}
	var delays []time.Duration
	w := Worker{Config: WorkerConfig{Name: "conflict-repair", MaxSteps: 3, ErrorInterval: 7 * time.Second, IdleInterval: 2 * time.Second}, Stepper: r, Status: store, Sleep: func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		if len(delays) == 1 {
			s := store.Snapshot().Workers[0]
			if s.State != status.WorkerError || s.ErrorCount != 1 || s.ProcessedCount != 0 {
				t.Fatalf("premature success: %+v", s)
			}
		}
		return nil
	}}
	if err := w.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := store.Snapshot().Workers[0]
	if calls != 3 || len(delays) != 2 || delays[0] != 7*time.Second || delays[1] != 2*time.Second || s.ProcessedCount != 1 || s.ErrorCount != 1 || s.State != status.WorkerStopped {
		t.Fatalf("calls=%d delays=%v status=%+v", calls, delays, s)
	}
}
