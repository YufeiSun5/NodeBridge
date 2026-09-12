package apply_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
)

func TestUpdateMissingNoopAndMultipleRows(t *testing.T) {
	for _, test := range []struct {
		name           string
		changed, found int
		code           string
	}{
		{"missing", 0, 0, "target_row_missing"},
		{"unchanged", 0, 1, ""},
		{"changed", 1, 0, ""},
		{"multiple_changed", 2, 0, "multiple_target_rows"},
		{"multiple_unchanged", 0, 2, "multiple_target_rows"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT COUNT").WithArgs("evt-001").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectExec("UPDATE `scada_center`.`device_settings` SET").WillReturnResult(sqlmock.NewResult(0, int64(test.changed)))
			if test.changed == 0 {
				rows := sqlmock.NewRows([]string{"1"})
				for range test.found {
					rows.AddRow(1)
				}
				mock.ExpectQuery(regexp.QuoteMeta("SELECT 1 FROM `scada_center`.`device_settings` WHERE `setting_id` = ? LIMIT 2 FOR UPDATE")).WithArgs(7).WillReturnRows(rows)
			}
			if test.code == "" {
				mock.ExpectExec("INSERT INTO sync_apply_log").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			_, err = apply.NewSQLWorker(db).Apply(context.Background(), mappedEvent(event.TypeUpdate))
			if test.code == "" && err != nil || test.code != "" && (err == nil || !strings.Contains(err.Error(), test.code)) {
				t.Fatalf("result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInsertUniqueConflictNeverUpdatesExistingRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	row := mappedEvent(event.TypeInsert)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// Anchoring excludes any ON DUPLICATE KEY UPDATE suffix.
	mock.ExpectExec("^INSERT INTO `scada_center`.`device_settings` \\([^;]+\\) VALUES \\([?, ]+\\)$").WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry for key 'standard_code'"})
	mock.ExpectRollback()
	_, err = apply.NewSQLWorker(db).Apply(context.Background(), row)
	if err == nil || !strings.Contains(err.Error(), "unique_key_conflict") {
		t.Fatalf("result: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHardDeleteRowsAndTransactionFailure(t *testing.T) {
	for _, test := range []struct {
		count   int64
		failLog bool
	}{{0, false}, {1, false}, {2, false}, {1, true}} {
		t.Run(fmt.Sprintf("rows=%d/logFail=%t", test.count, test.failLog), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			row := mappedEvent(event.TypeDelete)
			row.DeleteMode = rules.DeleteHard
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `scada_center`.`device_settings` WHERE `setting_id` = ?")).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, test.count))
			if test.count <= 1 {
				log := mock.ExpectExec("INSERT INTO sync_apply_log")
				if test.failLog {
					log.WillReturnError(fmt.Errorf("receipt failure"))
				} else {
					log.WillReturnResult(sqlmock.NewResult(1, 1))
				}
			}
			if test.count > 1 || test.failLog {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			_, err = apply.NewSQLWorker(db).Apply(context.Background(), row)
			if (err != nil) != (test.count > 1 || test.failLog) {
				t.Fatalf("result: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIncompleteTargetKeyRejectedBeforeTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, key := range []map[string]any{nil, {"setting_id": nil}, {"setting_id": 7}} {
		row := mappedEvent(event.TypeDelete)
		row.TargetKeyColumns = []string{"setting_id", "tenant_id"}
		row.TargetPrimaryKey = key
		if _, err := apply.NewSQLWorker(db).Apply(context.Background(), row); err == nil {
			t.Fatal("incomplete key accepted")
		}
		if _, err := apply.NewSQLWorker(db).ApplyBatch(context.Background(), []mapper.MappedEvent{row}); err == nil {
			t.Fatal("incomplete batch key accepted")
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
