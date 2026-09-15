package replay

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/cdc"
)

func TestPositionClassification(t *testing.T) {
	for _, tc := range []struct {
		name, phase, database, table string
		missing, want                bool
	}{
		{"inside", "BEGIN", "business", "standards", false, true},
		{"after", "END", "business", "standards", false, false},
		{"different database", "BEGIN", "other", "standards", false, false},
		{"different table", "BEGIN", "business", "other", false, false},
		{"before first marker", "", "", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			o := Observer{DB: db, Database: "control", uuid: "uuid"}
			rows := sqlmock.NewRows([]string{"phase", "database_name", "table_name"})
			if !tc.missing {
				rows.AddRow(tc.phase, tc.database, tc.table)
			}
			mock.ExpectQuery("SELECT phase,database_name,table_name FROM sync_replay_position").WithArgs("uuid", "mysql-bin.000001", uint32(200)).WillReturnRows(rows)
			got, err := o.Contains(context.Background(), cdc.ChangeEvent{DatabaseName: "business", TableName: "standards", BinlogFile: "mysql-bin.000001", BinlogPos: 200})
			if err != nil || got != tc.want {
				t.Fatal(got, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestObserverPersistsBeforeAckAndRejectsUncommittedProof(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		o := Observer{DB: db, Database: "control", uuid: "uuid"}
		token := strings.Repeat("a", 64)
		c := cdc.ChangeEvent{DatabaseName: "control", TableName: Table, Operation: cdc.OperationInsert, BinlogFile: "mysql-bin.000001", BinlogPos: 100, After: map[string]any{"token": token, "phase": "BEGIN", "database_name": "business", "table_name": "standards"}}
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM sync_replay_marker").WithArgs(token, "business", "standards").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
		if count == 2 {
			mock.ExpectExec("INSERT INTO sync_replay_position").WithArgs("uuid", c.BinlogFile, c.BinlogPos, "BEGIN", token, "business", "standards").WillReturnResult(sqlmock.NewResult(0, 1))
		}
		err = o.Observe(context.Background(), c)
		if (err == nil) != (count == 2) {
			t.Fatal(count, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestObserverFailsClosed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	o := Observer{DB: db, Database: "control", uuid: "uuid"}
	if _, err := o.Contains(context.Background(), cdc.ChangeEvent{}); err == nil {
		t.Fatal("missing position accepted")
	}
	mock.ExpectQuery("SELECT phase").WillReturnError(errors.New("offline"))
	if _, err := o.Contains(context.Background(), cdc.ChangeEvent{BinlogFile: "mysql-bin.000001", BinlogPos: 100}); err == nil {
		t.Fatal("lookup error ignored")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRestartReadsDurableBoundary(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for range 2 {
		o := Observer{DB: db, Database: "control"}
		mock.ExpectQuery("SELECT @@server_uuid").WillReturnRows(sqlmock.NewRows([]string{"uuid"}).AddRow("same-stream"))
		mock.ExpectQuery("SELECT phase").WithArgs("same-stream", "mysql-bin.000009", uint32(400)).WillReturnRows(sqlmock.NewRows([]string{"phase", "database_name", "table_name"}).AddRow("BEGIN", "business", "standards"))
		ok, err := o.Contains(context.Background(), cdc.ChangeEvent{DatabaseName: "business", TableName: "standards", BinlogFile: "mysql-bin.000009", BinlogPos: 400})
		if err != nil || !ok {
			t.Fatal(ok, err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMarkerWriteAndRollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO sync_replay_marker").WithArgs(sqlmock.AnyArg(), "BEGIN", "business", "standards").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sync_replay_marker").WithArgs(sqlmock.AnyArg(), "END", "business", "standards").WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	token, err := Begin(context.Background(), tx, "business", "standards")
	if err != nil {
		t.Fatal(err)
	}
	if err := End(context.Background(), tx, token, "business", "standards"); !errors.Is(err, sql.ErrConnDone) {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
