package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type snapshotTiming struct {
	At        time.Time `json:"at"`
	Node      string    `json:"node"`
	Operation string    `json:"operation"`
	Phase     string    `json:"phase"`
	ElapsedMS float64   `json:"elapsed_ms"`
	Failed    bool      `json:"failed"`
}

func snapshotStep(w io.Writer, node, operation string, run func() error) error {
	start := time.Now()
	enc := json.NewEncoder(w)
	if err := enc.Encode(snapshotTiming{At: start, Node: node, Operation: operation, Phase: "start"}); err != nil {
		return err
	}
	err := run()
	writeErr := enc.Encode(snapshotTiming{At: time.Now(), Node: node, Operation: operation, Phase: "end", ElapsedMS: float64(time.Since(start).Microseconds()) / 1000, Failed: err != nil})
	if err != nil {
		return fmt.Errorf("snapshot %s %s: %w", node, operation, err)
	}
	return writeErr
}

func snapshotQuery(ctx context.Context, w io.Writer, db *sql.DB, node, operation, query string, args []any, dest ...any) error {
	return snapshotStep(w, node, operation, func() error {
		return db.QueryRowContext(ctx, query, args...).Scan(dest...)
	})
}
