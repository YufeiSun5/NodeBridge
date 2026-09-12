package apply_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncruntime"
	"github.com/go-sql-driver/mysql"
	"github.com/rabbitmq/amqp091-go"
)

func TestBusinessSafetyRealRabbitMySQL(t *testing.T) {
	dsn, broker := os.Getenv("NODEBRIDGE_BUSINESS_SAFETY_TEST_DSN"), os.Getenv("NODEBRIDGE_REMEDIATION_RABBITMQ_URL")
	if dsn == "" || broker == "" {
		t.Skip("owned MySQL and RabbitMQ endpoints required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid fixture DSN")
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	endpoint, brokerErr := url.Parse(broker)
	if err != nil || cfg.Net != "tcp" || cfg.DBName != "" || host != "127.0.0.1" || brokerErr != nil || endpoint.Hostname() != "127.0.0.1" {
		t.Fatal("fixture requires explicit loopback endpoints and no existing database")
	}
	cfg.ParseTime = true
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 3*time.Second, 5*time.Second, 5*time.Second
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	name := fmt.Sprintf("nb_pipeline_fix_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"`"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE `"+name+"`"); err != nil {
			t.Errorf("owned database cleanup: %v", err)
		}
	}()
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/edge"); err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE TABLE target_rows (target_id BIGINT PRIMARY KEY, target_value INT NOT NULL) ENGINE=InnoDB")
	conn, err := amqp091.Dial(broker)
	if err != nil {
		t.Fatal("fixture broker connection failed")
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	queue, err := ch.QueueDeclare(name, true, false, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := ch.QueueDelete(queue.Name, false, false, false); err != nil {
			t.Errorf("owned queue cleanup: %v", err)
		}
	}()
	pub, err := rabbitmq.NewPublisher(ch)
	if err != nil {
		t.Fatal(err)
	}
	set := &rules.RuleSet{Rules: []rules.SyncRule{{ID: "owned-rule", Enable: true, DatabaseName: "owned_source", TableName: "source_rows", TargetDatabaseName: name, TargetTableName: "target_rows", PrimaryKeys: []string{"source_id"}, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "source_id", TargetColumn: "target_id"}, {SourceColumn: "source_value", TargetColumn: "target_value"}}, Direction: rules.DirectionServerToEdge, ConflictPolicy: rules.ConflictNone, DeleteMode: rules.DeleteHard}}}
	runtime := syncruntime.EdgeDownlinkBatchRuntime{Source: syncruntime.AMQPBatchGetSource{Channel: ch, Queue: queue.Name}, Worker: apply.NewCheckedSQLWorker(db), Consumer: rabbitmq.Consumer{RequeueOnError: true}, Rules: set, MaxBatch: 1, TargetDatabaseOverride: name + "_default"}
	const id int64 = 9007199254740993
	send := func(eid, op string, value int) {
		t.Helper()
		evt := event.SyncEvent{EventID: eid, OriginNodeID: "owned-server", SourceNodeID: "owned-server", DatabaseName: "owned_source", TableName: "source_rows", EventType: op, PrimaryKey: map[string]any{"source_id": id}, After: map[string]any{"source_id": id, "source_value": value}}
		body, err := json.Marshal(evt)
		if err != nil {
			t.Fatal(err)
		}
		if err := pub.Publish(ctx, rabbitmq.PublishRequest{RoutingKey: queue.Name, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	check := func(eid, failure string, wantReady, wantReceipts int) {
		t.Helper()
		_, err := runtime.RunOnce(ctx)
		if failure == "" && err != nil || failure != "" && (err == nil || !strings.Contains(err.Error(), failure)) {
			t.Fatalf("%s expected %q: %v", eid, failure, err)
		}
		var receipts int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log WHERE event_id=?", eid).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		actual, err := ch.QueueDeclarePassive(queue.Name, true, false, false, false, nil)
		if err != nil || actual.Messages != wantReady || receipts != wantReceipts {
			t.Fatalf("%s ready=%d receipt=%d expected=%d/%d err=%v", eid, actual.Messages, receipts, wantReady, wantReceipts, err)
		}
	}
	send("missing-update", event.TypeUpdate, 2)
	check("missing-update", "target_row_missing", 1, 0)
	check("missing-update", "target_row_missing", 1, 0)
	exec("INSERT INTO target_rows VALUES (?,1)", id)
	check("missing-update", "", 0, 1)
	var value int
	if err := db.QueryRowContext(ctx, "SELECT target_value FROM target_rows WHERE target_id=?", id).Scan(&value); err != nil || value != 2 {
		t.Fatal("mapped large-key update failed", err)
	}
	send("missing-update", event.TypeUpdate, 99)
	check("missing-update", "", 0, 1)
	if err := db.QueryRowContext(ctx, "SELECT target_value FROM target_rows WHERE target_id=?", id).Scan(&value); err != nil || value != 2 {
		t.Fatal("duplicate event changed business row", err)
	}
	exec("CREATE TRIGGER fail_receipt BEFORE INSERT ON sync_apply_log FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned receipt failure'")
	send("receipt-failure", event.TypeUpdate, 3)
	check("receipt-failure", "owned receipt failure", 1, 0)
	if err := db.QueryRowContext(ctx, "SELECT target_value FROM target_rows WHERE target_id=?", id).Scan(&value); err != nil || value != 2 {
		t.Fatal("business write survived receipt rollback", err)
	}
	exec("DROP TRIGGER fail_receipt")
	check("receipt-failure", "", 0, 1)
	send("disabled-rule", event.TypeUpdate, 4)
	set.Rules[0].Enable = false
	check("disabled-rule", "rule", 1, 0)
	set.Rules[0].Enable = true
	check("disabled-rule", "", 0, 1)
	send("hard-delete", event.TypeDelete, 0)
	check("hard-delete", "", 0, 1)
	var rows int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM target_rows").Scan(&rows); err != nil || rows != 0 {
		t.Fatal("HARD delete did not remove mapped row", err)
	}
}
