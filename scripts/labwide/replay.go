package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type replayProof struct{ Origin, Database, Table, PK, Operation string }

func checkReplay(want, got [][]byte, node string, t table, database string, proofs map[string]replayProof) ([][]byte, error) {
	if len(want) != 50 || len(got) != 50 {
		return nil, fmt.Errorf("replay check requires 50 columns")
	}
	if string(want[5]) == node {
		if len(got[6]) != 0 {
			return nil, fmt.Errorf("source-owned pk=%s has replay marker %s", want[0], got[6])
		}
		return got, nil
	}
	p, ok := proofs[string(got[6])]
	if !ok || len(got[6]) == 0 || p.Origin != string(want[5]) || p.Database != database || p.Table != t.Name || (p.Operation != "INSERT" && p.Operation != "UPDATE" && p.Operation != "DELETE") {
		return nil, fmt.Errorf("invalid replay proof pk=%s event=%s", want[0], got[6])
	}
	var pk map[string]any
	d := json.NewDecoder(strings.NewReader(p.PK))
	d.UseNumber()
	if err := d.Decode(&pk); err != nil {
		return nil, err
	}
	if len(pk) != 1 || fmt.Sprint(pk[t.col("id")]) != string(want[0]) || !bytes.Equal(want[5], got[5]) {
		return nil, fmt.Errorf("replay identity mismatch pk=%s", want[0])
	}
	copyRow := append([][]byte(nil), got...)
	copyRow[6] = want[6]
	return copyRow, nil
}

func replayProofs(db *sql.DB, rows [][][]byte) (map[string]replayProof, error) {
	ids := []any{}
	seen := map[string]bool{}
	for _, row := range rows {
		if len(row) == 50 && len(row[6]) != 0 && !seen[string(row[6])] {
			seen[string(row[6])] = true
			ids = append(ids, string(row[6]))
		}
	}
	result := map[string]replayProof{}
	if len(ids) == 0 {
		return result, nil
	}
	r, err := db.Query("SELECT event_id,origin_node_id,target_database_name,target_table_name,pk_value,op_type FROM sync_apply_log WHERE event_id IN ("+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+")", ids...)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	for r.Next() {
		var id string
		var p replayProof
		if err = r.Scan(&id, &p.Origin, &p.Database, &p.Table, &p.PK, &p.Operation); err != nil {
			return nil, err
		}
		if _, ok := result[id]; ok {
			return nil, fmt.Errorf("duplicate apply proof %s", id)
		}
		result[id] = p
	}
	return result, r.Err()
}

func (l *lab) compareApplied(side int, t table, want, actual [][]byte, proofs map[string]replayProof) error {
	row, err := checkReplay(want, actual, l.sides[side].Node, t, l.sides[side].Database, proofs)
	if err != nil {
		return err
	}
	return compareRows(l.s, want, row)
}
