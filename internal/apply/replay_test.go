package apply

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func replayFixture(operation, mode string) mapper.MappedEvent {
	after := map[string]any{"id": int64(1), "value": "new", "last_event_id": "", "updated_by_node": "previous"}
	return mapper.MappedEvent{Event: event.SyncEvent{EventID: "evt-current", EventType: operation, OriginNodeID: "edge-001", SourceNodeID: "edge-001", After: after}, SourceDatabase: "edge", SourceTable: "source", TargetDatabase: "center", TargetTable: "target", SyncMode: mode, TargetPrimaryKey: map[string]any{"id": int64(1)}, TargetAfter: after}
}

func TestReplayWriteKeepsSourceImmutableAndHonorsMappedColumns(t *testing.T) {
	original := replayFixture(event.TypeUpdate, rules.SyncModeOrderedCRUD)
	prepared := replayWrite(original)
	if prepared.TargetAfter["last_event_id"] != "evt-current" || prepared.TargetAfter["updated_by_node"] != "edge-001" {
		t.Fatal(prepared.TargetAfter)
	}
	if original.Event.After["last_event_id"] != "" || original.TargetAfter["updated_by_node"] != "previous" {
		t.Fatal("mutated source event")
	}
	original.TargetColumns = map[string]string{"last_event_id": "event_ref", "updated_by_node": "writer"}
	original.TargetAfter = map[string]any{"event_ref": "old", "writer": "old"}
	prepared = replayWrite(original)
	if prepared.TargetAfter["event_ref"] != "evt-current" || prepared.TargetAfter["writer"] != "edge-001" || len(prepared.TargetAfter) != 2 {
		t.Fatal(prepared.TargetAfter)
	}
	if original.TargetAfter["event_ref"] != "old" {
		t.Fatal("mutated mapped image")
	}
	original.TargetAfter = map[string]any{"id": 1, "value": "business"}
	if len(replayWrite(original).TargetAfter) != 2 {
		t.Fatal("invented metadata columns")
	}
}

func TestSQLWorkerSingleWritesReplayMetadataInBusinessTransaction(t *testing.T) {
	for _, operation := range []string{event.TypeInsert, event.TypeUpdate} {
		t.Run(operation, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT COUNT\\(1\\) FROM sync_apply_log").WithArgs("evt-current").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			if operation == event.TypeInsert {
				mock.ExpectExec("INSERT INTO `center`.`target`").WithArgs(int64(1), "evt-current", "edge-001", "new").WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectExec("UPDATE `center`.`target`").WithArgs("evt-current", "edge-001", "new", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			if _, err = NewSQLWorker(db).Apply(context.Background(), replayFixture(operation, rules.SyncModeOrderedCRUD)); err != nil {
				t.Fatal(err)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSQLWorkerAppendBatchWritesReplayMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	evt := replayFixture(event.TypeInsert, rules.SyncModeAppendOnly)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT event_id FROM sync_apply_log WHERE event_id IN").WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("INSERT INTO `center`.`target`").WithArgs(int64(1), "evt-current", "edge-001", "new").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := NewSQLWorker(db).ApplyBatch(context.Background(), []mapper.MappedEvent{evt, evt})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 2 || !result.Results[1].AlreadyApplied {
		t.Fatal(result)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLWorkerCompactBatchUsesLastEventMarker(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := replayFixture(event.TypeUpdate, rules.SyncModeCRUDCompact)
	b := replayFixture(event.TypeUpdate, rules.SyncModeCRUDCompact)
	b.Event.EventID = "evt-next"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT event_id FROM sync_apply_log WHERE event_id IN").WillReturnRows(sqlmock.NewRows([]string{"event_id"}))
	mock.ExpectExec("SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE `center`.`target`").WithArgs("evt-next", "edge-001", "new", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("RELEASE SAVEPOINT nb_apply_0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if _, err = NewSQLWorker(db).ApplyBatch(context.Background(), []mapper.MappedEvent{a, b}); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
