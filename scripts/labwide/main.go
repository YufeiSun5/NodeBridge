package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type lab struct {
	c     settings
	s     schema
	root  string
	db    [2]*sql.DB
	sides [2]layout
	rates func(float64) rate
	seed  func(int) (progress, [][]any, error)
}

func (l *lab) rateAt(minute float64) rate {
	if l.rates != nil {
		return l.rates(minute)
	}
	return rateAt(minute)
}

type progress struct {
	At         time.Time   `json:"at"`
	Side       string      `json:"side"`
	Inserts    int64       `json:"inserts"`
	Updates    int64       `json:"updates"`
	Tick       int         `json:"tick"`
	LatenessMS int64       `json:"lateness_ms"`
	Done       bool        `json:"done"`
	Error      string      `json:"error,omitempty"`
	Coverage   [50]int64   `json:"update_coverage"`
	Data       rowCoverage `json:"field_coverage"`
}

func main() {
	config := flag.String("config", "", "run configuration")
	action := flag.String("action", "snapshot", "init, bootstrap, run, snapshot, verify, lifecycle, ddl-add, ddl-drop")
	flag.Parse()
	if err := execute(*config, *action); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func execute(path, action string) error {
	c, s, err := readSettings(path)
	if err != nil {
		return err
	}
	l := lab{c: c, s: s, root: filepath.Dir(path), sides: layouts(c.Prefix)}
	snapshotCtx, cancelSnapshot := context.WithTimeout(context.Background(), 14*time.Second)
	defer cancelSnapshot()
	if action == "quarantine" {
		return l.quarantine()
	}
	for i, dsn := range []string{c.EdgeDSN, c.ServerDSN} {
		l.db[i], err = sql.Open("mysql", dsn)
		if err != nil {
			return err
		}
		defer l.db[i].Close()
		l.db[i].SetMaxOpenConns(4)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if action == "snapshot" {
			err = snapshotStep(os.Stderr, l.sides[i].Node, "connect", func() error { return l.db[i].PingContext(snapshotCtx) })
		} else {
			err = l.db[i].PingContext(ctx)
		}
		cancel()
		if err != nil {
			return fmt.Errorf("side %d: %w", i, err)
		}
	}
	switch action {
	case "init":
		return l.init()
	case "bootstrap":
		return l.bootstrap()
	case "run":
		return l.run()
	case "snapshot":
		return l.snapshot(snapshotCtx)
	case "metrics":
		return l.metrics()
	case "verify":
		return l.verify()
	case "probe":
		return l.probe()
	case "late-probe":
		l.c.Prefix += "_late"
		l.c.RunID += "_late"
		l.root = filepath.Join(l.root, "late")
		if err := os.MkdirAll(l.root, 0700); err != nil {
			return err
		}
		return l.probe()
	case "boundary":
		l.c.Prefix += "_bnd"
		l.c.RunID += "_bnd"
		l.root = filepath.Join(l.root, "boundary")
		if err := os.MkdirAll(l.root, 0700); err != nil {
			return err
		}
		return l.probe()
	case "lifecycle":
		return l.lifecycle()
	case "latency":
		return l.latency()
	case "handoff":
		return l.handoff(7000000000000)
	case "late-handoff":
		return l.handoff(7000000000001)
	case "ddl-add":
		return l.ddl(true)
	case "ddl-drop":
		return l.ddl(false)
	default:
		return fmt.Errorf("unknown action %s", action)
	}
}
func atomicJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err = os.WriteFile(temp, b, 0600); err != nil {
		return err
	}
	defer os.Remove(temp)
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = os.Rename(temp, path)
		locked := os.IsPermission(err) || errors.Is(err, syscall.Errno(32))
		if err == nil || runtime.GOOS != "windows" || !locked || time.Now().After(deadline) {
			return err
		}
		// Readers and virus scanners can briefly deny replacement on Windows.
		time.Sleep(20 * time.Millisecond)
	}
}
func (l *lab) ledger() string { return quote(l.c.Prefix + "_commits") }
func (l *lab) oracle() string { return quote(l.c.Prefix + "_oracle") }
func (l *lab) init() error {
	for i, side := range l.sides {
		for _, t := range []table{side.Stream, side.Target, side.State} {
			if _, err := l.db[i].Exec(t.ddl(l.s)); err != nil {
				return err
			}
			var n int
			if err := l.db[i].QueryRow("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=?", t.Name).Scan(&n); err != nil {
				return err
			}
			if n != 50 {
				return fmt.Errorf("table %s has %d columns", t.Name, n)
			}
		}
		if _, err := l.db[i].Exec("CREATE TABLE " + l.ledger() + " (tick BIGINT PRIMARY KEY, inserted BIGINT NOT NULL, updated BIGINT NOT NULL, committed_at DATETIME(3) NOT NULL) ENGINE=InnoDB"); err != nil {
			return err
		}
		if _, err := l.db[i].Exec("CREATE TABLE " + l.oracle() + " (id BIGINT PRIMARY KEY, revision BIGINT NOT NULL, row_json JSON NOT NULL) ENGINE=InnoDB"); err != nil {
			return err
		}
	}
	return atomicJSON(filepath.Join(l.root, "schema-verified.json"), map[string]any{"at": time.Now(), "columns": 50, "sides": l.sides})
}

