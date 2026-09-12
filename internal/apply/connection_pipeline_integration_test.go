package apply_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncruntime"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
	"github.com/rabbitmq/amqp091-go"
)

func connectionPerfIngress(t *testing.T, ctx context.Context, db *sql.DB, name, url string, worker *apply.SQLWorker, events []mapper.MappedEvent) syncruntime.Stepper {
	t.Helper()
	conn, err := amqp091.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ch.Close() })
	queue, err := ch.QueueDeclare(name, true, false, false, false, amqp091.Table{"x-expires": int32(120000)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = ch.QueueDelete(queue.Name, false, false, false) })
	pub, err := rabbitmq.NewPublisher(ch)
	if err != nil {
		t.Fatal(err)
	}
	var requests []rabbitmq.PublishRequest
	for _, mapped := range events {
		body, err := rabbitmq.EncodeJSON(mapped.Event)
		if err != nil {
			t.Fatal(err)
		}
		requests = append(requests, rabbitmq.PublishRequest{RoutingKey: queue.Name, Body: body})
	}
	if err := pub.PublishBatch(ctx, requests); err != nil {
		t.Fatal(err)
	}
	set := &rules.RuleSet{}
	for _, table := range []string{"perf_stream", "perf_state"} {
		mode := rules.SyncModeAppendOnly
		if table == "perf_state" {
			mode = rules.SyncModeOrderedCRUD
		}
		direction := rules.DirectionEdgeToServer
		if os.Getenv("NODEBRIDGE_APPLY_PIPELINE_DIRECTION") == "downlink" {
			direction = rules.DirectionServerToEdge
		}
		set.Rules = append(set.Rules, rules.SyncRule{DatabaseName: name, TableName: table, TargetDatabaseName: name, TargetTableName: table, Enable: true, PrimaryKeys: []string{"id"}, Direction: direction, DispatchTarget: rules.DispatchNone, SyncMode: mode})
	}
	if os.Getenv("NODEBRIDGE_APPLY_PIPELINE_DIRECTION") == "downlink" {
		var selected apply.Worker = worker
		if os.Getenv("NODEBRIDGE_APPLY_PIPELINE_LEGACY") == "1" {
			selected = connectionSingleWorker{worker}
		}
		return &syncruntime.EdgeDownlinkBatchRuntime{Source: syncruntime.AMQPBatchGetSource{Channel: ch, Queue: queue.Name}, Worker: selected, Rules: set, MaxBatch: 50}
	}
	return &syncruntime.ServerIngressBatchRuntime{Source: syncruntime.AMQPBatchGetSource{Channel: ch, Queue: queue.Name}, Worker: worker, Rules: set, EventStore: syncstore.New(db), MaxBatch: 50}
}

// Hide ApplyBatch to reproduce the old one-transaction-per-event path.
type connectionSingleWorker struct{ apply.Worker }

func verifyConnectionValues(t *testing.T, ctx context.Context, db *sql.DB, events []mapper.MappedEvent) {
	t.Helper()
	latest := make(map[string]mapper.MappedEvent)
	for _, evt := range events {
		latest[fmt.Sprint(evt.TargetTable, "/", evt.TargetPrimaryKey["id"])] = evt
	}
	for _, table := range []string{"perf_stream", "perf_state"} {
		rows, err := db.QueryContext(ctx, "SELECT * FROM "+table)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer rows.Close()
			columns, err := rows.Columns()
			if err != nil || len(columns) != 50 {
				t.Fatalf("columns=%d err=%v", len(columns), err)
			}
			for rows.Next() {
				values := make([]any, len(columns))
				dest := make([]any, len(columns))
				for i := range values {
					dest[i] = &values[i]
				}
				if err := rows.Scan(dest...); err != nil {
					t.Fatal(err)
				}
				key := fmt.Sprint(table, "/", values[0])
				evt, ok := latest[key]
				if !ok {
					t.Fatalf("unexpected row %s", key)
				}
				delete(latest, key)
				for i, column := range columns {
					want := fmt.Sprint(evt.TargetAfter[column])
					if column == "last_event_id" {
						want = evt.Event.EventID
					}
					if column == "updated_by_node" {
						want = evt.Event.OriginNodeID
					}
					got := fmt.Sprint(values[i])
					switch v := values[i].(type) {
					case []byte:
						got = string(v)
					case time.Time:
						got = v.Format("2006-01-02 15:04:05.000")
					}
					if got != want {
						t.Fatalf("row %s column %s mismatch", key, column)
					}
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
		}()
	}
	if len(latest) != 0 {
		t.Fatalf("missing rows: %d", len(latest))
	}
}
