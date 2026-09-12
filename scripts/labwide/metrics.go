package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

func (l *lab) metrics() error {
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	result := map[string]any{"at": time.Now()}
	tables := []any{l.sides[0].Stream.Name, l.sides[1].Stream.Name, l.sides[0].State.Name, l.sides[1].State.Name}
	for side, layout := range l.sides {
		d := map[string]any{}
		apply := map[string]int64{}
		err := metricsRows(ctx, os.Stderr, l.db[side], layout.Node, "apply_counts", "SELECT table_name,op_type,COUNT(*) FROM sync_apply_log WHERE table_name IN (?,?,?,?) GROUP BY table_name,op_type", tables, func(rows *sql.Rows) error {
			var table, op string
			var n int64
			if err := rows.Scan(&table, &op, &n); err != nil {
				return err
			}
			apply[table+"/"+op] = n
			return nil
		})
		if err != nil {
			return err
		}
		d["apply_counts"] = apply
		sizes := map[string]any{}
		err = metricsRows(ctx, os.Stderr, l.db[side], layout.Node, "table_estimates", "SELECT table_name,COALESCE(data_length,0)+COALESCE(index_length,0),COALESCE(table_rows,0) FROM information_schema.tables WHERE table_schema=DATABASE() AND (table_name IN ('sync_apply_log','sync_event_log','sync_ack_log','sync_error_log') OR LEFT(table_name,?)=?)", []any{len(l.c.Prefix), l.c.Prefix}, func(rows *sql.Rows) error {
			var name string
			var size, n int64
			if err := rows.Scan(&name, &size, &n); err != nil {
				return err
			}
			sizes[name] = map[string]int64{"allocated_bytes_estimate": size, "rows_estimate": n}
			return nil
		})
		if err != nil {
			return err
		}
		d["table_estimates"] = sizes
		status := map[string]string{}
		err = metricsRows(ctx, os.Stderr, l.db[side], layout.Node, "mysql_cumulative_io", "SHOW GLOBAL STATUS WHERE Variable_name IN ('Innodb_os_log_written','Innodb_data_read','Innodb_data_written','Bytes_received','Bytes_sent','Questions','Innodb_buffer_pool_reads','Innodb_buffer_pool_read_requests','Innodb_buffer_pool_pages_free','Innodb_buffer_pool_pages_dirty','Innodb_buffer_pool_wait_free','Innodb_data_fsyncs','Innodb_data_pending_fsyncs','Innodb_data_pending_reads','Innodb_data_pending_writes','Innodb_log_waits','Threads_running')", nil, func(rows *sql.Rows) error {
			var name, value string
			if err := rows.Scan(&name, &value); err != nil {
				return err
			}
			status[name] = value
			return nil
		})
		if err != nil {
			return err
		}
		d["mysql_cumulative_io"] = status
		offsets := []any{}
		err = metricsRows(ctx, os.Stderr, l.db[side], layout.Node, "canal_offsets", "SELECT reader_name,COALESCE(binlog_file,''),COALESCE(binlog_pos,0),updated_at FROM sync_upload_offset", nil, func(rows *sql.Rows) error {
			var reader, file, at string
			var pos int64
			if err := rows.Scan(&reader, &file, &pos, &at); err != nil {
				return err
			}
			offsets = append(offsets, map[string]any{"reader": reader, "file": file, "position": pos, "at": at})
			return nil
		})
		if err != nil {
			return err
		}
		d["canal_offsets"] = offsets
		if side == 1 {
			events := map[string]int64{}
			err = metricsRows(ctx, os.Stderr, l.db[side], layout.Node, "server_event_counts", "SELECT table_name,status,COUNT(*) FROM sync_event_log WHERE table_name IN (?,?,?,?) GROUP BY table_name,status", tables, func(rows *sql.Rows) error {
				var table, status string
				var n int64
				if err := rows.Scan(&table, &status, &n); err != nil {
					return err
				}
				events[table+"/"+status] = n
				return nil
			})
			if err != nil {
				return err
			}
			d["server_event_counts"] = events
		}
		result[layout.Node] = d
	}
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func metricsRows(ctx context.Context, out io.Writer, db *sql.DB, node, operation, query string, args []any, scan func(*sql.Rows) error) error {
	return snapshotStep(out, node, "metrics."+operation, func() error {
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := scan(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	})
}
