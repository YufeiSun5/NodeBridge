package alignment

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
	"github.com/rabbitmq/amqp091-go"
)

func TestOwnedSnapshotAMQPTransport(t *testing.T) {
	if os.Getenv("NODEBRIDGE_OWNED_ALIGNMENT_TRANSPORT") != "1" {
		t.Skip("run scripts/test-alignment-transport.ps1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var dbs [2]*sql.DB
	databases := []string{fmt.Sprintf("nb_snapshot_source_%d", time.Now().UnixNano()), fmt.Sprintf("nb_snapshot_target_%d", time.Now().UnixNano())}
	nodes := []string{"owned-source", "owned-target"}
	for i, key := range []string{"NODEBRIDGE_ALIGNMENT_SOURCE_DSN", "NODEBRIDGE_ALIGNMENT_TARGET_DSN"} {
		cfg, err := mysql.ParseDSN(os.Getenv(key))
		if err != nil {
			t.Fatal("invalid owned fixture DSN")
		}
		host, _, err := net.SplitHostPort(cfg.Addr)
		if err != nil || cfg.Net != "tcp" || host != "127.0.0.1" || cfg.DBName != "" {
			t.Fatal("loopback fixture without an existing database required")
		}
		cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 3*time.Second, 5*time.Second, 5*time.Second
		cfg.InterpolateParams = true
		cfg.Params = map[string]string{"charset": "utf8mb4"}
		admin, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { admin.Close() })
		if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+databases[i]+"`"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()
			if _, err := admin.ExecContext(cleanup, "DROP DATABASE `"+databases[i]+"`"); err != nil {
				t.Error(err)
			}
		})
		cfg.DBName = databases[i]
		dbs[i], err = sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { dbs[i].Close() })
		if err := mysqlconn.RunMigrations(ctx, dbs[i], "../../migrations/"+[]string{"edge", "server"}[i]); err != nil {
			t.Fatal(err)
		}
	}
	var uuids [2]string
	for i, db := range dbs {
		if err := db.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&uuids[i]); err != nil {
			t.Fatal(err)
		}
	}
	if uuids[0] == uuids[1] {
		t.Fatal("two physically distinct MySQL servers required")
	}
	broker := os.Getenv("NODEBRIDGE_ALIGNMENT_BROKER_URL")
	parsed, err := url.Parse(broker)
	if err != nil || parsed.Scheme != "amqp" || parsed.Hostname() != "127.0.0.1" {
		t.Fatal("owned loopback RabbitMQ required")
	}
	execSQL := func(db *sql.DB, query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"forward", "reverse", "receipt_failure", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			tables := []string{"source_" + mode, "target_" + mode}
			keys := []string{"source_id", "target_id"}
			for i, db := range dbs {
				execSQL(db, "CREATE TABLE "+tables[i]+" ("+keys[i]+" BIGINT UNSIGNED PRIMARY KEY, amount DECIMAL(30,10), payload VARBINARY(8), note VARCHAR(20) NULL) ENGINE=InnoDB")
			}
			source, target := 0, 1
			if mode == "reverse" {
				source, target = 1, 0
			}
			execSQL(dbs[source], "INSERT INTO "+tables[source]+" VALUES (1,12345678901234567890.1234567890,X'0080FF',NULL),(18446744073709551615,-0.1234567890,X'','')")
			left, err := Observe(ctx, dbs[0], nodes[0], databases[0], tables[0])
			if err != nil {
				t.Fatal(err)
			}
			right, err := Observe(ctx, dbs[1], nodes[1], databases[1], tables[1])
			if err != nil {
				t.Fatal(err)
			}
			rule := rules.SyncRule{ID: mode, DatabaseName: databases[0], TableName: tables[0], TargetDatabaseName: databases[1], TargetTableName: tables[1], PrimaryKeys: []string{keys[0]}, TargetPrimaryKeys: []string{keys[1]}, DeleteMode: rules.DeleteHard, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}}
			plan, err := BuildPlan(rule, left, right, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			for i, db := range dbs {
				if _, err := PrepareSnapshotJob(ctx, db, plan, rule, nodes[i], true); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "receipt_failure" {
				execSQL(dbs[target], "CREATE TRIGGER owned_snapshot_receipt_failure BEFORE UPDATE ON sync_alignment_job FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned snapshot receipt failure'")
				defer func() { execSQL(dbs[target], "DROP TRIGGER owned_snapshot_receipt_failure") }()
			}
			sourceConn, err := amqp091.Dial(broker)
			if err != nil {
				t.Fatal(err)
			}
			defer sourceConn.Close()
			targetConn, err := amqp091.Dial(broker)
			if err != nil {
				t.Fatal(err)
			}
			defer targetConn.Close()
			copyCtx, stopCopy := context.WithTimeout(ctx, 8*time.Second)
			defer stopCopy()
			type received struct {
				result CopyResult
				err    error
			}
			done := make(chan received, 1)
			go func() {
				result, err := ReceiveSnapshotAMQP(copyCtx, dbs[target], targetConn, plan, rule, nodes[target], true)
				done <- received{result, err}
			}()
			defer func() {
				stopCopy()
				targetConn.Close()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("receiver cleanup timed out")
				}
			}()
			var sent CopyResult
			var sendErr error
			frames, replies := snapshotQueues(plan)
			inspection, err := sourceConn.Channel()
			if err != nil {
				t.Fatal(err)
			}
			defer inspection.Close()
			if mode == "truncated" {
				if err := declareSnapshotQueues(inspection, plan); err != nil {
					t.Fatal(err)
				}
				publisher, err := rabbitmq.NewPublisher(inspection)
				if err != nil {
					t.Fatal(err)
				}
				body, err := EncodeSnapshotFrame(SnapshotFrame{PlanID: plan.ID, Values: [][]byte{[]byte("1"), []byte("12345678901234567890.1234567890"), {0, 128, 255}, nil}})
				if err != nil {
					t.Fatal(err)
				}
				if err := publisher.Publish(copyCtx, rabbitmq.PublishRequest{RoutingKey: frames, Body: body, Headers: amqp091.Table{attemptHeader: strings.Repeat("a", 32)}}); err != nil {
					t.Fatal(err)
				}
				deadline := time.Now().Add(3 * time.Second)
				accepted := false
				for time.Now().Before(deadline) {
					q, err := inspection.QueueDeclarePassive(replies, true, false, false, false, nil)
					if err != nil {
						t.Fatal(err)
					}
					if q.Messages == 1 {
						accepted = true
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if !accepted {
					t.Fatal("row was not accepted before simulated disconnect")
				}
				stopCopy()
			} else {
				sent, sendErr = SendSnapshotAMQP(copyCtx, dbs[source], sourceConn, plan, rule, nodes[source], true)
			}
			var targetResult received
			select {
			case targetResult = <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("receiver did not finish")
			}
			// Return the delivered completion for the unconditional cleanup join.
			done <- targetResult
			success := mode == "forward" || mode == "reverse"
			if success && (sendErr != nil || targetResult.err != nil || sent != targetResult.result || sent.Rows != 2) {
				t.Fatalf("transfer: %+v %v %+v", sent, sendErr, targetResult)
			}
			if !success && targetResult.err == nil {
				t.Fatal("failed transfer reported success")
			}
			if mode == "receipt_failure" && (sendErr == nil || !strings.Contains(targetResult.err.Error(), "1644")) {
				t.Fatalf("receipt failure not exercised: %v %v", sendErr, targetResult.err)
			}
			var count int
			if err := dbs[target].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tables[target]).Scan(&count); err != nil {
				t.Fatal(err)
			}
			wantRows, wantMessages := 0, 3
			if success {
				wantRows, wantMessages = 2, 0
			}
			if mode == "truncated" {
				wantMessages = 1
			}
			if count != wantRows {
				t.Fatalf("partial target rows: %d want %d", count, wantRows)
			}
			q, err := inspection.QueueDeclarePassive(frames, true, false, false, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			if q.Messages != wantMessages {
				t.Fatalf("input ACK boundary: %d pending want %d", q.Messages, wantMessages)
			}
			job, err := ReadSnapshotJob(ctx, dbs[target], plan, nodes[target])
			if err != nil {
				t.Fatal(err)
			}
			if success && (job.Phase != JobTargetCommitted || job.Result == nil || *job.Result != sent) {
				t.Fatal("commit receipt missing", job)
			}
			if !success && job.Phase != JobPrepared {
				t.Fatal("failed copy advanced job", job)
			}
			for _, db := range dbs {
				if err := CheckPendingJobs(ctx, db); err == nil {
					t.Fatal("transport incorrectly activated CDC")
				}
			}
		})
	}
	t.Log("PASS: distinct MySQL servers and separate RabbitMQ connections; lossless mapped copies in both directions; receipt failure and disconnect retain unacked input and roll back target; no CDC cutover or public operation claimed")
}
