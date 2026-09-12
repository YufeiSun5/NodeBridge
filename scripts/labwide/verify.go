package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func readChunk(db *sql.DB, t table, s schema, after int64, limit int) ([][][]byte, error) {
	rows, err := db.Query("SELECT "+t.names(s)+" FROM "+quote(t.Name)+" WHERE "+quote(t.col("id"))+">? ORDER BY "+quote(t.col("id"))+" LIMIT ?", after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([][][]byte, 0, limit)
	for rows.Next() {
		values := make([][]byte, len(s.Columns))
		dest := make([]any, len(values))
		for i := range values {
			dest[i] = &values[i]
		}
		if err = rows.Scan(dest...); err != nil {
			return nil, err
		}
		result = append(result, values)
	}
	return result, rows.Err()
}
func rawRow(values []any) [][]byte {
	raw := make([][]byte, len(values))
	for i, v := range values {
		if v != nil {
			raw[i] = []byte(fmt.Sprint(v))
		}
	}
	return raw
}
func compareRows(s schema, want, got [][]byte) error {
	if len(want) != 50 || len(got) != 50 {
		return fmt.Errorf("not a 50-column row")
	}
	for i := range want {
		if (want[i] == nil) != (got[i] == nil) || !bytes.Equal(want[i], got[i]) {
			return fmt.Errorf("pk=%s column=%s expected=%q actual=%q", want[0], s.Columns[i].Name, want[i], got[i])
		}
	}
	return nil
}

type comparison struct {
	Source  string   `json:"source"`
	Target  string   `json:"target"`
	Rows    int64    `json:"rows"`
	Columns int      `json:"columns"`
	SHA256  string   `json:"sha256"`
	Chunks  []string `json:"chunks"`
}

func (l *lab) verify() error {
	results := []comparison{}
	for side, source := range l.sides {
		target := l.sides[1-side].Target
		c := comparison{Source: source.Stream.Name, Target: target.Name, Columns: 50}
		h := sha256.New()
		var last int64
		for {
			a, err := readChunk(l.db[side], source.Stream, l.s, last, 1000)
			if err != nil {
				return err
			}
			b, err := readChunk(l.db[1-side], target, l.s, last, 1000)
			if err != nil {
				return err
			}
			if len(a) != len(b) {
				return fmt.Errorf("%s -> %s chunk at %d has %d/%d rows", source.Stream.Name, target.Name, last, len(a), len(b))
			}
			if len(a) == 0 {
				break
			}
			proofs, err := replayProofs(l.db[1-side], b)
			if err != nil {
				return err
			}
			for i, row := range a {
				id, err := strconv.ParseInt(string(row[0]), 10, 64)
				if err != nil {
					return err
				}
				expected := rawRow(makeRow(l.c, source, id, 1))
				if err = compareRows(l.s, expected, row); err != nil {
					return fmt.Errorf("source oracle: %w", err)
				}
				if err = l.compareApplied(1-side, target, row, b[i], proofs); err != nil {
					return err
				}
				addCanonical(h, l.s, row)
				last = id
				c.Rows++
			}
			c.Chunks = append(c.Chunks, digest(l.s, a))
		}
		c.SHA256 = hex.EncodeToString(h.Sum(nil))
		results = append(results, c)
		var committed, updated, ticks, actual int64
		if err := l.db[side].QueryRow("SELECT COALESCE(SUM(inserted),0),COALESCE(SUM(updated),0),COUNT(*) FROM "+l.ledger()).Scan(&committed, &updated, &ticks); err != nil {
			return err
		}
		if err := l.db[side].QueryRow("SELECT COUNT(*) FROM " + quote(source.Stream.Name) + " WHERE id<8000000").Scan(&actual); err != nil {
			return err
		}
		if actual != committed {
			return fmt.Errorf("side %d insert ledger %d != source %d", side, committed, actual)
		}
		if l.c.Start != "" {
			var expectedI, expectedU int64
			generation := l.c.DurationSeconds * 390 / 420
			for tick := 0; tick < generation; tick++ {
				r := l.rateAt(float64(tick) * 420 / float64(l.c.DurationSeconds))
				expectedI += int64(r.I)
				expectedU += int64(r.U)
			}
			if ticks != int64(generation) || committed != expectedI || updated != expectedU {
				return fmt.Errorf("incomplete ledger side=%d ticks=%d I=%d/%d U=%d/%d", side, ticks, committed, expectedI, updated, expectedU)
			}
		}
	}
	// Each writer's oracle is committed in the same transaction as the business UPDATE.
	oracle := map[int64][][]byte{}
	for owner := range l.db {
		rows, err := l.db[owner].Query("SELECT row_json FROM " + l.oracle() + " ORDER BY id")
		if err != nil {
			return err
		}
		for rows.Next() {
			var b []byte
			if err = rows.Scan(&b); err != nil {
				rows.Close()
				return err
			}
			var expected []any
			decoder := json.NewDecoder(bytes.NewReader(b))
			decoder.UseNumber()
			if err = decoder.Decode(&expected); err != nil {
				rows.Close()
				return err
			}
			id, err := strconv.ParseInt(fmt.Sprint(expected[0]), 10, 64)
			if err != nil {
				rows.Close()
				return err
			}
			if _, exists := oracle[id]; exists {
				rows.Close()
				return fmt.Errorf("overlapping writer oracle key %d", id)
			}
			oracle[id] = rawRow(expected)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	for side := range l.db {
		var last int64
		count := 0
		for {
			chunk, err := readChunk(l.db[side], l.sides[side].State, l.s, last, 1000)
			if err != nil {
				return err
			}
			if len(chunk) == 0 {
				break
			}
			proofs, err := replayProofs(l.db[side], chunk)
			if err != nil {
				return err
			}
			for _, row := range chunk {
				id, err := strconv.ParseInt(string(row[0]), 10, 64)
				if err != nil {
					return err
				}
				if err = l.compareApplied(side, l.sides[side].State, oracle[id], row, proofs); err != nil {
					return fmt.Errorf("state oracle side %d: %w", side, err)
				}
				last = id
				count++
			}
		}
		if count != len(oracle) {
			return fmt.Errorf("unexpected state count side %d: %d/%d", side, count, len(oracle))
		}
	}
	filename := "consistency.json"
	if l.c.Start == "" {
		filename = "bootstrap-consistency.json"
	} else {
		if err := l.verifyLogs(); err != nil {
			return err
		}
	}
	return atomicJSON(filepath.Join(l.root, filename), map[string]any{"at": time.Now(), "passed": true, "append": results, "state_oracle": "transactional, both copies, all 50 columns", "columns": 50})
}
func (l *lab) getRow(side int, t table, id any) ([][]byte, error) {
	values := make([][]byte, 50)
	dest := make([]any, 50)
	for i := range values {
		dest[i] = &values[i]
	}
	err := l.db[side].QueryRow("SELECT "+t.names(l.s)+" FROM "+quote(t.Name)+" WHERE "+quote(t.col("id"))+"=?", id).Scan(dest...)
	return values, err
}
func until(description string, fn func() (bool, error)) error {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		ok, err := fn()
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if ok {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", description)
}
func (l *lab) awaitRow(side int, t table, row []any) error {
	var lastComparison error
	err := until("row "+t.Name, func() (bool, error) {
		actual, err := l.getRow(side, t, row[0])
		if err != nil {
			return false, err
		}
		proofs, err := replayProofs(l.db[side], [][][]byte{actual})
		if err != nil {
			return false, err
		}
		lastComparison = l.compareApplied(side, t, rawRow(row), actual, proofs)
		return lastComparison == nil, nil
	})
	if err != nil && lastComparison != nil {
		return fmt.Errorf("%w; last comparison: %v", err, lastComparison)
	}
	return err
}
func (l *lab) lifecycle() error {
	idBase := int64(10000000) + time.Now().Unix()%100000
	for side, layout := range l.sides {
		step := func(name string, work func() error) error {
			return snapshotStep(os.Stderr, layout.Node, fmt.Sprintf("lifecycle.%d.%s", idBase+layout.Offset, name), work)
		}
		row := makeRow(l.c, layout, idBase+layout.Offset, 1)
		err := step("insert-1", func() error { return l.commitLifecycleRow(side, layout.State, row, true, false) })
		if err != nil {
			return err
		}
		if err = step("await-insert-1", func() error { return l.awaitRow(1-side, l.sides[1-side].State, row) }); err != nil {
			return err
		}
		for revision := int64(2); revision <= 4; revision++ {
			row = makeRow(l.c, layout, idBase+layout.Offset, revision)
			err = step(fmt.Sprintf("update-%d", revision), func() error { return l.commitLifecycleRow(side, layout.State, row, false, false) })
			if err != nil {
				return err
			}
			if err = step(fmt.Sprintf("await-update-%d", revision), func() error { return l.awaitRow(1-side, l.sides[1-side].State, row) }); err != nil {
				return err
			}
		}
		if err = step("delete", func() error {
			_, err := l.db[side].Exec("DELETE FROM "+quote(layout.State.Name)+" WHERE "+quote(layout.State.col("id"))+"=?", row[0])
			return err
		}); err != nil {
			return err
		}
		err = step("await-delete", func() error {
			return until("soft delete", func() (bool, error) {
				r, e := l.getRow(1-side, l.sides[1-side].State, row[0])
				if e != nil {
					return false, e
				}
				return string(r[9]) == "1" && r[10] != nil && string(r[11]) == layout.Node && len(r[6]) > 0, nil
			})
		})
		if err != nil {
			return err
		}
		row = makeRow(l.c, layout, idBase+layout.Offset, 5)
		err = step("reinsert-5", func() error { return l.commitLifecycleRow(side, layout.State, row, true, true) })
		if err != nil {
			return err
		}
		if err = step("await-reinsert-5", func() error { return l.awaitRow(1-side, l.sides[1-side].State, row) }); err != nil {
			return err
		}
	}
	return atomicJSON(filepath.Join(l.root, fmt.Sprintf("lifecycle-%d.json", idBase)), map[string]any{"passed": true, "base": idBase, "per_side": map[string]int{"INSERT": 2, "UPDATE": 3, "DELETE": 1}})
}

func (l *lab) commitLifecycleRow(side int, target table, row []any, insert, oracle bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := l.db[side].BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if insert {
		err = l.insert(ctx, tx, target, [][]any{row})
	} else {
		err = l.update(ctx, tx, target, row, append([]int{3, 4, 5, 6, 8}, updateIndices(10)...))
	}
	if err == nil && oracle {
		err = l.saveOracle(ctx, tx, row)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (l *lab) ddl(add bool) error {
	for side, layout := range l.sides {
		clause := "DROP COLUMN lab_extra"
		want := 0
		if add {
			clause = "ADD COLUMN lab_extra VARCHAR(64) NULL"
			want = 1
		}
		if _, err := l.db[side].Exec("ALTER TABLE " + quote(layout.Stream.Name) + " " + clause); err != nil {
			return err
		}
		target := l.sides[1-side].Target
		err := until("DDL "+target.Name, func() (bool, error) {
			var n int
			err := l.db[1-side].QueryRow("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='lab_extra'", target.Name).Scan(&n)
			return n == want, err
		})
		if err != nil {
			return err
		}
		if add {
			row := makeRow(l.c, layout, 8000000+layout.Offset, 1)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			tx, err := l.db[side].BeginTx(ctx, nil)
			if err != nil {
				cancel()
				return err
			}
			q := insertSQL(layout.Stream, l.s, 1)
			// Insert the added value in the same statement so append-only remains INSERT-only.
			end := len(q) - 1
			q = q[:end] + ",?)"
			columnEnd := len("INSERT INTO " + quote(layout.Stream.Name) + " (" + layout.Stream.names(l.s))
			q = q[:columnEnd] + ",lab_extra" + q[columnEnd:]
			_, err = tx.ExecContext(ctx, q, append(row, "ddl-"+layout.Node)...)
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			cancel()
			if err != nil {
				return err
			}
			err = until("DDL value", func() (bool, error) {
				var note string
				e := l.db[1-side].QueryRow("SELECT lab_extra FROM "+quote(target.Name)+" WHERE "+quote(target.col("id"))+"=?", row[0]).Scan(&note)
				return note == "ddl-"+layout.Node, e
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
