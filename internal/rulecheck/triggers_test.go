package rulecheck

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRequireNoTriggersNeedsVisibility(t *testing.T) {
	for _, mode := range []string{"safe", "invisible", "trigger"} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery("SELECT CURRENT_USER").WillReturnRows(sqlmock.NewRows([]string{"account"}).AddRow("reader@localhost"))
		visible := 1
		if mode == "invisible" {
			visible = 0
		}
		mock.ExpectQuery("SELECT COUNT.*grants_seen").WithArgs("db", "db", "table", "'reader'@'localhost'").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(visible))
		if mode != "invisible" {
			triggers := 0
			if mode == "trigger" {
				triggers = 1
			}
			mock.ExpectQuery("SELECT COUNT.*information_schema.TRIGGERS").WithArgs("db", "table").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(triggers))
		}
		if err := RequireNoTriggers(context.Background(), db, "db", "table"); (err != nil) != (mode != "safe") {
			t.Fatalf("mode=%s err=%v", mode, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}
