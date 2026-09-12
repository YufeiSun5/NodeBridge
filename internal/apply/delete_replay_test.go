package apply_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func TestTrackedHardDeleteAtomicReceipt(t *testing.T) {
	for _, failReceipt := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mapped := mappedEvent(event.TypeDelete)
		mapped.DeleteMode = rules.DeleteHard
		mapped.TrackDeleteReplay = true
		mapped.TargetBefore = map[string]any{"last_event_id": "older", "updated_by_node": "remote"}
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT COUNT.*FROM sync_apply_log").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery("SELECT 1 FROM .* FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		mock.ExpectQuery("SELECT CURRENT_USER").WillReturnRows(sqlmock.NewRows([]string{"account"}).AddRow("root@localhost"))
		mock.ExpectQuery("SELECT COUNT.*grants_seen").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT COUNT.*information_schema.TRIGGERS").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectExec("UPDATE .* SET `last_event_id`=.*, `updated_by_node`=.* WHERE").WithArgs("evt-001", "edge-001", 7).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT INTO sync_delete_replay").WithArgs("evt-001", "edge-001", "scada_center", "device_settings").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 1))
		receipt := mock.ExpectExec("INSERT INTO sync_apply_log")
		if failReceipt {
			receipt.WillReturnError(errors.New("receipt failed"))
			mock.ExpectRollback()
		} else {
			receipt.WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
		}
		_, err = apply.NewSQLWorker(db).Apply(context.Background(), mapped)
		if (err != nil) != failReceipt {
			t.Fatalf("receipt failure=%t err=%v", failReceipt, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}
