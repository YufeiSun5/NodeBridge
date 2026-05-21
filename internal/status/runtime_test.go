package status

import (
	"errors"
	"testing"
	"time"
)

func TestRuntimeStoreRecordsWorkerLifecycle(t *testing.T) {
	store := NewRuntimeStore()
	fixed := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	store.clock = func() time.Time { return fixed }

	store.RecordIdle("server-ingress")
	store.RecordProcessed("server-ingress", "evt-001", "applied", 2)
	store.RecordError("server-ingress", errors.New("mysql down"))
	store.RecordStopped("server-ingress")

	snapshot := store.Snapshot()
	if len(snapshot.Workers) != 1 {
		t.Fatalf("expected one worker, got %+v", snapshot)
	}
	worker := snapshot.Workers[0]
	if worker.Name != "server-ingress" || worker.State != WorkerStopped {
		t.Fatalf("unexpected worker identity/state %+v", worker)
	}
	if worker.LastEventID != "evt-001" || worker.LastAction != "applied" {
		t.Fatalf("unexpected event/action %+v", worker)
	}
	if worker.ProcessedCount != 1 || worker.ErrorCount != 1 || worker.DispatchCount != 2 {
		t.Fatalf("unexpected counts %+v", worker)
	}
	if worker.LastError != "" {
		t.Fatalf("stopped worker should clear last error, got %q", worker.LastError)
	}
	if len(snapshot.Logs) != 4 {
		t.Fatalf("expected lifecycle logs, got %+v", snapshot.Logs)
	}
	if snapshot.Logs[1].EventID != "evt-001" || snapshot.Logs[1].Action != "applied" {
		t.Fatalf("unexpected processed log %+v", snapshot.Logs[1])
	}
}

func TestRuntimeStoreKeepsLogRing(t *testing.T) {
	store := NewRuntimeStore()
	store.logLimit = 2

	store.RecordIdle("worker")
	store.RecordProcessed("worker", "evt-001", "applied", 0)
	store.RecordError("worker", errors.New("failed"))

	logs := store.Snapshot().Logs
	if len(logs) != 2 {
		t.Fatalf("expected two logs, got %+v", logs)
	}
	if logs[0].EventID != "evt-001" || logs[1].Level != "ERROR" {
		t.Fatalf("unexpected logs %+v", logs)
	}
}
