//go:build windows

package main

import (
	"context"
	"database/sql"
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
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/buildinfo"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
	"github.com/rabbitmq/amqp091-go"
)

func TestOwnedCanalBusinessPipeline(t *testing.T) {
	if os.Getenv("NODEBRIDGE_OWNED_CDC_FIXTURE") != "1" {
		t.Skip("run scripts/test-canal-business-pipeline.ps1")
	}
	root, err := filepath.Abs(os.Getenv("NODEBRIDGE_CDC_FIXTURE_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	allowed, _ := filepath.Abs("../../.cache/canal-business")
	if !strings.HasPrefix(strings.ToLower(root), strings.ToLower(allowed)+string(os.PathSeparator)) {
		t.Fatal("fixture root must be isolated under .cache/canal-business")
	}
	my, err := mysql.ParseDSN(os.Getenv("NODEBRIDGE_CDC_TEST_DSN"))
	if err != nil {
		t.Fatal("invalid fixture DSN")
	}
	host, portText, err := net.SplitHostPort(my.Addr)
	broker := os.Getenv("NODEBRIDGE_CDC_TEST_RABBITMQ_URL")
	b, brokerErr := url.Parse(broker)
	canal := os.Getenv("NODEBRIDGE_CDC_TEST_CANAL_ADDR")
	canalHost, _, canalErr := net.SplitHostPort(canal)
	if err != nil || my.Net != "tcp" || my.DBName != "" || host != "127.0.0.1" || brokerErr != nil || b.Hostname() != "127.0.0.1" || canalErr != nil || canalHost != "127.0.0.1" {
		t.Fatal("only empty-database loopback fixtures are accepted")
	}
	port, _ := strconv.Atoi(portText)
	my.ParseTime = true
	my.Timeout, my.ReadTimeout, my.WriteTimeout = 3*time.Second, 5*time.Second, 5*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	admin, err := sql.Open("mysql", my.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	openDB := func(name, mode string) *sql.DB {
		t.Helper()
		if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"`"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()
			if _, err := admin.ExecContext(cleanup, "DROP DATABASE `"+name+"`"); err != nil {
				t.Errorf("cleanup %s: %v", name, err)
			}
		})
		cfg := *my
		cfg.DBName = name
		db, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/"+mode); err != nil {
			t.Fatal(err)
		}
		return db
	}
	sourceMode, targetMode := "edge", "server"
	direction := os.Getenv("NODEBRIDGE_CDC_DIRECTION")
	if direction == rules.DirectionServerToEdge {
		sourceMode, targetMode = "server", "edge"
	} else if direction != "" && direction != rules.DirectionEdgeToServer {
		t.Fatal("unsupported fixture direction")
	}
	source := openDB("nb_cdc_source", sourceMode)
	target := openDB("nb_cdc_target", targetMode)
	sqlExec := func(db *sql.DB, query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	sqlExec(source, "CREATE TABLE source_rows (source_id BIGINT UNSIGNED PRIMARY KEY, amount DECIMAL(30,6) NOT NULL, optional_text VARCHAR(30) NULL, raw_bytes VARBINARY(20) NOT NULL, changed_at DATETIME(6) NOT NULL, last_event_id VARCHAR(64) NOT NULL DEFAULT '', updated_by_node VARCHAR(64) NOT NULL DEFAULT '') ENGINE=InnoDB")
	sqlExec(target, "CREATE TABLE target_rows (target_id BIGINT UNSIGNED PRIMARY KEY, target_amount DECIMAL(30,6) NOT NULL, target_text VARCHAR(30) NULL, target_bytes VARBINARY(20) NOT NULL, target_time DATETIME(6) NOT NULL, last_event_id VARCHAR(64) NOT NULL DEFAULT '', updated_by_node VARCHAR(64) NOT NULL DEFAULT '') ENGINE=InnoDB")
	conn, err := amqp091.Dial(broker)
	if err != nil {
		t.Fatal("owned broker unavailable")
	}
	t.Cleanup(func() { conn.Close() })
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ch.Close() })
	for _, topology := range []rabbitmq.Topology{rabbitmq.EdgeTopology(), rabbitmq.ServerTopology([]string{"owned-cdc-edge"})} {
		if err := rabbitmq.InitializeTopology(ch, topology); err != nil {
			t.Fatal(err)
		}
	}
	set := rules.RuleSet{Rules: []rules.SyncRule{{ID: "owned-cdc-rule", Enable: true, DatabaseName: "nb_cdc_source", TableName: "source_rows", TargetDatabaseName: "nb_cdc_target", TargetTableName: "target_rows", PrimaryKeys: []string{"source_id"}, Direction: rules.DirectionEdgeToServer, DispatchTarget: rules.DispatchNone, ConflictPolicy: rules.ConflictNone, DeleteMode: rules.DeleteHard, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "source_id", TargetColumn: "target_id"}, {SourceColumn: "amount", TargetColumn: "target_amount"}, {SourceColumn: "optional_text", TargetColumn: "target_text"}, {SourceColumn: "raw_bytes", TargetColumn: "target_bytes"}, {SourceColumn: "changed_at", TargetColumn: "target_time"}}}}}
	paths := map[string]string{}
	if sourceMode == "server" {
		set.Rules[0].Direction = rules.DirectionServerToEdge
		set.Rules[0].DispatchTarget = rules.DispatchSelectedEdges
		set.Rules[0].DispatchNodeIDs = []string{"owned-cdc-edge"}
	}
	for _, mode := range []string{"edge", "server"} {
		dir := filepath.Join(root, mode)
		cfg := appconfig.Config{Mode: mode, Node: appconfig.NodeConfig{ID: "owned-cdc-" + mode}, MySQL: appconfig.MySQLConfig{Host: host, Port: port, Username: my.User, Password: my.Passwd, Database: "nb_cdc_target"}, RabbitMQ: appconfig.RabbitMQConfig{Mode: "external", LocalURL: broker, ServerURL: broker}, Sync: appconfig.SyncConfig{UploadBatchSize: 16, DispatchBatchSize: 16, FlushIntervalMillis: 50, RetryIntervalSeconds: 1}}
		cfg.MCP.Enable = true
		if mode == sourceMode {
			cfg.MySQL.Database = "nb_cdc_source"
			cfg.CDC = appconfig.CDCConfig{Type: "canal", Mode: "external", CanalAddr: canal, Destination: "example", ReaderName: "owned-cdc-reader", Filter: `nb_cdc_source\.source_rows`, BatchSize: 16}
		}
		path := filepath.Join(dir, "config.yaml")
		if err := appconfig.SaveFile(path, cfg); err != nil {
			t.Fatal(err)
		}
		if err := rules.SaveFile(filepath.Join(dir, "rules.yaml"), set); err != nil {
			t.Fatal(err)
		}
		paths[mode] = path
	}
	candidate := filepath.Join(root, "SyncAgent.exe")
	start := func(mode string) func() {
		t.Helper()
		dir := filepath.Dir(paths[mode])
		stopFile := filepath.Join(dir, "stop.request")
		log, err := os.Create(filepath.Join(dir, fmt.Sprintf("process-%d.log", time.Now().UnixNano())))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, candidate, "run", "-config", paths[mode], "-rules", filepath.Join(dir, "rules.yaml"), "-stop-file", stopFile)
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
				_ = os.WriteFile(stopFile, []byte("owned test stop"), 0600)
				select {
				case err := <-done:
					if err != nil {
						t.Errorf("%s agent exited: %v; logs at %s", mode, err, dir)
					}
				case <-time.After(8 * time.Second):
					_ = cmd.Process.Kill()
					<-done
					t.Errorf("%s agent needed forced cleanup", mode)
				}
				log.Close()
			})
		}
		t.Cleanup(stop)
		return stop
	}
	stopServer := start("server")
	stopEdge := start("edge")
	wait := func(label string, condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) && ctx.Err() == nil {
			if condition() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("%s timed out; owned logs at %s", label, root)
	}
	// Warm-up INSERTs avoid assuming that an open TCP port proves CDC readiness.
	warm := uint64(1)
	wait("Canal capture readiness", func() bool {
		_, err := source.ExecContext(ctx, "INSERT INTO source_rows (source_id,amount,optional_text,raw_bytes,changed_at) VALUES (?,1,NULL,X'00','2026-09-11 08:09:10.123456')", warm)
		if err != nil {
			return false
		}
		warm++
		var count int
		return target.QueryRowContext(ctx, "SELECT COUNT(*) FROM target_rows").Scan(&count) == nil && count > 0
	})
	sqlExec(source, "DELETE FROM source_rows")
	wait("warm-up deletes", func() bool {
		var count int
		return target.QueryRowContext(ctx, "SELECT COUNT(*) FROM target_rows").Scan(&count) == nil && count == 0
	})
	const key uint64 = 9007199254740993
	const amount = "123456789012345678901234.123456"
	sqlExec(source, "INSERT INTO source_rows (source_id,amount,optional_text,raw_bytes,changed_at) VALUES (?,?,NULL,X'00017F80FF','2026-09-11 08:09:10.123456')", key, amount)
	lastValue := ""
	checkValue := func(wantText sql.NullString, wantAmount, wantBytes string) bool {
		var gotAmount, gotBytes, gotTime string
		var gotText sql.NullString
		err := target.QueryRowContext(ctx, "SELECT CAST(target_amount AS CHAR),target_text,HEX(target_bytes),DATE_FORMAT(target_time,'%Y-%m-%d %H:%i:%s.%f') FROM target_rows WHERE target_id=?", key).Scan(&gotAmount, &gotText, &gotBytes, &gotTime)
		actual := fmt.Sprintf("amount=%s text=%+v bytes=%s time=%s error=%v", gotAmount, gotText, gotBytes, gotTime, err)
		if actual != lastValue {
			t.Log(actual)
			lastValue = actual
		}
		return err == nil && gotAmount == wantAmount && gotText == wantText && gotBytes == wantBytes && gotTime == "2026-09-11 08:09:10.123456"
	}
	wait("lossless mapped INSERT", func() bool { return checkValue(sql.NullString{}, amount, "00017F80FF") })
	stopEdge()
	stopEdge = start("edge")
	_ = stopEdge
	sqlExec(source, "UPDATE source_rows SET amount='0.000001',optional_text='',raw_bytes=X'00FF' WHERE source_id=?", key)
	wait("UPDATE after edge restart", func() bool { return checkValue(sql.NullString{String: "", Valid: true}, "0.000001", "00FF") })
	stopServer()
	stopServer = start("server")
	_ = stopServer
	sqlExec(source, "DELETE FROM source_rows WHERE source_id=?", key)
	wait("HARD DELETE after server restart", func() bool {
		var count int
		return target.QueryRowContext(ctx, "SELECT COUNT(*) FROM target_rows WHERE target_id=?", key).Scan(&count) == nil && count == 0
	})
	var receipts int
	if err := target.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log WHERE database_name='nb_cdc_source' AND table_name='source_rows' AND target_database_name='nb_cdc_target' AND target_table_name='target_rows' AND JSON_UNQUOTE(JSON_EXTRACT(pk_value,'$.target_id'))=?", strconv.FormatUint(key, 10)).Scan(&receipts); err != nil || receipts != 3 {
		t.Fatal("expected exactly three durable apply receipts", receipts, err)
	}
	const retainedKey uint64 = 9007199254740994
	sqlExec(target, "INSERT INTO target_rows (target_id,target_amount,target_text,target_bytes,target_time) VALUES (?,1,'remote',X'00','2026-09-11 08:09:10.123456')", retainedKey)
	remote := event.SyncEvent{EventID: "owned-retained-remote", OriginNodeID: "owned-cdc-" + targetMode, SourceNodeID: "owned-cdc-" + targetMode, DatabaseName: "owned_remote", TableName: "remote_rows", EventType: event.TypeInsert,
		PrimaryKey: map[string]any{"source_id": retainedKey}, After: map[string]any{"source_id": retainedKey, "amount": "1.000000", "optional_text": "remote", "raw_bytes": []byte{0}, "changed_at": "2026-09-11 08:09:10.123456", "last_event_id": "", "updated_by_node": ""}}
	remoteRule := rules.SyncRule{DatabaseName: remote.DatabaseName, TableName: remote.TableName, TargetDatabaseName: "nb_cdc_source", TargetTableName: "source_rows", PrimaryKeys: []string{"source_id"}, DeleteMode: rules.DeleteHard}
	mapped, err := mapper.MapEvent(remote, remoteRule)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := apply.NewCheckedSQLWorker(source).Apply(ctx, mapped); err != nil {
		t.Fatal(err)
	}
	sqlExec(source, "UPDATE source_rows SET optional_text='local-after-remote' WHERE source_id=?", retainedKey)
	wait("local UPDATE retaining prior remote markers", func() bool {
		var text string
		return target.QueryRowContext(ctx, "SELECT target_text FROM target_rows WHERE target_id=?", retainedKey).Scan(&text) == nil && text == "local-after-remote"
	})
	var retainedReceipts int
	if err := target.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log WHERE database_name='nb_cdc_source' AND table_name='source_rows' AND JSON_UNQUOTE(JSON_EXTRACT(pk_value,'$.target_id'))=?", strconv.FormatUint(retainedKey, 10)).Scan(&retainedReceipts); err != nil || retainedReceipts != 1 {
		t.Fatalf("expected only the retained-marker local UPDATE to be applied: %d %v", retainedReceipts, err)
	}
	t.Log("PASS: real SQL Apply replay INSERT suppressed by Canal runtime; subsequent local UPDATE retaining its markers reached peer exactly once")
	remoteRule.Direction = rules.DirectionBidirectional
	remote.EventID, remote.EventType = "owned-retained-delete", event.TypeDelete
	remote.Before, remote.After = remote.After, nil
	remote.Before["optional_text"] = "local-after-remote"
	mappedDelete, err := mapper.MapEvent(remote, remoteRule)
	if err != nil {
		t.Fatal(err)
	}
	sqlExec(source, "CREATE TRIGGER reject_delete_receipt BEFORE INSERT ON sync_apply_log FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned receipt rollback'")
	if _, err := apply.NewCheckedSQLWorker(source).Apply(ctx, mappedDelete); err == nil {
		t.Fatal("injected delete receipt failure was accepted")
	}
	var marker string
	if err := source.QueryRowContext(ctx, "SELECT last_event_id FROM source_rows WHERE source_id=?", retainedKey).Scan(&marker); err != nil || marker != "owned-retained-remote" {
		t.Fatalf("delete or marker survived rollback: %s %v", marker, err)
	}
	var proofs int
	if err := source.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_delete_replay WHERE event_id=?", remote.EventID).Scan(&proofs); err != nil || proofs != 0 {
		t.Fatalf("delete proof survived rollback: %d %v", proofs, err)
	}
	sqlExec(source, "DROP TRIGGER reject_delete_receipt")
	sqlExec(target, "DELETE FROM target_rows WHERE target_id=?", retainedKey)
	if _, err := apply.NewCheckedSQLWorker(source).Apply(ctx, mappedDelete); err != nil {
		t.Fatal(err)
	}
	const barrierKey uint64 = retainedKey + 1
	sqlExec(source, "INSERT INTO source_rows (source_id,amount,optional_text,raw_bytes,changed_at) VALUES (?,1,'after-delete',X'00','2026-09-11 08:09:10.123456')", barrierKey)
	wait("CDC crossed tracked delete", func() bool {
		var count int
		return target.QueryRowContext(ctx, "SELECT COUNT(*) FROM target_rows WHERE target_id=?", barrierKey).Scan(&count) == nil && count == 1
	})
	if err := target.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log WHERE database_name='nb_cdc_source' AND table_name='source_rows' AND JSON_UNQUOTE(JSON_EXTRACT(pk_value,'$.target_id'))=?", strconv.FormatUint(retainedKey, 10)).Scan(&retainedReceipts); err != nil || retainedReceipts != 1 {
		t.Fatalf("tracked delete echoed as business event: %d %v", retainedReceipts, err)
	}
	if err := source.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_delete_replay WHERE event_id=?", remote.EventID).Scan(&proofs); err != nil || proofs != 1 {
		t.Fatalf("delete proof not committed: %d %v", proofs, err)
	}
	t.Log("PASS: tracked hard delete stamp/proof/business deletion/receipt rollback and retry; real Canal suppressed marker UPDATE and DELETE, confirmed by later captured barrier row; BIDIRECTIONAL runtime gate remains closed")
	const localDeleteKey uint64 = retainedKey + 2
	sqlExec(target, "INSERT INTO target_rows (target_id,target_amount,target_text,target_bytes,target_time) VALUES (?,1,'remote',X'00','2026-09-11 08:09:10.123456')", localDeleteKey)
	seed := event.SyncEvent{EventID: "owned-local-delete-seed", OriginNodeID: "owned-cdc-" + targetMode, SourceNodeID: "owned-cdc-" + targetMode, DatabaseName: "owned_remote", TableName: "remote_rows", EventType: event.TypeInsert, PrimaryKey: map[string]any{"source_id": localDeleteKey}, After: map[string]any{"source_id": localDeleteKey, "amount": "1.000000", "optional_text": "remote", "raw_bytes": []byte{0}, "changed_at": "2026-09-11 08:09:10.123456", "last_event_id": "", "updated_by_node": ""}}
	seedMapped, err := mapper.MapEvent(seed, remoteRule)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := apply.NewCheckedSQLWorker(source).Apply(ctx, seedMapped); err != nil {
		t.Fatal(err)
	}
	sqlExec(source, "DELETE FROM source_rows WHERE source_id=?", localDeleteKey)
	wait("local hard DELETE retaining old INSERT marker", func() bool {
		var count int
		return target.QueryRowContext(ctx, "SELECT COUNT(*) FROM target_rows WHERE target_id=?", localDeleteKey).Scan(&count) == nil && count == 0
	})
	if err := target.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log WHERE op_type='DELETE' AND database_name='nb_cdc_source' AND table_name='source_rows' AND JSON_UNQUOTE(JSON_EXTRACT(pk_value,'$.target_id'))=?", strconv.FormatUint(localDeleteKey, 10)).Scan(&retainedReceipts); err != nil || retainedReceipts != 1 {
		t.Fatalf("local delete retaining INSERT marker did not reach peer exactly once: %d %v", retainedReceipts, err)
	}
	t.Log("PASS: local hard DELETE with retained remote INSERT marker reached peer exactly once, not mistaken for replay")
	t.Logf("PASS: candidate processes, Canal -> RabbitMQ -> checked Apply -> MySQL; mapped BIGINT/DECIMAL/NULL/empty/binary/time CRUD and edge/server restart; receipts=%d", receipts)
	mcp := exec.CommandContext(ctx, "node", "../../scripts/test-mcp-package-version.mjs", candidate, buildinfo.Version, filepath.Join(root, "package-mcp-version.json"), paths["server"], filepath.Join(root, "server", "rules.yaml"))
	mcp.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := mcp.CombinedOutput(); err != nil {
		t.Fatalf("candidate MCP failed: %v: %s", err, output)
	} else {
		t.Log(string(output))
	}
	stopServer()
	stopEdge()
	verifyOwnedCaptureFence(t, ctx, source, canal)
}
