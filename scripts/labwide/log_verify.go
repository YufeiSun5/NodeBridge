package main

import (
	"fmt"
	"path/filepath"
	"time"
)

func (l *lab) verifyLogs() error {
	lifecycles, err := filepath.Glob(filepath.Join(l.root, "lifecycle-*.json"))
	if err != nil {
		return err
	}
	latencies, err := filepath.Glob(filepath.Join(l.root, "latency-*.json"))
	if err != nil {
		return err
	}
	handoffs, err := filepath.Glob(filepath.Join(l.root, "ownership-handoff-*.json"))
	if err != nil {
		return err
	}
	if len(lifecycles) < 2 || len(handoffs) != 2 || len(latencies) < 5 {
		return fmt.Errorf("incomplete scenario receipts: lifecycle=%d handoff=%d latency=%d", len(lifecycles), len(handoffs), len(latencies))
	}
	evidence := map[string]any{"at": time.Now(), "passed": false}
	for side, source := range l.sides {
		var inserts, updates int64
		if err = l.db[side].QueryRow("SELECT SUM(inserted),SUM(updated) FROM "+l.ledger()).Scan(&inserts, &updates); err != nil {
			return err
		}
		stateI := int64(1000 + 2*len(lifecycles) + len(latencies))
		if side == 0 {
			stateI += int64(len(handoffs))
		}
		expected := map[string]map[string]int64{source.Stream.Name: {"INSERT": inserts + 1, "ADD_COLUMN": 1, "DROP_COLUMN": 1}, source.State.Name: {"INSERT": stateI, "UPDATE": updates + int64(3*len(lifecycles)+len(handoffs)), "DELETE": int64(len(lifecycles))}}
		for table, operations := range expected {
			var expectedTotal int64
			for op, want := range operations {
				var count int64
				if err = l.db[1-side].QueryRow("SELECT COUNT(*) FROM sync_apply_log WHERE table_name=? AND origin_node_id=? AND op_type=?", table, source.Node, op).Scan(&count); err != nil {
					return err
				}
				if count != want {
					return fmt.Errorf("apply event ledger %s/%s expected=%d actual=%d", table, op, want, count)
				}
				expectedTotal += want
			}
			var all, success int64
			if err = l.db[1].QueryRow("SELECT COUNT(*),COALESCE(SUM(status='SUCCESS'),0) FROM sync_event_log WHERE table_name=? AND origin_node_id=?", table, source.Node).Scan(&all, &success); err != nil {
				return err
			}
			if all != expectedTotal || success != expectedTotal {
				return fmt.Errorf("server event ledger %s expected=%d all=%d success=%d", table, expectedTotal, all, success)
			}
		}
		evidence[source.Node] = expected
	}
	evidence["passed"] = true
	return atomicJSON(filepath.Join(l.root, "event-ledger-consistency.json"), evidence)
}
