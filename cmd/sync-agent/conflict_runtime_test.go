package main

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/capture"
	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type conflictSourceStub struct{}

func (*conflictSourceStub) Start(context.Context) error { return nil }
func (*conflictSourceStub) Stop(context.Context) error  { return nil }
func (*conflictSourceStub) FetchChangesOnce(context.Context) ([]cdc.ChangeEvent, cdc.Offset, error) {
	return nil, cdc.Offset{}, nil
}
func (*conflictSourceStub) Commit(context.Context, cdc.Offset) error { return nil }

func TestAttachConflictRuntimeSharesFence(t *testing.T) {
	cfg := &appconfig.Config{}
	cfg.Node.ID, cfg.MySQL.Database, cfg.CDC.Type = "edge-test", "nb_test", "canal"
	set := &rules.RuleSet{Rules: []rules.SyncRule{{Enable: true, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin}}}
	db := &sql.DB{} // Construction must not perform IO.
	worker := apply.NewCheckedSQLWorker(db)
	source := &conflictSourceStub{}
	wrapped, recorder, runtime, err := attachConflictRuntime(cfg, set, db, worker, source)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := wrapped.(*capture.Source); !ok {
		t.Fatalf("source %T", wrapped)
	}
	local := recorder.(conflict.LocalRecorder)
	repair := runtime.Repairer.(apply.RepairWorker)
	if worker.CaptureFence == nil || worker.CaptureFence != repair.CaptureFence || local.DB != db || repair.DB != db || local.NodeID != cfg.Node.ID || repair.NodeID != cfg.Node.ID || local.Rules != set || repair.Rules != set {
		t.Fatal("components do not share node dependencies")
	}
	if _, _, _, err := attachConflictRuntime(cfg, set, db, worker, source); err == nil {
		t.Fatal("double attachment accepted")
	}
	worker = apply.NewCheckedSQLWorker(db)
	if _, _, _, err := attachConflictRuntime(cfg, set, db, worker, nil); err == nil || worker.CaptureFence != nil {
		t.Fatal("failed construction mutated worker")
	}
	cfg.CDC.Type = "mock"
	if _, _, _, err := attachConflictRuntime(cfg, set, db, worker, source); err == nil {
		t.Fatal("non-Canal accepted")
	}
}

func TestConflictRuntimeDisabledPreservesSource(t *testing.T) {
	source := &conflictSourceStub{}
	for _, set := range []*rules.RuleSet{nil, {}, {Rules: []rules.SyncRule{{Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin}}}, {Rules: []rules.SyncRule{{Enable: true, Direction: rules.DirectionEdgeToServer}}}} {
		got, recorder, repair, err := attachConflictRuntime(nil, set, nil, nil, source)
		if err != nil || got != source || recorder != nil || repair != nil {
			t.Fatal("inactive rules changed runtime")
		}
	}
}

func TestConflictCaptureFilterKeepsBusinessAndAddsOnlyLocalFence(t *testing.T) {
	pattern := regexp.MustCompile("^(?:" + conflictCaptureFilter(`source\.(orders|items)`, "nb_test") + ")$")
	for _, name := range []string{"source.orders", "source.items", "nb_test.sync_capture_fence"} {
		if !pattern.MatchString(name) {
			t.Fatal(name)
		}
	}
	for _, name := range []string{"other.sync_capture_fence", "nb_test.other", "source.unrelated"} {
		if pattern.MatchString(name) {
			t.Fatal(name)
		}
	}
	if conflictCaptureFilter("", "nb_test") != "" {
		t.Fatal("empty default narrowed")
	}
}
