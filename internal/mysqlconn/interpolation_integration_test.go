package mysqlconn_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Opt-in; all writes use a connection-local temporary table.
func TestInterpolationValuesReal(t *testing.T) {
	dsn := os.Getenv("NODEBRIDGE_MYSQL_VALUES_TEST_DSN")
	if dsn == "" {
		t.Skip("NODEBRIDGE_MYSQL_VALUES_TEST_DSN required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	for _, enabled := range []bool{false, true} {
		for _, mode := range []string{"STRICT_TRANS_TABLES", "STRICT_TRANS_TABLES,NO_BACKSLASH_ESCAPES"} {
			t.Run(fmt.Sprintf("interpolate_%t/%s", enabled, mode), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				c := *cfg
				c.InterpolateParams, c.ParseTime, c.Loc = enabled, true, time.UTC
				c.Params = map[string]string{"charset": "utf8mb4"}
				db, err := sql.Open("mysql", c.FormatDSN())
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				conn, err := db.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				if _, err := conn.ExecContext(ctx, "SET SESSION sql_mode = ?", mode); err != nil {
					t.Fatal(err)
				}
				if _, err := conn.ExecContext(ctx, `CREATE TEMPORARY TABLE nb_values (id INT PRIMARY KEY, n BIGINT UNSIGNED, d DECIMAL(38,18), s LONGTEXT, b LONGBLOB, dt DATETIME(6), nullable TEXT, f DOUBLE)`); err != nil {
					t.Fatal(err)
				}
				defer conn.ExecContext(ctx, "DROP TEMPORARY TABLE nb_values")
				textValue := "'\"\\\x00\n\r\x1a?; DROP TABLE nb_values; -- \u4e2d\u6587\u65e5\u672c\U0001f680"
				binary := []byte{0, 1, 26, 39, 34, 92, 128, 255}
				date := time.Date(2026, 9, 10, 1, 2, 3, 123456000, time.UTC)
				const decimal = "9007199254740993.123456789012345678"
				tx, err := conn.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				query := "INSERT INTO nb_values (id,n,d,s,b,dt,nullable,f) VALUES (?,?,?,?,?,?,?,?)"
				args := []any{1, uint64(18446744073709551615), decimal, textValue, binary, date, nil, 1.2345678901234567}
				if _, err := tx.ExecContext(ctx, query, args...); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
				var n uint64
				var d, s string
				var b []byte
				var dt time.Time
				var nullable sql.NullString
				var f float64
				if err := conn.QueryRowContext(ctx, "SELECT n,d,s,b,dt,nullable,f FROM nb_values WHERE id=?", 1).Scan(&n, &d, &s, &b, &dt, &nullable, &f); err != nil {
					t.Fatal(err)
				}
				if n != args[1] || d != decimal || s != textValue || !bytes.Equal(b, binary) || !dt.Equal(date) || nullable.Valid || f != args[7] {
					t.Fatal("round-trip value mismatch")
				}
				tx, err = conn.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				args[0] = 2
				if _, err := tx.ExecContext(ctx, query, args...); err != nil {
					t.Fatal(err)
				}
				if _, err := tx.ExecContext(ctx, query, args...); err == nil {
					t.Fatal("duplicate key accepted")
				}
				if err := tx.Rollback(); err != nil {
					t.Fatal(err)
				}
				var count int
				if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM nb_values").Scan(&count); err != nil || count != 1 {
					t.Fatalf("rollback count=%d err=%v", count, err)
				}
			})
		}
	}
}
