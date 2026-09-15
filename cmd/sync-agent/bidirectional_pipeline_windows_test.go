//go:build windows

package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/eventstatus"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
	"github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

func TestOwnedBidirectionalPipeline(t *testing.T) {
	if os.Getenv("NODEBRIDGE_OWNED_CDC_FIXTURE") != "1" || os.Getenv("NODEBRIDGE_CDC_TEST_SECOND_CANAL_ADDR") == "" {
		t.Skip("run scripts/test-canal-business-pipeline.ps1 -Bidirectional")
	}
	root, err := filepath.Abs(os.Getenv("NODEBRIDGE_CDC_FIXTURE_ROOT"))
	allowed, _ := filepath.Abs("../../.cache/canal-business")
	if err != nil || !strings.HasPrefix(strings.ToLower(root), strings.ToLower(allowed)+string(os.PathSeparator)) {
		t.Fatal("owned fixture directory required")
	}
	my, err := mysql.ParseDSN(os.Getenv("NODEBRIDGE_CDC_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	host, portText, err := net.SplitHostPort(my.Addr)
	broker := os.Getenv("NODEBRIDGE_CDC_TEST_RABBITMQ_URL")
	b, brokerErr := url.Parse(broker)
	if err != nil || my.Net != "tcp" || my.DBName != "" || host != "127.0.0.1" || brokerErr != nil || b.Hostname() != "127.0.0.1" {
		t.Fatal("owned loopback fixtures required")
	}
	canals := []string{os.Getenv("NODEBRIDGE_CDC_TEST_CANAL_ADDR"), os.Getenv("NODEBRIDGE_CDC_TEST_SECOND_CANAL_ADDR")}
	for _, canal := range canals {
		h, _, e := net.SplitHostPort(canal)
		if e != nil || h != "127.0.0.1" {
			t.Fatal("owned Canal required")
		}
	}
	port, _ := strconv.Atoi(portText)
	my.ParseTime = true
	my.Timeout, my.ReadTimeout, my.WriteTimeout = 3*time.Second, 5*time.Second, 5*time.Second
	secondMySQL, err := mysql.ParseDSN(os.Getenv("NODEBRIDGE_CDC_TEST_SECOND_DSN"))
	if err != nil || secondMySQL.Net != "tcp" || secondMySQL.DBName != "" || secondMySQL.Addr == my.Addr {
		t.Fatal("two independent owned MySQL instances required", err)
	}
	secondHost, secondPortText, err := net.SplitHostPort(secondMySQL.Addr)
	if err != nil || secondHost != "127.0.0.1" {
		t.Fatal("second MySQL must bind loopback", err)
	}
	secondPort, _ := strconv.Atoi(secondPortText)
	secondMySQL.ParseTime = true
	secondMySQL.Timeout, secondMySQL.ReadTimeout, secondMySQL.WriteTimeout = my.Timeout, my.ReadTimeout, my.WriteTimeout
	dbConfigs, dbPorts := []*mysql.Config{my, secondMySQL}, []int{port, secondPort}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	t.Cleanup(cancel)
	databases := []string{"nb_cdc_source", "nb_cdc_target"}
	tables, keys, values := []string{"source_rows", "target_rows"}, []string{"edge_id", "server_id"}, []string{"edge_value", "server_value"}
	modes, nodes := []string{"edge", "server"}, []string{"owned-bidi-edge", "owned-bidi-server"}
	var dbs [2]*sql.DB
	commandTime := time.Now().Unix()
	execute := func(db *sql.DB, query string, args ...any) {
		t.Helper()
		// The owned host can step its wall clock backwards. Pin only test-issued
		// commands to explicit source seconds; Agent sessions stay untouched.
		commandTime++
		connection, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		defer func() { _ = connection.Raw(func(any) error { return driver.ErrBadConn }) }()
		if _, err := connection.ExecContext(ctx, "SET TIMESTAMP=?", commandTime); err != nil {
			t.Fatal(err)
		}
		if _, err := connection.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	for i, name := range databases {
		admin, err := sql.Open("mysql", dbConfigs[i].FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { admin.Close() })
		execute(admin, "CREATE DATABASE `"+name+"`")
		t.Cleanup(func() {
			cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()
			if _, err := admin.ExecContext(cleanup, "DROP DATABASE `"+name+"`"); err != nil {
				t.Error(err)
			}
		})
		cfg := *dbConfigs[i]
		cfg.DBName = name
		db, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		dbs[i] = db
		t.Cleanup(func() { db.Close() })
		if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/"+modes[i]); err != nil {
			t.Fatal(err)
		}
		execute(db, "CREATE TABLE "+tables[i]+" ("+keys[i]+" BIGINT UNSIGNED PRIMARY KEY,"+values[i]+" DECIMAL(30,10) NOT NULL,raw_bytes VARBINARY(8),note VARCHAR(32)) ENGINE=InnoDB")
	}
	var uuids [2]string
	for i := range dbs {
		if err := dbs[i].QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&uuids[i]); err != nil {
			t.Fatal(err)
		}
	}
	if uuids[0] == uuids[1] {
		t.Fatal("paired pipeline must use different MySQL instances")
	}
	conn, err := amqp091.Dial(broker)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ch.Close() })
	for _, topology := range []rabbitmq.Topology{rabbitmq.EdgeTopology(), rabbitmq.ServerTopology([]string{nodes[0]})} {
		if err := rabbitmq.InitializeTopology(ch, topology); err != nil {
			t.Fatal(err)
		}
	}
	rule := rules.SyncRule{ID: "owned-bidi", Enable: true, DatabaseName: databases[0], TableName: tables[0], TargetDatabaseName: databases[1], TargetTableName: tables[1], PrimaryKeys: []string{keys[0]}, TargetPrimaryKeys: []string{keys[1]}, SourceNodeIDs: []string{nodes[0]}, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteHard, ColumnMappings: []rules.ColumnMapping{{SourceColumn: values[0], TargetColumn: values[1]}}}
	if os.Getenv("NODEBRIDGE_CDC_INITIAL_ALIGNMENT") != "1" {
		t.Setenv("NODEBRIDGE_CDC_ALIGNMENT_SOURCE", "empty")
	}
	rule.Enable = false
	rule.InitialAlignment.Policy = rules.AlignmentManual
	ruleBytes, err := yaml.Marshal(rules.RuleSet{Rules: []rules.SyncRule{rule}})
	if err != nil {
		t.Fatal(err)
	}
	for i, mode := range modes {
		dir := filepath.Join(root, mode)
		cfg := appconfig.Config{Mode: mode, Node: appconfig.NodeConfig{ID: nodes[i]}, MySQL: appconfig.MySQLConfig{Host: host, Port: dbPorts[i], Username: dbConfigs[i].User, Password: dbConfigs[i].Passwd, Database: databases[i]}, RabbitMQ: appconfig.RabbitMQConfig{Mode: "external", LocalURL: broker, ServerURL: broker}, CDC: appconfig.CDCConfig{Type: "canal", Mode: "external", CanalAddr: canals[i], Destination: "example", ReaderName: "owned-bidi-reader", Filter: databases[i] + `\.` + tables[i], BatchSize: 1}, Sync: appconfig.SyncConfig{UploadBatchSize: 16, DispatchBatchSize: 16, FlushIntervalMillis: 50, RetryIntervalSeconds: 1}}
		if err := appconfig.SaveFile(filepath.Join(dir, "config.yaml"), cfg); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), ruleBytes, 0600); err != nil {
			t.Fatal(err)
		}
	}
	prepareOwnedInitialAlignment(t, ctx, root, dbs, ch, rule)
	start := func(i int) func() {
		t.Helper()
		dir := filepath.Join(root, modes[i])
		stopFile := filepath.Join(dir, "stop.request")
		log, err := os.Create(filepath.Join(dir, fmt.Sprintf("paired-%d.log", time.Now().UnixNano())))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, filepath.Join(root, "SyncAgent.exe"), "run", "-config", filepath.Join(dir, "config.yaml"), "-rules", filepath.Join(dir, "rules.yaml"), "-stop-file", stopFile, "-edges", nodes[0])
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			log.Close()
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		var once sync.Once
		stop := func() {
			once.Do(func() {
				_ = os.WriteFile(stopFile, []byte("owned paired stop"), 0600)
				select {
				case err := <-done:
					if err != nil {
						t.Errorf("%s Agent: %v; %s", modes[i], err, dir)
					}
				case <-time.After(8 * time.Second):
					_ = cmd.Process.Kill()
					<-done
					t.Errorf("%s forced cleanup", modes[i])
				}
				log.Close()
			})
		}
		t.Cleanup(stop)
		return stop
	}
	stops := []func(){start(0), start(1)}
	diagnostics := func(label string) {
		state := map[string]any{"failure": label}
		for i, db := range dbs {
			node := map[string]any{}
			for _, table := range []string{tables[i], "sync_row_version", "sync_conflict_event", "sync_conflict_state", "sync_apply_log", "sync_delete_replay", "sync_repair_replay", "sync_capture_fence", "sync_replay_marker", "sync_replay_position"} {
				queryCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
				rows, err := db.QueryContext(queryCtx, "SELECT * FROM "+table+" LIMIT 100")
				if err != nil {
					node[table] = err.Error()
					done()
					continue
				}
				columns, _ := rows.Columns()
				data := []map[string]string{}
				for rows.Next() {
					values, scan := make([][]byte, len(columns)), make([]any, len(columns))
					for j := range scan {
						scan[j] = &values[j]
					}
					if err := rows.Scan(scan...); err != nil {
						t.Log("diagnostic scan", err)
						break
					}
					record := map[string]string{}
					for j, column := range columns {
						if strings.Contains(column, "hash") || strings.Contains(column, "identity") {
							record[column] = fmt.Sprintf("%x", values[j])
						} else {
							record[column] = string(values[j])
						}
					}
					data = append(data, record)
				}
				rows.Close()
				done()
				node[table] = data
			}
			state[modes[i]] = node
		}
		b, err := json.MarshalIndent(state, "", "  ")
		if err == nil {
			err = os.WriteFile(filepath.Join(root, "failure-state.json"), b, 0600)
		}
		if err != nil {
			t.Log("write failure diagnostics", err)
		}
	}
	wait := func(label string, condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(25 * time.Second)
		for time.Now().Before(deadline) && ctx.Err() == nil {
			if condition() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		diagnostics(label)
		t.Fatalf("%s timed out: %s", label, root)
	}
	insert := func(i int, key uint64, value, note string) {
		execute(dbs[i], "INSERT INTO "+tables[i]+" ("+keys[i]+","+values[i]+",raw_bytes,note) VALUES (?,?,X'0080FF',?)", key, value, note)
	}
	update := func(i int, key uint64, note string) {
		execute(dbs[i], "UPDATE "+tables[i]+" SET note=? WHERE "+keys[i]+"=?", note, key)
	}
	read := func(i int, key uint64, note string) bool {
		var actual string
		return dbs[i].QueryRowContext(ctx, "SELECT note FROM "+tables[i]+" WHERE "+keys[i]+"=?", key).Scan(&actual) == nil && actual == note
	}
	absent := func(i int, key uint64) bool {
		var n int
		return dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tables[i]+" WHERE "+keys[i]+"=?", key).Scan(&n) == nil && n == 0
	}
	for i := range 2 {
		warm := uint64(1000 + i*1000)
		wait("paired CDC readiness "+modes[i], func() bool {
			insert(i, warm, "1", "warm")
			warm++
			var n int
			return dbs[1-i].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tables[1-i]+" WHERE "+keys[1-i]+">=? AND "+keys[1-i]+"<?", 1000+i*1000, 2000+i*1000).Scan(&n) == nil && n > 0
		})
	}
	{
		wait("post-snapshot pre-start write", func() bool { return read(0, 8, "before-agent-start") && read(1, 8, "before-agent-start") })
		for i := range nodes {
			wait("legacy isolation "+modes[i], func() bool {
				var n int
				return dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_alignment_event WHERE event_id=?", "legacy-before-alignment-"+modes[1-i]).Scan(&n) == nil && n == 1
			})
			archiveStatus, err := (eventstatus.Service{DB: dbs[i], LogPath: filepath.Join(root, modes[i], "logs", "sync-runtime.jsonl")}).Query(ctx, eventstatus.Request{EventID: "legacy-before-alignment-" + modes[1-i]})
			if err != nil || len(archiveStatus.Items) != 1 || archiveStatus.Items[0].ApplyStatus != "superseded" || archiveStatus.Items[0].AppliedAt != "" {
				t.Fatal("archived old input reported as applied or unknown", archiveStatus, err)
			}
			if os.Getenv("NODEBRIDGE_CDC_ALIGNMENT_SOURCE") == "empty" {
				if !absent(i, 7) {
					t.Fatal("old INSERT resurrected a both-empty table")
				}
			} else if !read(i, 7, "snapshot") {
				t.Fatal("legacy input overwrote snapshot")
			}
		}
		t.Log("PASS: first-copy cutover keeps pre-start writes and archives old messages on both endpoints without overwriting/resurrecting the baseline")
	}
	const key uint64 = 18446744073709551615
	const amount = "12345678901234567890.1234567890"
	insert(0, key, amount, "edge-insert")
	wait("edge mapped INSERT", func() bool { return read(1, key, "edge-insert") })
	var gotAmount, gotBytes string
	if err := dbs[1].QueryRowContext(ctx, "SELECT CAST(server_value AS CHAR),HEX(raw_bytes) FROM target_rows WHERE server_id=?", key).Scan(&gotAmount, &gotBytes); err != nil || gotAmount != amount || gotBytes != "0080FF" {
		t.Fatalf("lossless values: %s %s %v", gotAmount, gotBytes, err)
	}
	time.Sleep(1100 * time.Millisecond)
	update(1, key, "server-update")
	wait("server inverse UPDATE", func() bool { return read(0, key, "server-update") })
	time.Sleep(1100 * time.Millisecond)
	update(0, key, "edge-retained-marker")
	wait("edge retained marker UPDATE", func() bool { return read(1, key, "edge-retained-marker") })
	time.Sleep(1100 * time.Millisecond)
	execute(dbs[1], "DELETE FROM target_rows WHERE server_id=?", key)
	wait("server hard DELETE", func() bool { return absent(0, key) && absent(1, key) })
	time.Sleep(1100 * time.Millisecond)
	insert(0, key, amount, "edge-recreate")
	wait("newer recreate", func() bool { return read(1, key, "edge-recreate") })
	for _, winner := range []int{1, 0} {
		stops[0]()
		stops[1]()
		loser := 1 - winner
		update(loser, key, "offline-older")
		time.Sleep(1100 * time.Millisecond)
		want := "offline-newer-" + modes[winner]
		update(winner, key, want)
		stops[0], stops[1] = start(0), start(1)
		wait("two local writers converge to "+modes[winner], func() bool { return read(0, key, want) && read(1, key, want) })
	}
	for round := range 6 {
		older, winner := round%2, 1-round%2
		if round != 0 {
			// Cross-node binlog timestamps are second-granular; this scenario
			// asserts a strictly newer recreation, not the deterministic tie rule.
			time.Sleep(1100 * time.Millisecond)
			insert(older, key, amount, "delete-round")
			wait("recreate before offline delete", func() bool { return read(0, key, "delete-round") && read(1, key, "delete-round") })
		}
		stops[0]()
		stops[1]()
		update(older, key, "before-delete")
		time.Sleep(1100 * time.Millisecond)
		execute(dbs[winner], "DELETE FROM "+tables[winner]+" WHERE "+keys[winner]+"=?", key)
		stops[0], stops[1] = start(0), start(1)
		wait(fmt.Sprintf("newer tombstone wins offline conflict round %d", round), func() bool {
			if !absent(0, key) || !absent(1, key) {
				return false
			}
			for i, db := range dbs {
				var n int
				query := "SELECT COUNT(*) FROM sync_conflict_state WHERE JSON_UNQUOTE(JSON_EXTRACT(state_json,'$.primary_key." + keys[i] + "'))=? AND JSON_UNQUOTE(JSON_EXTRACT(state_json,'$.version.origin_node_id'))=? AND JSON_EXTRACT(state_json,'$.version.deleted')=TRUE AND repair_required=0"
				if err := db.QueryRowContext(ctx, query, strconv.FormatUint(key, 10), nodes[winner]).Scan(&n); err != nil || n != 1 {
					return false
				}
			}
			return true
		})
	}
	const retryKey uint64 = 9007199254740993
	execute(dbs[0], "CREATE TRIGGER reject_owned_receipt BEFORE INSERT ON sync_apply_log FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned receipt failure'")
	insert(1, retryKey, "1", "retry-after-rollback")
	loggedFailure := func(worker string) bool {
		data, err := os.ReadFile(filepath.Join(root, "edge", "logs", "sync-runtime.jsonl"))
		if err != nil {
			return false
		}
		for _, line := range strings.Split(string(data), "\n") {
			var record struct {
				Worker string `json:"worker"`
				Code   int    `json:"mysql_error_code"`
			}
			if json.Unmarshal([]byte(line), &record) == nil && record.Worker == worker && record.Code == 1644 {
				return true
			}
		}
		return false
	}
	wait("receipt failure actually attempted", func() bool { return loggedFailure("edge-downlink") })
	if !absent(0, retryKey) {
		t.Fatal("failed receipt did not roll back business write")
	}
	execute(dbs[0], "DROP TRIGGER reject_owned_receipt")
	wait("receipt rollback retry", func() bool { return read(0, retryKey, "retry-after-rollback") })
	execute(dbs[0], "CREATE TRIGGER reject_owned_version BEFORE INSERT ON sync_conflict_event FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned local version failure'")
	insert(0, retryKey+1, "2", "capture-reconnect")
	wait("local version failure actually attempted", func() bool { return loggedFailure("edge-cdc-canal") })
	execute(dbs[0], "DROP TRIGGER reject_owned_version")
	wait("Canal reconnect without restarting Agent", func() bool { return read(1, retryKey+1, "capture-reconnect") })
	var loserID string
	if err := dbs[1].QueryRowContext(ctx, "SELECT a.event_id FROM sync_apply_log a JOIN sync_conflict_event c ON JSON_UNQUOTE(JSON_EXTRACT(c.version_json,'$.event_id'))=a.event_id WHERE c.decision='SUPERSEDED' LIMIT 1").Scan(&loserID); err != nil {
		t.Fatal("superseded receipt missing", err)
	}
	observation, err := (eventstatus.Service{DB: dbs[1], LogPath: filepath.Join(root, "server", "logs", "sync-runtime.jsonl")}).Query(ctx, eventstatus.Request{EventID: loserID})
	if err != nil || len(observation.Items) != 1 || observation.Items[0].ApplyStatus != "superseded" {
		t.Fatalf("loser status: %+v %v", observation, err)
	}
	for _, i := range []int{0, 1} {
		var pending int
		wait("repairs drained "+modes[i], func() bool {
			return dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_conflict_state WHERE repair_required=1").Scan(&pending) == nil && pending == 0
		})
	}
	stops[0]()
	stops[1]()
	for i := range 2 {
		execute(dbs[i], "INSERT INTO sync_alignment_job (job_id,scope_hash,plan_id,node_id,endpoint_role,phase,plan_json,updated_at) VALUES (?,?,?,?,?,'PREPARED','{}',UTC_TIMESTAMP(6))", strings.Repeat("a", 64), ownedTableScope(databases[i], tables[i]), strings.Repeat("c", 64), nodes[i], []string{"SOURCE", "TARGET"}[i])
		dir := filepath.Join(root, modes[i])
		blockedCtx, done := context.WithTimeout(ctx, 8*time.Second)
		cmd := exec.CommandContext(blockedCtx, filepath.Join(root, "SyncAgent.exe"), "run", "-config", filepath.Join(dir, "config.yaml"), "-rules", filepath.Join(dir, "rules.yaml"), "-stop-file", filepath.Join(dir, "stop.request"), "-edges", nodes[0])
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		output, err := cmd.CombinedOutput()
		done()
		if err == nil || !strings.Contains(string(output), "alignment_cutover_pending") {
			t.Fatalf("%s Agent did not block unresolved alignment: %v %s", modes[i], err, output)
		}
	}
	t.Log("PASS: two production Agents with no business replay columns, batch-size-one Canal readers, RabbitMQ, lossless CRUD, local edits after replay, source-time LWW, offline delete tombstone, restart, receipt rollback/retry and repair drain. Short test only.")
	t.Log("PASS: both production Agent modes refuse to start with an unresolved durable alignment job")
}
