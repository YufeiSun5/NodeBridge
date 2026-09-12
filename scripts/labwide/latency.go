package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

func (l *lab) latency() error {
	samples := map[string]any{}
	phases := map[string]string{}
	committed := map[string]time.Time{}
	testStart, err := time.Parse(time.RFC3339Nano, l.c.Start)
	if err != nil {
		return err
	}
	base := int64(3000000000000) + time.Now().UnixMilli()
	for side, layout := range l.sides {
		row := makeRow(l.c, layout, base+layout.Offset, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		tx, err := l.db[side].BeginTx(ctx, nil)
		if err != nil {
			cancel()
			return err
		}
		err = l.insert(ctx, tx, layout.State, [][]any{row})
		if err == nil {
			err = l.saveOracle(ctx, tx, row)
		}
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		cancel()
		if err != nil {
			return err
		}
		start := time.Now()
		committed[layout.Node] = start.UTC()
		phases[layout.Node] = latencyPhase(start.Sub(testStart).Seconds(), l.c.DurationSeconds)
		if err = l.awaitRow(1-side, l.sides[1-side].State, row); err != nil {
			return err
		}
		samples[layout.Node] = float64(time.Since(start).Microseconds()) / 1000
	}
	samples["phases"] = phases
	samples["committed_at"] = committed
	return atomicJSON(filepath.Join(l.root, fmt.Sprintf("latency-%d.json", base)), samples)
}

func latencyPhase(elapsed float64, duration int) string {
	minute := elapsed * 420 / float64(duration)
	if minute >= 270 && minute < 300 {
		return "peak"
	}
	if minute >= 300 && minute < 330 {
		return "update_heavy"
	}
	return "steady"
}

// Transfer ownership only after the prior write is visible, never by competing writes.
func (l *lab) handoff(id int64) error {
	for step, side := range []int{0, 1, 0} {
		row := makeRow(l.c, l.sides[side], id, int64(step+1))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		tx, err := l.db[side].BeginTx(ctx, nil)
		if err != nil {
			cancel()
			return err
		}
		if step == 0 {
			err = l.insert(ctx, tx, l.sides[side].State, [][]any{row})
		} else {
			indices := []int{3, 4, 5, 6, 8}
			indices = append(indices, updateIndices(10)...)
			err = l.update(ctx, tx, l.sides[side].State, row, indices)
		}
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		cancel()
		if err != nil {
			return err
		}
		if err = l.awaitRow(1-side, l.sides[1-side].State, row); err != nil {
			return err
		}
		if step == 2 {
			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			tx, err = l.db[side].BeginTx(ctx, nil)
			if err != nil {
				cancel()
				return err
			}
			err = l.saveOracle(ctx, tx, row)
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			cancel()
			if err != nil {
				return err
			}
		}
	}
	return atomicJSON(filepath.Join(l.root, fmt.Sprintf("ownership-handoff-%d.json", id)), map[string]any{"passed": true, "sequence": []string{"edge-001", "server-001", "edge-001"}, "revision": 3})
}
