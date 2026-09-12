package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestIncidentRateOverrideKeepsDefault(t *testing.T) {
	l := lab{}
	for _, minute := range []float64{0, 120, 270, 300, 410} {
		if l.rateAt(minute) != rateAt(minute) {
			t.Fatal("default workload changed")
		}
	}
	l.rates = func(float64) rate { return rate{80, 40} }
	if l.rateAt(0) != (rate{80, 40}) {
		t.Fatal("diagnostic override ignored")
	}
}

// Opt-in diagnostics: the supervisor owns Agent, rules, queue isolation and restoration.
func TestIncidentSustainedPeakReal(t *testing.T) {
	path := os.Getenv("NODEBRIDGE_INCIDENT_PEAK_CONFIG")
	if path == "" {
		t.Skip("NODEBRIDGE_INCIDENT_PEAK_CONFIG required")
	}
	c, s, err := readSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	resume := os.Getenv("NODEBRIDGE_INCIDENT_RESUME") == "backed-up-original-scene"
	validPrefix := len(c.Prefix) >= 12 && c.Prefix[:12] == "nb_incident_"
	if resume {
		validPrefix = c.Prefix == "nb7_wide25200s_0910_213112"
	}
	if c.DurationSeconds < 120 || c.DurationSeconds > 1800 || !validPrefix {
		t.Fatal("bounded incident configuration required")
	}
	l := lab{c: c, s: s, root: filepath.Dir(path), sides: layouts(c.Prefix), rates: func(float64) rate { return rate{80, 40} }}
	for i, dsn := range []string{c.EdgeDSN, c.ServerDSN} {
		l.db[i], err = sql.Open("mysql", dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer l.db[i].Close()
		l.db[i].SetMaxOpenConns(4)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.DurationSeconds+180)*time.Second)
	defer cancel()
	verify := l.verify
	if resume {
		if err := l.prepareIncidentResume(); err != nil {
			t.Fatal(err)
		}
		verify = l.verifyIncidentResume
	} else {
		if err := l.init(); err != nil {
			t.Fatal(err)
		}
		if err := l.bootstrap(); err != nil {
			t.Fatal(err)
		}
		bootstrapDeadline := time.Now().Add(2 * time.Minute)
		for {
			err := l.verify()
			if err == nil {
				break
			}
			if time.Now().After(bootstrapDeadline) {
				t.Fatal(err)
			}
			time.Sleep(time.Second)
		}
	}
	start := time.Now()
	l.c.Start = start.Format(time.RFC3339Nano)
	if err := atomicJSON(path, l.c); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(l.root, "mysql-waits.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var monitor sync.WaitGroup
	monitorCtx, stopMonitor := context.WithCancel(ctx)
	monitor.Add(1)
	go func() {
		defer monitor.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		enc := json.NewEncoder(f)
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				entry := map[string]any{"at": time.Now(), "seconds": time.Since(start).Seconds()}
				for name, query := range map[string]string{
					"status":     "SELECT VARIABLE_NAME,VARIABLE_VALUE FROM performance_schema.global_status WHERE VARIABLE_NAME IN ('Innodb_redo_log_current_lsn','Innodb_redo_log_checkpoint_lsn','Innodb_buffer_pool_reads','Innodb_buffer_pool_wait_free','Innodb_os_log_written','Innodb_data_written')",
					"locks":      "SELECT r.OBJECT_NAME,r.INDEX_NAME,r.LOCK_MODE,r.LOCK_DATA,b.LOCK_MODE,b.LOCK_DATA FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks r ON w.REQUESTING_ENGINE_LOCK_ID=r.ENGINE_LOCK_ID JOIN performance_schema.data_locks b ON w.BLOCKING_ENGINE_LOCK_ID=b.ENGINE_LOCK_ID LIMIT 12",
					"statements": "SELECT USER,STATE,LEFT(INFO,180) FROM information_schema.PROCESSLIST WHERE DB='scada_center' AND COMMAND<>'Sleep' AND ID<>CONNECTION_ID()",
				} {
					rows, err := l.db[1].QueryContext(monitorCtx, query)
					if err != nil {
						entry[name+"_error"] = err.Error()
						continue
					}
					cols, _ := rows.Columns()
					var values [][]sql.NullString
					for rows.Next() {
						v := make([]sql.NullString, len(cols))
						d := make([]any, len(cols))
						for i := range d {
							d[i] = &v[i]
						}
						if err := rows.Scan(d...); err == nil {
							values = append(values, v)
						}
					}
					rows.Close()
					entry[name] = values
				}
				if err := enc.Encode(entry); err != nil {
					return
				}
			}
		}
	}()
	defer func() { stopMonitor(); monitor.Wait() }()
	observations := make(chan error, 1)
	go func() { observations <- l.observe(monitorCtx) }()
	defer func() {
		stopMonitor()
		if err := <-observations; err != nil {
			t.Error(err)
		}
	}()
	if resume {
		latencies := make(chan error, 1)
		go func() {
			for {
				if err := l.latency(); err != nil {
					latencies <- err
					cancel()
					return
				}
				select {
				case <-monitorCtx.Done():
					latencies <- nil
					return
				case <-time.After(30 * time.Second):
				}
			}
		}()
		defer func() {
			stopMonitor()
			if err := <-latencies; err != nil {
				t.Error(err)
			}
		}()
	}
	errors := make(chan error, 2)
	for side := range l.db {
		go func(side int) { errors <- l.produce(ctx, side, start) }(side)
	}
	var productionError error
	for range l.db {
		if err := <-errors; err != nil {
			productionError = err
			cancel()
		}
	}
	if productionError != nil {
		t.Fatal(productionError)
	}
	var last error
	for time.Since(start) < time.Duration(c.DurationSeconds+120)*time.Second {
		last = verify()
		if last == nil {
			fmt.Printf("incident peak verification passed: resumed=%v seconds=%.3f\n", resume, time.Since(start).Seconds())
			return
		}
		time.Sleep(3 * time.Second)
	}
	t.Fatal(last)
}
