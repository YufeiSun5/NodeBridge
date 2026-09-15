//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func runOwnedMultiSoak(t *testing.T, ctx context.Context, root string, duration time.Duration, dbs []*sql.DB, tables, keys, values []string, execute func(int, string, ...any), start func(int) func(), stops []func()) {
	t.Helper()
	began := time.Now()
	end := began.Add(duration)
	batches, operations := 0, 0
	state := map[string]any{"status": "running", "started_at": began, "planned_end_at": end, "duration_seconds": int(duration.Seconds()), "faults": []any{}, "scope": "three isolated MySQL/Canal instances and Windows Agents; synthetic increasing source timestamps"}
	publish := func() {
		state["heartbeat_at"] = time.Now()
		state["batches"], state["operations"] = batches, operations
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "soak-progress.json")
		if err := os.WriteFile(path+".tmp", data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".tmp", path); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if t.Failed() {
			state["status"] = "failed"
			publish()
		}
	})
	baseline := 0
	if err := dbs[0].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tables[0]).Scan(&baseline); err != nil {
		t.Fatal(err)
	}
	snapshot := func(i int) (int, string, error) {
		rows, err := dbs[i].QueryContext(ctx, "SELECT "+keys[i]+","+values[i]+",HEX(payload),note FROM "+tables[i]+" ORDER BY "+keys[i])
		if err != nil {
			return 0, "", err
		}
		defer rows.Close()
		h := sha256.New()
		n := 0
		for rows.Next() {
			var key, decimal string
			var payload, note sql.NullString
			if err := rows.Scan(&key, &decimal, &payload, &note); err != nil {
				return 0, "", err
			}
			data, err := json.Marshal([]any{key, decimal, payload, note})
			if err != nil {
				return 0, "", err
			}
			_, _ = h.Write(data)
			n++
		}
		return n, hex.EncodeToString(h.Sum(nil)), rows.Err()
	}
	verify := func(expected int, phase string) {
		deadline := time.Now().Add(90 * time.Second)
		var observations []any
		for {
			observations = nil
			good := true
			first := ""
			for i := range dbs {
				n, digest, err := snapshot(i)
				observations = append(observations, map[string]any{"node": i, "rows": n, "sha256": digest, "error": fmt.Sprint(err)})
				if i == 0 {
					first = digest
				}
				if err != nil || n != expected || digest != first {
					good = false
				}
			}
			if good {
				state["last_verified_at"] = time.Now()
				state["last_phase"] = phase
				state["snapshots"] = observations
				publish()
				return
			}
			if time.Now().After(deadline) {
				state["mismatch"] = observations
				publish()
				t.Fatalf("soak convergence failed: %s %+v", phase, observations)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	publish()
	verify(baseline, "baseline")
	faultNumber := 0
	for time.Now().Before(end) {
		if _, err := os.Stat(filepath.Join(root, "stop.requested")); err == nil {
			t.Fatal("operator stopped owned soak")
		}
		batches++
		base := 1000000 + batches*100
		offline := -1
		if faultNumber < 2 && time.Since(began) >= time.Duration(faultNumber+1)*2*time.Hour {
			offline = []int{2, 1}[faultNumber]
			stops[offline]()
			state["offline_node"] = offline
			state["phase"] = "planned_offline_writes"
			publish()
		}
		for i := range dbs {
			for row := 0; row < 12; row++ {
				key := base + i*12 + row
				var payload any = []byte{0, 128, 255, byte(row)}
				var note any = fmt.Sprintf("中文-%d-%d", batches, row)
				if row%4 == 0 {
					payload = nil
					note = nil
				}
				execute(i, "INSERT INTO "+tables[i]+" ("+keys[i]+","+values[i]+",payload,note) VALUES (?,'12345678901234567890.1234567890',?,?)", key, payload, note)
				operations++
			}
			tx, err := dbs[i].BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO "+tables[i]+" ("+keys[i]+","+values[i]+",note) VALUES (?,1,'must-rollback')", base+80+i); err != nil {
				_ = tx.Rollback()
				t.Fatal(err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
		}
		if offline >= 0 {
			until := time.Now().Add(10 * time.Minute)
			for time.Now().Before(until) {
				publish()
				time.Sleep(5 * time.Second)
			}
			stops[offline] = start(offline)
			faultNumber++
			state["faults_completed"] = faultNumber
			delete(state, "offline_node")
		}
		expected := baseline + 36
		if batches > 1 {
			expected += 24
		}
		verify(expected, "insert and rollback")
		// Check exact values independently of cross-node equality.
		for i := range dbs {
			for row := 0; row < 36; row++ {
				var decimal string
				var payload, note sql.NullString
				err := dbs[i].QueryRowContext(ctx, "SELECT "+values[i]+",HEX(payload),note FROM "+tables[i]+" WHERE "+keys[i]+"=?", base+row).Scan(&decimal, &payload, &note)
				localRow := row % 12
				valid := decimal == "12345678901234567890.1234567890"
				if localRow%4 == 0 {
					valid = valid && !payload.Valid && !note.Valid
				} else {
					valid = valid && payload.String == fmt.Sprintf("0080FF%02X", localRow) && note.String == fmt.Sprintf("中文-%d-%d", batches, localRow)
				}
				if err != nil || !valid {
					t.Fatalf("soak exact value mismatch node=%d key=%d: %v", i, base+row, err)
				}
			}
		}
		for origin := range dbs {
			writer := (origin + 1) % 3
			execute(writer, "UPDATE "+tables[writer]+" SET note=?,"+values[writer]+"=-9876543210.1234567890 WHERE "+keys[writer]+">=? AND "+keys[writer]+"<?", fmt.Sprintf("跨节点更新-%d", batches), base+origin*12, base+(origin+1)*12)
			operations += 12
		}
		verify(expected, "cross-origin update")
		for origin := range dbs {
			writer := (origin + 2) % 3
			execute(writer, "DELETE FROM "+tables[writer]+" WHERE "+keys[writer]+">=? AND "+keys[writer]+"<?", base+origin*12, base+origin*12+4)
			operations += 4
		}
		verify(expected-12, "cross-origin hard delete")
		if batches > 1 {
			writer := batches % 3
			execute(writer, "DELETE FROM "+tables[writer]+" WHERE "+keys[writer]+">=? AND "+keys[writer]+"<?", base-100, base)
			operations += 24
		}
		verify(baseline+24, "bounded retention")
		for i := range dbs {
			var pending int
			if err := dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_conflict_state WHERE repair_required=1").Scan(&pending); err != nil || pending != 0 {
				t.Fatalf("soak repair pending node=%d count=%d err=%v", i, pending, err)
			}
		}
		state["phase"] = "verified_running"
		publish()
		t.Logf("SOAK batch=%d operations=%d elapsed=%s all three digests equal", batches, operations, time.Since(began).Round(time.Second))
		time.Sleep(5 * time.Second)
	}
	state["status"] = "workload_completed"
	state["finished_at"] = time.Now()
	publish()
}
