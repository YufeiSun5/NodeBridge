package apply_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestConflictApplyCannotRunWithoutLocalCaptureFence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mapped := mappedEvent(event.TypeInsert)
	mapped.ConflictPolicy = rules.ConflictLastWriteWin
	if _, err := apply.NewSQLWorker(db).Apply(context.Background(), mapped); err == nil || !strings.Contains(err.Error(), "capture_fence_required") {
		t.Fatalf("got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type unusedFence struct{ t *testing.T }

func (f unusedFence) Wait(context.Context) error {
	f.t.Fatal("invalid event reached fence")
	return nil
}

func TestConflictMappingFreezesOriginalAndRejectsChangedProjection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	evt := event.SyncEvent{EventID: "evt", OriginNodeID: "remote", DatabaseName: "remote_db", TableName: "source", EventType: event.TypeInsert, PrimaryKey: map[string]any{"id": 1}, After: map[string]any{"id": 1, "value": "original"}}
	mapped, err := mapper.MapEvent(evt, rules.SyncRule{PrimaryKeys: []string{"id"}, TargetDatabaseName: "local_db", TargetTableName: "target", ConflictPolicy: rules.ConflictLastWriteWin})
	if err != nil {
		t.Fatal(err)
	}
	evt.After["value"] = "changed-after-mapping"
	var frozen event.SyncEvent
	if err := json.Unmarshal(mapped.ConflictSource, &frozen); err != nil || frozen.After["value"] != "original" || frozen.DatabaseName != "remote_db" {
		t.Fatalf("original event was not frozen: %+v %v", frozen, err)
	}
	mapped.TargetAfter["value"] = "tampered"
	worker := apply.NewSQLWorker(db)
	worker.CaptureFence = unusedFence{t}
	if _, err := worker.Apply(context.Background(), mapped); err == nil || !strings.Contains(err.Error(), "projection_mismatch") {
		t.Fatalf("changed mapped payload accepted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepairWorkerDoesNotTouchDisabledRules(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker := apply.RepairWorker{DB: db, NodeID: "local", CaptureFence: unusedFence{t}, Rules: &rules.RuleSet{Rules: []rules.SyncRule{{ID: "disabled", DatabaseName: "db", TableName: "table", Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin}}}}
	if repaired, err := worker.RunOnce(context.Background()); err != nil || repaired {
		t.Fatalf("disabled rule repair=%t error=%v", repaired, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
