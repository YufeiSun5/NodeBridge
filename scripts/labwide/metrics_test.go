package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMetricsRowsTrace(t *testing.T) {
	for _, mode := range []string{"success", "query_error", "row_error", "scan_error", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			expect := mock.ExpectQuery("SELECT private_value")
			rows := sqlmock.NewRows([]string{"value"}).AddRow(9).AddRow(10)
			if mode == "query_error" {
				expect.WillReturnError(errors.New("query failed"))
			} else {
				if mode == "row_error" {
					rows.RowError(1, errors.New("row failed"))
				}
				if mode == "scan_error" {
					rows = sqlmock.NewRows([]string{"value"}).AddRow("invalid")
				}
				expect.WillReturnRows(rows)
				if mode == "timeout" {
					expect.WillDelayFor(time.Second)
				} else {
					expect.RowsWillBeClosed()
				}
			}
			deadline := 5 * time.Second
			if mode == "timeout" {
				deadline = 50 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			var out bytes.Buffer
			var values []int
			err = metricsRows(ctx, &out, db, "server-001", "apply_counts", "SELECT private_value", nil, func(rows *sql.Rows) error {
				if !strings.Contains(out.String(), `"phase":"start"`) {
					t.Fatal("trace must start before scanning")
				}
				var n int
				if err := rows.Scan(&n); err != nil {
					return err
				}
				values = append(values, n)
				return nil
			})
			failed := mode != "success"
			if (err != nil) != failed || (failed && !strings.Contains(err.Error(), "server-001 metrics.apply_counts")) {
				t.Fatalf("err=%v", err)
			}
			if !failed && (len(values) != 2 || values[0] != 9 || values[1] != 10) {
				t.Fatalf("values=%v", values)
			}
			var begin, end snapshotTiming
			dec := json.NewDecoder(&out)
			if err := dec.Decode(&begin); err != nil {
				t.Fatal(err)
			}
			if err := dec.Decode(&end); err != nil {
				t.Fatal(err)
			}
			if begin.Phase != "start" || end.Phase != "end" || end.Failed != failed || end.ElapsedMS < 0 {
				t.Fatalf("begin=%+v end=%+v", begin, end)
			}
			if mode == "timeout" && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Fatalf("timeout did not cancel the context: %v", ctx.Err())
			}
			if strings.Contains(out.String(), "private_value") {
				t.Fatal("trace leaked SQL")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
