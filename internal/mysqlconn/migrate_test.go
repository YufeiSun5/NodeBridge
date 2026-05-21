package mysqlconn_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
)

func TestRunMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "002_second.sql"), "CREATE TABLE second (id BIGINT);")
	writeFile(t, filepath.Join(dir, "001_first.sql"), "CREATE TABLE first (id BIGINT);")

	mock.ExpectExec("CREATE TABLE first").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE second").WillReturnResult(sqlmock.NewResult(0, 0))

	if err := mysqlconn.RunMigrations(context.Background(), db, dir); err != nil {
		t.Fatalf("RunMigrations returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
