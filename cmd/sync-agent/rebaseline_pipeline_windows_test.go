//go:build windows

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
	"github.com/go-sql-driver/mysql"
	"github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

func TestOwnedRebaselinePipeline(t *testing.T) {
	if os.Getenv("NODEBRIDGE_OWNED_MULTI_FIXTURE") != "1" {
		t.Skip("run scripts/test-multi-node-sync.ps1 -Rebaseline")
	}
	root, err := filepath.Abs(os.Getenv("NODEBRIDGE_MULTI_FIXTURE_ROOT"))
	allowed, _ := filepath.Abs("../../.cache/multi-node-sync")
	if err != nil || !strings.HasPrefix(strings.ToLower(root), strings.ToLower(allowed)+string(os.PathSeparator)) {
		t.Fatal("owned fixture required")
	}
	var endpoints []struct {
		Port  int    `json:"mysql_port"`
		Canal string `json:"canal_addr"`
		URL   string `json:"local_url"`
	}
	if json.Unmarshal([]byte(os.Getenv("NODEBRIDGE_MULTI_ENDPOINTS")), &endpoints) != nil || len(endpoints) != 3 {
		t.Fatal("owned endpoints required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	t.Cleanup(cancel)
	nodes := []string{"owned-rebase-edge", "owned-rebase-server"}
	modes := []string{"edge", "server"}
	business := []string{"nb_multi_edge1", "nb_multi_server"}
	tables := []string{"edge_rows", "central_rows"}
	dirs := make([]string, 2)
	dbs := make([]*sql.DB, 2)
	clients := make([]*ownedMCPClient, 2)
	rule := rules.SyncRule{ID: "old-rule", DatabaseName: business[0], TableName: tables[0], TargetDatabaseName: business[1], TargetTableName: tables[1], PrimaryKeys: []string{"id"}, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteHard, SourceNodeIDs: []string{nodes[0]}, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}}
	sqlExec := func(i int, query string, args ...any) {
		t.Helper()
		if _, err := dbs[i].ExecContext(ctx, query, args...); err != nil {
			t.Fatal(query, err)
		}
	}
	for i := 0; i < 2; i++ {
		ep := endpoints[i]
		host, _, err := net.SplitHostPort(ep.Canal)
		broker, berr := url.Parse(ep.URL)
		if err != nil || berr != nil || host != "127.0.0.1" || broker.Hostname() != "127.0.0.1" || ep.Port < 1024 {
			t.Fatal("loopback only")
		}
		cfg := mysql.NewConfig()
		cfg.User, cfg.Passwd, cfg.Net, cfg.Addr = "root", "owned_multi_only", "tcp", fmt.Sprintf("127.0.0.1:%d", ep.Port)
		cfg.ParseTime = true
		dbs[i], err = sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { dbs[i].Close() })
		metadata := fmt.Sprintf("nb_old_meta_%d", i)
		sqlExec(i, "CREATE DATABASE "+metadata)
		sqlExec(i, "CREATE DATABASE "+business[i])
		sqlExec(i, "CREATE TABLE "+business[i]+"."+tables[i]+" (id BIGINT PRIMARY KEY,note VARCHAR(64),stamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB")
		cfg.DBName = metadata
		meta, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		if err := mysqlconn.RunMigrations(ctx, meta, "../../migrations/"+modes[i]); err != nil {
			t.Fatal(err)
		}
		meta.Close()
		conn, err := amqp091.Dial(ep.URL)
		if err != nil {
			t.Fatal(err)
		}
		ch, err := conn.Channel()
		if err != nil {
			t.Fatal(err)
		}
		topology := rabbitmq.EdgeTopology()
		if i == 1 {
			topology = rabbitmq.ServerTopology(nodes[:1])
		}
		if err := rabbitmq.InitializeTopology(ch, topology); err != nil {
			t.Fatal(err)
		}
		ch.Close()
		conn.Close()
		dirs[i] = filepath.Join(root, nodes[i])
		config := appconfig.Config{Mode: modes[i], Node: appconfig.NodeConfig{ID: nodes[i]}, MySQL: appconfig.MySQLConfig{Host: "127.0.0.1", Port: ep.Port, Username: "root", Password: "owned_multi_only", Database: metadata}, RabbitMQ: appconfig.RabbitMQConfig{Mode: "external", LocalURL: ep.URL, ServerURL: endpoints[1].URL}, CDC: appconfig.CDCConfig{Type: "canal", Mode: "external", CanalAddr: ep.Canal, Destination: "example", ReaderName: "owned-rebaseline", Filter: business[i] + `\.` + tables[i], BatchSize: 16}, Sync: appconfig.SyncConfig{UploadBatchSize: 16, DispatchBatchSize: 16, FlushIntervalMillis: 50, RetryIntervalSeconds: 1}, MCP: appconfig.MCPServerConfig{Enable: true}}
		if err := appconfig.SaveFile(filepath.Join(dirs[i], "config.yaml"), config); err != nil {
			t.Fatal(err)
		}
		data, _ := yaml.Marshal(rules.RuleSet{Rules: []rules.SyncRule{rule}})
		if err := os.WriteFile(filepath.Join(dirs[i], "rules.yaml"), data, 0600); err != nil {
			t.Fatal(err)
		}
		clients[i] = startOwnedMCP(t, ctx, filepath.Join(root, "SyncAgent.exe"), dirs[i])
	}
	sqlExec(0, "INSERT INTO "+business[0]+"."+tables[0]+" (id,note) VALUES(1,'edge-authority')")
	align := func(ruleID string) {
		t.Helper()
		for i := 0; i < 2; i++ {
			if _, err := clients[i].tool("nodebridge_start_initial_alignment", uiapi.InitialAlignmentRequest{RuleID: ruleID, PeerNodeID: nodes[1-i], Confirm: true}); err != nil {
				t.Fatal(err)
			}
		}
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) {
			complete := true
			for i := 0; i < 2; i++ {
				raw, err := clients[i].tool("nodebridge_initial_alignment_status", map[string]any{})
				var status uiapi.InitialAlignmentStatus
				if err != nil || json.Unmarshal(raw, &status) != nil {
					t.Fatal(err, string(raw))
				}
				if !status.Running && status.Stage != "completed" {
					t.Fatalf("alignment %s: %+v", nodes[i], status)
				}
				complete = complete && !status.Running && status.Stage == "completed"
			}
			if complete {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("alignment timeout")
	}
	align(rule.ID)
	t.Log("PASS: original bidirectional mapping has a real committed snapshot and active topology")
	desired := rule
	desired.ID = "replacement-rule"
	desired.Name = "中文双向计划"
	desired.TargetDatabaseName = "nb_rebased_business"
	sqlExec(1, "CREATE DATABASE "+desired.TargetDatabaseName)
	sqlExec(1, "CREATE TABLE "+desired.TargetDatabaseName+".central_rows LIKE "+business[1]+".central_rows")
	sqlExec(1, "INSERT INTO "+desired.TargetDatabaseName+".central_rows (id,note) VALUES(99,'target-only-backup')")
	plans := make([]alignment.RebaselinePlan, 2)
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		state := map[string]any{}
		for i := 0; i < 2; i++ {
			if plans[i].NewDatabase == "" {
				continue
			}
			for label, query := range map[string]string{"rows": "SELECT * FROM " + business[i] + "." + tables[i], "versions": "SELECT * FROM " + plans[i].NewDatabase + ".sync_row_version LIMIT 20", "retired": "SELECT event_id,reason FROM " + plans[i].NewDatabase + ".sync_alignment_event LIMIT 20"} {
				rows, err := dbs[i].QueryContext(ctx, query)
				if err != nil {
					state[fmt.Sprintf("%d-%s", i, label)] = err.Error()
					continue
				}
				columns, _ := rows.Columns()
				var records []map[string]string
				for rows.Next() {
					values := make([][]byte, len(columns))
					scan := make([]any, len(columns))
					for j := range scan {
						scan[j] = &values[j]
					}
					if rows.Scan(scan...) != nil {
						break
					}
					record := map[string]string{}
					for j, key := range columns {
						record[key] = string(values[j])
					}
					records = append(records, record)
				}
				rows.Close()
				state[fmt.Sprintf("%d-%s", i, label)] = records
			}
		}
		raw, _ := json.MarshalIndent(state, "", "  ")
		_ = os.WriteFile(filepath.Join(root, "rebaseline-failure.json"), raw, 0600)
	})
	for i := 0; i < 2; i++ {
		raw, err := clients[i].tool("nodebridge_plan_rebaseline", alignment.RebaselineRequest{MigrationID: "owned-replacement", EdgeNode: nodes[0], ServerNode: nodes[1], Rules: []rules.SyncRule{desired}})
		if err != nil || json.Unmarshal(raw, &plans[i]) != nil {
			t.Fatal("plan", err, string(raw))
		}
		if len(plans[i].Retired) != 1 {
			t.Fatal("old proof absent", len(plans[i].Retired))
		}
	}
	for i := 0; i < 2; i++ {
		request := alignment.RebaselineApply{Plan: plans[i], Confirm: true, TargetWritersStopped: true}
		for attempt := 0; attempt < 2; attempt++ {
			raw, err := clients[i].tool("nodebridge_prepare_rebaseline", request)
			var result alignment.RebaselineResult
			if err != nil || json.Unmarshal(raw, &result) != nil || !result.Prepared {
				t.Fatal("prepare/retry", err, string(raw))
			}
		}
		var active int
		if err := dbs[i].QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM nb_old_meta_%d.sync_alignment_cutover WHERE phase='ACTIVE'", i)).Scan(&active); err != nil || active != 1 {
			t.Fatal("old evidence altered", err, active)
		}
	}
	var rows int
	if err := dbs[1].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+plans[1].NewDatabase+"."+plans[1].Tables[0].BackupTable+" WHERE id=99 AND note='target-only-backup'").Scan(&rows); err != nil || rows != 1 {
		t.Fatal("target backup missing", err, rows)
	}
	if err := dbs[1].QueryRowContext(ctx, "SELECT COUNT(*) FROM nb_rebased_business.central_rows").Scan(&rows); err != nil || rows != 0 {
		t.Fatal("target not prepared", err, rows)
	}
	// Direct startup must refuse even if a caller manually enables unaligned rules.
	probeCtx, done := context.WithTimeout(ctx, 10*time.Second)
	probe := exec.CommandContext(probeCtx, filepath.Join(root, "SyncAgent.exe"), "run", "-config", filepath.Join(dirs[1], "config.yaml"), "-rules", filepath.Join(dirs[1], "rules.yaml"))
	probe.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, probeErr := probe.CombinedOutput()
	done()
	startupLog, _ := os.ReadFile(filepath.Join(dirs[1], "logs", "sync-runtime.jsonl"))
	output = append(output, startupLog...)
	if probeErr == nil || !strings.Contains(string(output), "rebaseline_alignment_incomplete") {
		t.Fatal("unaligned startup not blocked", probeErr, string(output))
	}
	t.Log("PASS: changed rule ID/target database prepared; old ledgers retained; target-only row backed up; retry idempotent; startup blocked")
	align(desired.ID)
	business[1] = desired.TargetDatabaseName
	for i := 0; i < 2; i++ {
		// Retrying preparation after copying must never delete the new baseline.
		if _, err := clients[i].tool("nodebridge_prepare_rebaseline", alignment.RebaselineApply{Plan: plans[i], Confirm: true, TargetWritersStopped: true}); err != nil {
			t.Fatal(err)
		}
		set, revision, err := rules.LoadFileWithRevision(filepath.Join(dirs[i], "rules.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		set.Rules[0].Enable = true
		if _, err := clients[i].tool("nodebridge_save_sync_rules", map[string]any{"rules": set.Rules, "expected_revision": revision}); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(label string, predicate func() bool) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if predicate() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("timeout: " + label)
	}
	has := func(i, id int, note string) bool {
		var n int
		return dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+business[i]+"."+tables[i]+" WHERE id=? AND note=?", id, note).Scan(&n) == nil && n == 1
	}
	absent := func(i, id int) bool {
		var n int
		return dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+business[i]+"."+tables[i]+" WHERE id=?", id).Scan(&n) == nil && n == 0
	}
	if !has(1, 1, "edge-authority") || !absent(1, 99) {
		t.Fatal("source authority lost")
	}
	broker, err := amqp091.Dial(endpoints[1].URL)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := broker.Channel()
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatal(err)
	}
	oldProof := plans[0].Retired[0]
	for i, boundary := range []alignment.SnapshotBoundary{oldProof.Source, oldProof.Target} {
		evt := event.SyncEvent{EventID: fmt.Sprintf("old-epoch-%d", i), EventType: event.TypeUpdate, OriginNodeID: boundary.OriginNodeID, SourceNodeID: boundary.OriginNodeID, TargetNodeID: nodes[1-i], DatabaseName: boundary.DatabaseName, TableName: boundary.TableName, PrimaryKey: map[string]any{"id": 1}, After: map[string]any{"id": 1, "note": "must-not-overwrite"}, Headers: map[string]string{alignment.EpochHeader: oldProof.ID, alignment.SourceUUIDHeader: boundary.MySQLServerUUID}}
		body, _ := json.Marshal(evt)
		exchange, key := "server.ingress.x", "server.ingress"
		if i == 1 {
			exchange, key = "server.dispatch.x", nodes[0]+".downlink"
		}
		if err := publisher.Publish(ctx, rabbitmq.PublishRequest{Exchange: exchange, RoutingKey: key, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	channel.Close()
	broker.Close()
	stops := make([]func(), 2)
	for i := 0; i < 2; i++ {
		dir := dirs[i]
		stopPath := filepath.Join(dir, "stop.request")
		log, err := os.Create(filepath.Join(dir, "rebaseline-agent.log"))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, filepath.Join(root, "SyncAgent.exe"), "run", "-config", filepath.Join(dir, "config.yaml"), "-rules", filepath.Join(dir, "rules.yaml"), "-stop-file", stopPath, "-edges", nodes[0])
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		finished := make(chan error, 1)
		go func() { finished <- cmd.Wait() }()
		var once sync.Once
		stops[i] = func() {
			once.Do(func() {
				_ = os.WriteFile(stopPath, []byte("stop"), 0600)
				select {
				case err := <-finished:
					if err != nil {
						t.Error("Agent", err)
					}
				case <-time.After(10 * time.Second):
					_ = cmd.Process.Kill()
					<-finished
					t.Error("Agent forced stop")
				}
				log.Close()
			})
		}
		t.Cleanup(stops[i])
	}
	for i := 0; i < 2; i++ {
		id := 10 + i
		note := fmt.Sprintf("insert-%d", i)
		sqlExec(i, "INSERT INTO "+business[i]+"."+tables[i]+" (id,note) VALUES (?,?)", id, note)
		wait(note, func() bool { return has(1-i, id, note) })
		// Binlog event time has one-second resolution; exercise a strictly newer winner.
		time.Sleep(1100 * time.Millisecond)
		note = fmt.Sprintf("update-%d", i)
		sqlExec(i, "UPDATE "+business[i]+"."+tables[i]+" SET note=? WHERE id=?", note, id)
		wait(note, func() bool { return has(1-i, id, note) })
		time.Sleep(1100 * time.Millisecond)
		sqlExec(i, "DELETE FROM "+business[i]+"."+tables[i]+" WHERE id=?", id)
		wait("delete", func() bool { return absent(1-i, id) })
	}
	for i := 0; i < 2; i++ {
		wait("old epoch archived", func() bool {
			var n int
			return dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+plans[i].NewDatabase+".sync_alignment_event WHERE event_id=?", fmt.Sprintf("old-epoch-%d", 1-i)).Scan(&n) == nil && n == 1
		})
	}
	if !has(0, 1, "edge-authority") || !has(1, 1, "edge-authority") {
		t.Fatal("retired messages overwrote new baseline")
	}
	for _, stop := range stops {
		stop()
	}
	t.Log("PASS: migrated target and changed bidirectional rule complete real Canal/RabbitMQ/Agent INSERT UPDATE DELETE in BOTH directions")
	t.Log("PASS: queued old epochs in both directions archived without overwriting the new baseline, including the old target database scope")
}
