package mysqlconn

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSystemUpgradeRetryChecksumAndDemoBoundary(t *testing.T) {
	for _, state := range []string{"new", "applied", "changed", "failed"} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "001_mvp_tables.sql"), []byte("CREATE TABLE IF NOT EXISTS business_example (id INT); CREATE TABLE IF NOT EXISTS sync_owned (id INT);"), 0600); err != nil {
				t.Fatal(err)
			}
			migrations, err := readSystemMigrations(dir)
			if err != nil || len(migrations) != 1 || len(migrations[0].statements) != 1 {
				t.Fatal(migrations, err)
			}
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT GET_LOCK").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(1))
			mock.ExpectExec("CREATE TABLE IF NOT EXISTS sync_schema_migration").WillReturnResult(sqlmock.NewResult(0, 0))
			rows := sqlmock.NewRows([]string{"checksum"})
			if state == "applied" {
				rows.AddRow(migrations[0].checksum)
			}
			if state == "changed" {
				rows.AddRow("old-different-checksum")
			}
			mock.ExpectQuery("SELECT checksum").WithArgs("001_mvp_tables.sql").WillReturnRows(rows)
			if state == "new" {
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS sync_owned").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("INSERT INTO sync_schema_migration").WithArgs("001_mvp_tables.sql", migrations[0].checksum, "0.48.5").WillReturnResult(sqlmock.NewResult(1, 1))
			}
			if state == "failed" {
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS sync_owned").WillReturnError(errors.New("denied"))
			}
			mock.ExpectExec("DO RELEASE_LOCK").WillReturnResult(sqlmock.NewResult(0, 0))
			err = UpgradeSystem(context.Background(), db, dir, "0.48.5")
			if (err == nil) != (state == "new" || state == "applied") {
				t.Fatal(state, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSystemUpgradeAcceptsBundledMigrations(t *testing.T) {
	for _, scope := range []string{"edge", "server"} {
		migrations, err := readSystemMigrations(filepath.Join("..", "..", "migrations", scope))
		if err != nil || len(migrations) < 10 {
			t.Fatal(scope, err)
		}
	}
}
