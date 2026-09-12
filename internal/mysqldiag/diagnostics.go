// Package mysqldiag provides bounded, payload-free diagnostics for the configured database.
package mysqldiag

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Request struct {
	EventTables    []string `json:"event_tables,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}
type Step struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	ElapsedMS float64 `json:"elapsed_ms"`
	ErrorCode string  `json:"error_code,omitempty"`
	Data      any     `json:"data,omitempty"`
}
type Report struct {
	Database  string    `json:"database"`
	At        time.Time `json:"at"`
	Status    string    `json:"status"`
	ElapsedMS float64   `json:"elapsed_ms"`
	Steps     []Step    `json:"steps"`
}
type TableStats struct {
	Table         string `json:"table"`
	EstimatedRows int64  `json:"estimated_rows"`
	DataBytes     int64  `json:"data_bytes"`
	IndexBytes    int64  `json:"index_bytes"`
}
type IndexColumn struct {
	Table        string `json:"table"`
	Index        string `json:"index"`
	Column       string `json:"column"`
	Position     int    `json:"position"`
	PrefixLength *int64 `json:"prefix_length,omitempty"`
	Visible      bool   `json:"visible"`
}

var identifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,63}$`)

func (r Request) Validate() error {
	if r.TimeoutSeconds < 0 || r.TimeoutSeconds > 10 {
		return errors.New("timeout_seconds must be 1..10, or 0 for the 5-second default")
	}
	if len(r.EventTables) > 8 {
		return errors.New("event_tables is limited to 8 table names")
	}
	seen := map[string]bool{}
	for _, table := range r.EventTables {
		if !identifier.MatchString(table) || seen[table] {
			return errors.New("event_tables must contain unique simple identifiers")
		}
		seen[table] = true
	}
	return nil
}

func Collect(parent context.Context, db *sql.DB, database string, req Request) (Report, error) {
	if err := req.Validate(); err != nil {
		return Report{}, err
	}
	if !identifier.MatchString(database) {
		return Report{}, errors.New("saved database must be a simple identifier")
	}
	budget := req.TimeoutSeconds
	if budget == 0 {
		budget = 5
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(budget)*time.Second)
	defer cancel()
	start := time.Now()
	report := Report{Database: database, At: start, Status: "ok", Steps: []Step{}}
	run := func(name string, work func() (any, error)) {
		step := Step{Name: name, Status: "ok"}
		at := time.Now()
		err := ctx.Err()
		if err == nil {
			step.Data, err = work()
		} else {
			step.Status = "skipped"
		}
		step.ElapsedMS = float64(time.Since(at).Microseconds()) / 1000
		if err != nil {
			if step.Status != "skipped" {
				step.Status = "error"
			}
			step.Data = nil
			step.ErrorCode = "query_failed"
			if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
				step.ErrorCode = "deadline_exceeded"
			}
			if errors.Is(ctx.Err(), context.Canceled) {
				step.ErrorCode = "canceled"
			}
			report.Status = "partial"
		}
		report.Steps = append(report.Steps, step)
	}
	run("connect", func() (any, error) { return nil, db.PingContext(ctx) })
	run("buffer_pool", func() (any, error) {
		var n int64
		err := db.QueryRowContext(ctx, "SELECT @@innodb_buffer_pool_size").Scan(&n)
		return map[string]int64{"bytes": n}, err
	})
	run("system_tables", func() (any, error) {
		rows, err := db.QueryContext(ctx, "SELECT table_name, COALESCE(table_rows,0), COALESCE(data_length,0), COALESCE(index_length,0) FROM information_schema.tables WHERE table_schema=? AND table_name IN ('sync_event_log','sync_apply_log','sync_ack_log','sync_error_log') ORDER BY table_name", database)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []TableStats{}
		for rows.Next() {
			var row TableStats
			if err := rows.Scan(&row.Table, &row.EstimatedRows, &row.DataBytes, &row.IndexBytes); err != nil {
				return nil, err
			}
			out = append(out, row)
		}
		return out, rows.Err()
	})
	run("system_indexes", func() (any, error) {
		rows, err := db.QueryContext(ctx, "SELECT table_name,index_name,column_name,seq_in_index,sub_part,is_visible FROM information_schema.statistics WHERE table_schema=? AND table_name IN ('sync_event_log','sync_apply_log','sync_ack_log','sync_error_log') ORDER BY table_name,index_name,seq_in_index", database)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []IndexColumn{}
		for rows.Next() {
			var row IndexColumn
			var visible string
			if err := rows.Scan(&row.Table, &row.Index, &row.Column, &row.Position, &row.PrefixLength, &visible); err != nil {
				return nil, err
			}
			row.Visible = visible == "YES"
			out = append(out, row)
		}
		return out, rows.Err()
	})
	if len(req.EventTables) > 0 {
		args := make([]any, len(req.EventTables))
		for i, t := range req.EventTables {
			args[i] = t
		}
		query := "SELECT COUNT(*) FROM sync_event_log WHERE table_name IN (" + strings.TrimSuffix(strings.Repeat("?,", len(args)), ",") + ")"
		failed := query + " AND status='FAILED'"
		run("failed_event_plan", func() (any, error) {
			var raw []byte
			if err := db.QueryRowContext(ctx, "EXPLAIN FORMAT=JSON "+failed, args...).Scan(&raw); err != nil {
				return nil, err
			}
			if len(raw) > 65536 || !json.Valid(raw) {
				return nil, fmt.Errorf("invalid or oversized query plan")
			}
			return json.RawMessage(raw), nil
		})
		for _, entry := range []struct{ name, query string }{{"event_count", query}, {"failed_event_count", failed}} {
			run(entry.name, func() (any, error) {
				var n int64
				err := db.QueryRowContext(ctx, entry.query, args...).Scan(&n)
				return map[string]int64{"count": n}, err
			})
		}
	}
	report.ElapsedMS = float64(time.Since(start).Microseconds()) / 1000
	return report, nil
}
