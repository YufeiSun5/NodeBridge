package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func incidentRow(s schema, raw [][]byte) ([]any, error) {
	if len(raw) != len(s.Columns) {
		return nil, fmt.Errorf("incident row width mismatch")
	}
	row := make([]any, len(raw))
	for i, b := range raw {
		if b == nil {
			continue
		}
		if s.Columns[i].Type == "BIGINT" || s.Columns[i].Type == "TINYINT" {
			v, err := strconv.ParseInt(string(b), 10, 64)
			if err != nil {
				return nil, err
			}
			row[i] = v
		} else {
			row[i] = string(b)
		}
	}
	return row, nil
}

func (l *lab) prepareIncidentResume() error {
	var initial [2]progress
	var values [2][][]any
	for side, layout := range l.sides {
		p := &initial[side]
		p.Side = layout.Node
		if err := l.db[side].QueryRow("SELECT COALESCE(SUM(inserted),0),COALESCE(SUM(updated),0),COALESCE(MAX(tick)+1,0) FROM "+l.ledger()).Scan(&p.Inserts, &p.Updates, &p.Tick); err != nil {
			return err
		}
		var count int64
		if err := l.db[side].QueryRow("SELECT COUNT(*) FROM " + quote(layout.Stream.Name) + " WHERE id<8000000").Scan(&count); err != nil {
			return err
		}
		if count != p.Inserts || count < 600000 {
			return fmt.Errorf("incident source does not match committed history side=%d count=%d ledger=%d", side, count, p.Inserts)
		}
		raw, err := readChunk(l.db[side], layout.State, l.s, layout.Offset, 1000)
		if err != nil {
			return err
		}
		if len(raw) != 1000 {
			return fmt.Errorf("incident needs 1000 existing writer keys")
		}
		for i, r := range raw {
			row, err := incidentRow(l.s, r)
			if err != nil {
				return err
			}
			if row[0] != layout.Offset+int64(i)+1 || row[3].(int64) < 1 || row[5] != layout.Node {
				return fmt.Errorf("incident writer history mismatch side=%d slot=%d", side, i)
			}
			values[side] = append(values[side], row)
		}
	}
	l.seed = func(side int) (progress, [][]any, error) { return initial[side], values[side], nil }
	return atomicJSON(filepath.Join(l.root, "resume-baseline.json"), map[string]any{"at": time.Now(), "progress": initial, "rows": values, "scope": "existing physical tables; original failure backlog retained separately; continued write verification only"})
}

func TestIncidentRowPreservesPrecision(t *testing.T) {
	s := schema{Columns: []column{{Type: "BIGINT"}, {Type: "DECIMAL(18,6)"}, {Type: "TEXT"}}}
	row, err := incidentRow(s, [][]byte{[]byte("9007199254740993"), []byte("999999999999.123456"), nil})
	if err != nil || row[0] != int64(9007199254740993) || row[1] != "999999999999.123456" || row[2] != nil {
		t.Fatalf("row=%v err=%v", row, err)
	}
}

func (l *lab) verifyIncidentResume() error {
	for side, layout := range l.sides {
		base, _, err := l.seed(side)
		if err != nil {
			return err
		}
		var inserted, updated, ticks int64
		if err := l.db[side].QueryRow("SELECT SUM(inserted),SUM(updated),COUNT(*) FROM "+l.ledger()+" WHERE tick>=?", base.Tick).Scan(&inserted, &updated, &ticks); err != nil {
			return err
		}
		wantTicks := int64(l.c.DurationSeconds * 390 / 420)
		if ticks != wantTicks || inserted != wantTicks*80 || updated != wantTicks*40 {
			return fmt.Errorf("incident new ledger incomplete side=%d ticks=%d inserts=%d updates=%d", side, ticks, inserted, updated)
		}
		last := base.Inserts
		remaining := inserted
		for remaining > 0 {
			n := int64(1000)
			if remaining < n {
				n = remaining
			}
			a, err := readChunk(l.db[side], layout.Stream, l.s, last, int(n))
			if err != nil {
				return err
			}
			b, err := readChunk(l.db[1-side], l.sides[1-side].Target, l.s, last, int(n))
			if err != nil {
				return err
			}
			if len(a) != int(n) || len(b) != int(n) {
				return fmt.Errorf("incident new append lag side=%d at=%d", side, last)
			}
			proofs, err := replayProofs(l.db[1-side], b)
			if err != nil {
				return err
			}
			for i, r := range a {
				if err := l.compareApplied(1-side, l.sides[1-side].Target, r, b[i], proofs); err != nil {
					return err
				}
				last, err = strconv.ParseInt(string(r[0]), 10, 64)
				if err != nil {
					return err
				}
			}
			remaining -= n
		}
		a, err := readChunk(l.db[side], layout.State, l.s, layout.Offset, 1000)
		if err != nil {
			return err
		}
		b, err := readChunk(l.db[1-side], l.sides[1-side].State, l.s, layout.Offset, 1000)
		if err != nil {
			return err
		}
		if len(a) != 1000 || len(b) != 1000 {
			return fmt.Errorf("incident state keys missing")
		}
		proofs, err := replayProofs(l.db[1-side], b)
		if err != nil {
			return err
		}
		for i, r := range a {
			if err := l.compareApplied(1-side, l.sides[1-side].State, r, b[i], proofs); err != nil {
				return err
			}
		}
	}
	return atomicJSON(filepath.Join(l.root, "continued-consistency.json"), map[string]any{"at": time.Now(), "passed": true, "columns": 50, "scope": "new append range, 2000 writer keys, new commit ledger; not original missing events or seven-hour acceptance"})
}
