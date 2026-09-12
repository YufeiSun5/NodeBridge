package mysqlconn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

const diagnosticIndex = "idx_table_status"
const applyDiagnosticIndex = "idx_table_op"

// EnsureServerDiagnosticIndexes upgrades existing system tables without rebuilding payload rows.
func EnsureServerDiagnosticIndexes(ctx context.Context, db *sql.DB) error {
	return ensureDiagnosticIndex(ctx, db, "sync_event_log", diagnosticIndex, []string{"table_name", "status"})
}

// EnsureApplyDiagnosticIndexes covers exact operation counts on both node roles.
func EnsureApplyDiagnosticIndexes(ctx context.Context, db *sql.DB) error {
	return ensureDiagnosticIndex(ctx, db, "sync_apply_log", applyDiagnosticIndex, []string{"table_name", "op_type"})
}

func ensureDiagnosticIndex(ctx context.Context, db *sql.DB, table, index string, expected []string) error {
	columns, err := diagnosticIndexColumns(ctx, db, table, index)
	if err != nil {
		return err
	}
	if len(columns) > 0 {
		return validateDiagnosticIndex(index, columns, expected)
	}
	// Identifiers are fixed by the two internal callers, never configuration or SQL input.
	_, err = db.ExecContext(ctx, "ALTER TABLE "+table+" ADD INDEX "+index+" ("+strings.Join(expected, ", ")+"), ALGORITHM=INPLACE, LOCK=NONE")
	if err != nil {
		var serverError *mysql.MySQLError
		if !errors.As(err, &serverError) || serverError.Number != 1061 {
			return fmt.Errorf("add diagnostic index %s.%s: %w", table, index, err)
		}
	}
	columns, err = diagnosticIndexColumns(ctx, db, table, index)
	if err != nil {
		return err
	}
	return validateDiagnosticIndex(index, columns, expected)
}

func diagnosticIndexColumns(ctx context.Context, db *sql.DB, table, index string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT COLUMN_NAME, SUB_PART, IS_VISIBLE, NON_UNIQUE FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? ORDER BY seq_in_index", table, index)
	if err != nil {
		return nil, fmt.Errorf("inspect diagnostic index: %w", err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column, visible string
		var prefix sql.NullInt64
		var nonUnique int
		if err := rows.Scan(&column, &prefix, &visible, &nonUnique); err != nil {
			return nil, err
		}
		if prefix.Valid || visible != "YES" || nonUnique != 1 {
			return nil, errors.New("diagnostic index must be non-unique, visible and use full columns")
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func validateDiagnosticIndex(index string, columns, expected []string) error {
	if strings.Join(columns, ",") != strings.Join(expected, ",") {
		return fmt.Errorf("unexpected %s definition; require %s", index, strings.Join(expected, ","))
	}
	return nil
}
