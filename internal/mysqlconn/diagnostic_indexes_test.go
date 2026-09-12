package mysqlconn

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func indexRows(columns ...string) *sqlmock.Rows {
	r := sqlmock.NewRows([]string{"COLUMN_NAME", "SUB_PART", "IS_VISIBLE", "NON_UNIQUE"})
	for _, c := range columns {
		r.AddRow(c, nil, "YES", 1)
	}
	return r
}

func TestDiagnosticIndexUpgrade(t *testing.T) {
	for _, spec := range []struct {
		table, index, second string
		ensure               func(context.Context, *sql.DB) error
	}{
		{"sync_event_log", diagnosticIndex, "status", EnsureServerDiagnosticIndexes},
		{"sync_apply_log", applyDiagnosticIndex, "op_type", EnsureApplyDiagnosticIndexes},
	} {
		for _, scenario := range []string{"existing", "add", "concurrent", "wrong_columns", "ddl_denied", "missing_after_add", "query_failed", "invisible", "prefix", "unique"} {
			t.Run(spec.table+"/"+scenario, func(t *testing.T) {
				db, m, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				q := m.ExpectQuery("SELECT COLUMN_NAME").WithArgs(spec.table, spec.index)
				wantError := false
				switch scenario {
				case "existing":
					q.WillReturnRows(indexRows("table_name", spec.second))
				case "wrong_columns":
					q.WillReturnRows(indexRows("table_name", "pk_value"))
					wantError = true
				case "query_failed":
					q.WillReturnError(errors.New("unavailable"))
					wantError = true
				case "invisible":
					q.WillReturnRows(sqlmock.NewRows([]string{"c", "p", "v", "u"}).AddRow("table_name", nil, "NO", 1))
					wantError = true
				case "prefix":
					q.WillReturnRows(sqlmock.NewRows([]string{"c", "p", "v", "u"}).AddRow("table_name", 32, "YES", 1))
					wantError = true
				case "unique":
					q.WillReturnRows(sqlmock.NewRows([]string{"c", "p", "v", "u"}).AddRow("table_name", nil, "YES", 0))
					wantError = true
				default:
					q.WillReturnRows(indexRows())
					e := m.ExpectExec("ALTER TABLE " + spec.table + " ADD INDEX " + spec.index)
					switch scenario {
					case "concurrent":
						e.WillReturnError(&mysql.MySQLError{Number: 1061})
					case "ddl_denied":
						e.WillReturnError(errors.New("denied"))
						wantError = true
					default:
						e.WillReturnResult(sqlmock.NewResult(0, 0))
					}
					if scenario != "ddl_denied" {
						rows := indexRows("table_name", spec.second)
						if scenario == "missing_after_add" {
							rows = indexRows()
							wantError = true
						}
						m.ExpectQuery("SELECT COLUMN_NAME").WithArgs(spec.table, spec.index).WillReturnRows(rows)
					}
				}
				if err := spec.ensure(context.Background(), db); (err != nil) != wantError {
					t.Fatalf("err=%v wantError=%v", err, wantError)
				}
				if err := m.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
