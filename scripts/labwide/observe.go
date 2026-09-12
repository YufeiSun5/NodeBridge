package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

type revisionTracker map[string]int64

func (r revisionTracker) observe(key string, revision int64) error {
	if prior, ok := r[key]; ok && revision < prior {
		return fmt.Errorf("revision rollback %s: %d -> %d", key, prior, revision)
	}
	r[key] = revision
	return nil
}

func (l *lab) observe(ctx context.Context) (returnErr error) {
	tracker := revisionTracker{}
	var observations int64
	write := func() {
		report := map[string]any{"at": time.Now(), "observations": observations, "revisions": tracker, "passed": returnErr == nil}
		if returnErr != nil {
			report["error"] = returnErr.Error()
		}
		_ = atomicJSON(filepath.Join(l.root, "version-observations.json"), report)
	}
	defer write()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	lastWrite := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		for side, layout := range l.sides {
			queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			rows, err := l.db[side].QueryContext(queryCtx, "SELECT "+quote(layout.State.col("id"))+",revision,updated_by_node,last_event_id FROM "+quote(layout.State.Name)+" WHERE "+quote(layout.State.col("id"))+" IN (1,1000001)")
			if err != nil {
				cancel()
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
			for rows.Next() {
				var id, revision int64
				var owner, marker string
				if err = rows.Scan(&id, &revision, &owner, &marker); err != nil {
					break
				}
				expectedOwner := "edge-001"
				if id == 1000001 {
					expectedOwner = "server-001"
				}
				if owner != expectedOwner || (owner == layout.Node && marker != "") || (owner != layout.Node && marker == "") {
					err = fmt.Errorf("hot key replay ownership changed side=%s id=%d owner=%s marker=%s", layout.Node, id, owner, marker)
					break
				}
				err = tracker.observe(fmt.Sprintf("%s/%d", layout.Node, id), revision)
				if err != nil {
					break
				}
				observations++
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
		}
		if time.Since(lastWrite) >= 5*time.Second {
			write()
			lastWrite = time.Now()
		}
	}
}
