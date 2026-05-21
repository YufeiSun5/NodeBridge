package cdc

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLOffsetStoreLoadFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	updatedAt := time.Date(2026, 5, 21, 13, 30, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT reader_name, binlog_file, binlog_pos, gtid, updated_at
FROM sync_upload_offset
WHERE reader_name = ?
`)).
		WithArgs("edge-001").
		WillReturnRows(sqlmock.NewRows([]string{"reader_name", "binlog_file", "binlog_pos", "gtid", "updated_at"}).
			AddRow("edge-001", "mysql-bin.000001", int64(42), "gtid-1", updatedAt))

	offset, ok, err := NewMySQLOffsetStore(db).Load(context.Background(), "edge-001")
	if err != nil || !ok {
		t.Fatalf("expected loaded offset, ok=%t err=%v", ok, err)
	}
	if offset.BinlogFile != "mysql-bin.000001" || offset.BinlogPos != 42 || offset.GTID != "gtid-1" {
		t.Fatalf("unexpected offset %+v", offset)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMySQLOffsetStoreLoadMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT reader_name").WithArgs("edge-001").WillReturnError(sql.ErrNoRows)
	_, ok, err := NewMySQLOffsetStore(db).Load(context.Background(), "edge-001")
	if err != nil || ok {
		t.Fatalf("expected missing offset, ok=%t err=%v", ok, err)
	}
}

func TestMySQLOffsetStoreSave(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	updatedAt := time.Date(2026, 5, 21, 13, 30, 0, 0, time.UTC)
	store := NewMySQLOffsetStore(db)
	store.Clock = func() time.Time { return updatedAt }

	mock.ExpectExec("INSERT INTO sync_upload_offset").
		WithArgs("edge-001", "mysql-bin.000001", int64(42), nil, updatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.Save(context.Background(), Offset{
		ReaderName: "edge-001",
		BinlogFile: "mysql-bin.000001",
		BinlogPos:  42,
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMySQLOffsetStoreRequiresDB(t *testing.T) {
	store := NewMySQLOffsetStore(nil)
	if _, _, err := store.Load(context.Background(), "edge-001"); err == nil {
		t.Fatal("expected load db error")
	}
	if err := store.Save(context.Background(), Offset{ReaderName: "edge-001", GTID: "gtid"}); err == nil {
		t.Fatal("expected save db error")
	}
}