func insertSQL(t table, s schema, count int) string {
	row := "(" + strings.TrimSuffix(strings.Repeat("?,", len(s.Columns)), ",") + ")"
	return "INSERT INTO " + quote(t.Name) + " (" + t.names(s) + ") VALUES " + strings.TrimSuffix(strings.Repeat(row+",", count), ",")
}
func (l *lab) insert(ctx context.Context, tx *sql.Tx, t table, rows [][]any) error {
	args := make([]any, 0, len(rows)*50)
	for _, row := range rows {
		args = append(args, row...)
	}
	res, err := tx.ExecContext(ctx, insertSQL(t, l.s, len(rows)), args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != int64(len(rows)) {
		return fmt.Errorf("INSERT affected %d/%d", n, len(rows))
	}
	return nil
}
func (l *lab) saveOracle(ctx context.Context, tx *sql.Tx, row []any) error {
	b, err := json.Marshal(row)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO "+l.oracle()+" (id,revision,row_json) VALUES (?,?,?) ON DUPLICATE KEY UPDATE revision=VALUES(revision),row_json=VALUES(row_json)", row[0], row[3], string(b))
	return err
}

func (l *lab) saveOracleBatch(ctx context.Context, tx *sql.Tx, rows [][]any) error {
	args := make([]any, 0, len(rows)*3)
	for _, row := range rows {
		b, err := json.Marshal(row)
		if err != nil {
			return err
		}
		args = append(args, row[0], row[3], string(b))
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO "+l.oracle()+" (id,revision,row_json) VALUES "+strings.TrimSuffix(strings.Repeat("(?,?,?),", len(rows)), ",")+" ON DUPLICATE KEY UPDATE revision=VALUES(revision),row_json=VALUES(row_json)", args...)
	return err
}
func (l *lab) bootstrap() error {
	for i, side := range l.sides {
		for first := int64(1); first <= 1000; first += 100 {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			tx, err := l.db[i].BeginTx(ctx, nil)
			if err != nil {
				cancel()
				return err
			}
			rows := make([][]any, 0, 100)
			for id := first; id < first+100; id++ {
				rows = append(rows, makeRow(l.c, side, side.Offset+id, 1))
			}
			err = l.insert(ctx, tx, side.State, rows)
			if err == nil {
				err = l.saveOracleBatch(ctx, tx, rows)
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
		}
	}
	return nil
}

func (l *lab) run() error {
	start, err := time.Parse(time.RFC3339Nano, l.c.Start)
	if err != nil {
		return err
	}
	if l.c.DurationSeconds < 60 {
		return errors.New("duration must be at least 60 seconds")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observation := make(chan error, 1)
	go func() {
		err := l.observe(ctx)
		observation <- err
		if err != nil {
			cancel()
		}
	}()
	var wg sync.WaitGroup
	result := make(chan error, 2)
	for i := range l.db {
		wg.Add(1)
		go func(side int) {
			defer wg.Done()
			err := l.produce(ctx, side, start)
			result <- err
			if err != nil {
				cancel()
			}
		}(i)
	}
	wg.Wait()
	cancel()
	if err := <-observation; err != nil {
		return fmt.Errorf("online version observer: %w", err)
	}
	close(result)
	for err := range result {
		if err != nil {
			return err
		}
	}
	return nil
}
func (l *lab) produce(ctx context.Context, side int, start time.Time) (returnErr error) {
	p := progress{Side: l.sides[side].Node}
	output := filepath.Join(l.root, fmt.Sprintf("producer-%d.json", side))
	defer func() {
		p.At = time.Now()
		p.Done = true
		if returnErr != nil {
			p.Error = returnErr.Error()
		}
		_ = atomicJSON(output, p)
	}()
	rows := make([][]any, 1000)
	for i := range rows {
		rows[i] = makeRow(l.c, l.sides[side], l.sides[side].Offset+int64(i)+1, 1)
	}
	if l.seed != nil {
		var err error
		p, rows, err = l.seed(side)
		if err != nil {
			return err
		}
	}
	firstTick := p.Tick
	generationTicks := l.c.DurationSeconds * 390 / 420
	for tick := 0; tick < generationTicks; tick++ {
		due := start.Add(time.Duration(tick) * time.Second)
		timer := time.NewTimer(time.Until(due))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if _, err := os.Stat(filepath.Join(l.root, "stop.requested")); err == nil {
			return errors.New("stop requested")
		}
		rates := l.rateAt(float64(tick) * 420 / float64(l.c.DurationSeconds))
		tickctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		tx, err := l.db[side].BeginTx(tickctx, nil)
		if err != nil {
			cancel()
			return err
		}
		inserts := make([][]any, rates.I)
		for i := range inserts {
			inserts[i] = makeRow(l.c, l.sides[side], p.Inserts+int64(i)+1, 1)
		}
		err = l.insert(tickctx, tx, l.sides[side].Stream, inserts)
		coverage := p.Coverage
		dataCoverage := p.Data
		for _, row := range inserts {
			dataCoverage.row(row)
		}
		oracleRows := map[int][]any{}
		for u := 0; u < rates.U && err == nil; u++ {
			op := p.Updates + int64(u)
			slot := int(op % 1000)
			if op%20 >= 18 {
				slot = 0
			}
			row := rows[slot]
			revision := row[3].(int64) + 1
			row[3], row[4], row[5], row[6], row[8] = revision, revision, l.sides[side].Node, "", time.Now().UTC().Format("2006-01-02 15:04:05.000")
			indices := updateIndices(op)
			for _, index := range indices {
				value := updatedValue(row[0].(int64), revision, index, op)
				dataCoverage.change(index, row[index], value)
				row[index] = value
				coverage[index]++
			}
			dataCoverage.row(row)
			err = l.update(tickctx, tx, l.sides[side].State, row, append([]int{3, 4, 5, 6, 8}, indices...))
			oracleRows[slot] = row
		}
		if err == nil {
			batch := make([][]any, 0, len(oracleRows))
			for _, row := range oracleRows {
				batch = append(batch, row)
			}
			err = l.saveOracleBatch(tickctx, tx, batch)
		}
		if err == nil {
			_, err = tx.ExecContext(tickctx, "INSERT INTO "+l.ledger()+" VALUES (?,?,?,UTC_TIMESTAMP(3))", firstTick+tick, rates.I, rates.U)
		}
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		cancel()
		if err != nil {
			return fmt.Errorf("side %d tick %d: %w", side, tick, err)
		}
		p.Inserts += int64(rates.I)
		p.Updates += int64(rates.U)
		p.Coverage = coverage
		p.Data = dataCoverage
		p.Tick = firstTick + tick + 1
		p.At = time.Now()
		p.LatenessMS = time.Since(due).Milliseconds()
		if err = atomicJSON(output, p); err != nil {
			return err
		}
		if time.Since(due) > 30*time.Second {
			return fmt.Errorf("producer %d fell over 30 seconds behind", side)
		}
	}
	return nil
}
func (l *lab) update(ctx context.Context, tx *sql.Tx, t table, row []any, indices []int) error {
	sets := make([]string, 0, len(indices))
	args := make([]any, 0, len(indices)+1)
	for _, index := range indices {
		sets = append(sets, quote(t.col(l.s.Columns[index].Name))+"=?")
		args = append(args, row[index])
	}
	args = append(args, row[0])
	res, err := tx.ExecContext(ctx, "UPDATE "+quote(t.Name)+" SET "+strings.Join(sets, ",")+" WHERE "+quote(t.col("id"))+"=?", args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("UPDATE affected %d for %v", n, row[0])
	}
	return nil
}

func (l *lab) snapshot(ctx context.Context) error {
	result := map[string]any{"at": time.Now()}
	for i, side := range l.sides {
		d := map[string]any{}
		for _, entry := range []struct {
			label string
			t     table
		}{{"source", side.Stream}, {"incoming", side.Target}, {"state", side.State}} {
			label, t := entry.label, entry.t
			var count, max int64
			if err := snapshotQuery(ctx, os.Stderr, l.db[i], side.Node, label, "SELECT COUNT(*),COALESCE(MAX("+quote(t.col("id"))+"),0) FROM "+quote(t.Name), nil, &count, &max); err != nil {
				return err
			}
			d[label] = map[string]int64{"count": count, "max_id": max}
		}
		var ic, uc int64
		if err := snapshotQuery(ctx, os.Stderr, l.db[i], side.Node, "ledger", "SELECT COALESCE(SUM(inserted),0),COALESCE(SUM(updated),0) FROM "+l.ledger(), nil, &ic, &uc); err != nil {
			return err
		}
		d["committed_inserts"], d["committed_updates"] = ic, uc
		var failures int64
		if err := snapshotQuery(ctx, os.Stderr, l.db[i], side.Node, "error_log", "SELECT COUNT(*) FROM sync_error_log WHERE created_at>=? AND (event_id IS NULL OR event_id NOT IN (?,?))", []any{l.c.Created, l.c.RunID + "-different-id-same-key", l.c.RunID + "_late-different-id-same-key"}, &failures); err != nil {
			return err
		}
		var failedACK int64
		if err := snapshotQuery(ctx, os.Stderr, l.db[i], side.Node, "failed_acks", "SELECT COUNT(*) FROM sync_ack_log WHERE created_at>=? AND status='FAILED'", []any{l.c.Created}, &failedACK); err != nil {
			return err
		}
		d["failed_acks"] = failedACK
		if i == 1 {
			var failedEvents int64
			if err := snapshotQuery(ctx, os.Stderr, l.db[i], side.Node, "failed_event_log", "SELECT COUNT(*) FROM sync_event_log WHERE table_name IN (?,?,?,?) AND status='FAILED'", []any{l.sides[0].Stream.Name, l.sides[1].Stream.Name, l.sides[0].State.Name, l.sides[1].State.Name}, &failedEvents); err != nil {
				return err
			}
			failures += failedEvents
		}
		d["failed_events"] = failures
		result[side.Node] = d
	}
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
