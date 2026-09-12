package syncruntime

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/normalizer"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
	"github.com/go-sql-driver/mysql"
	"github.com/rabbitmq/amqp091-go"
)

// Opt-in: creates and removes only a uniquely named test database and queue.
func TestIntegrationServerDispatchPerformance(t *testing.T) {
	dsn, url := os.Getenv("NODEBRIDGE_DISPATCH_MYSQL_DSN"), os.Getenv("NODEBRIDGE_RABBITMQ_URL")
	if dsn == "" || url == "" {
		t.Skip("NODEBRIDGE_DISPATCH_MYSQL_DSN and NODEBRIDGE_RABBITMQ_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid test MySQL DSN")
	}
	cfg.DBName, cfg.ParseTime = "", true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("nb_dispatch_perf_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP DATABASE " + name); err != nil {
			t.Errorf("clean test database: %v", err)
		}
	}()
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/server"); err != nil {
		t.Fatal(err)
	}
	history, err := strconv.Atoi(os.Getenv("NODEBRIDGE_DISPATCH_HISTORY_ROWS"))
	if err != nil && os.Getenv("NODEBRIDGE_DISPATCH_HISTORY_ROWS") != "" {
		t.Fatal(err)
	}
	seedDispatchHistory(t, ctx, db, history)
	syncMode := os.Getenv("NODEBRIDGE_DISPATCH_SYNC_MODE")
	if syncMode == "" {
		syncMode = rules.SyncModeAppendOnly
	}
	if syncMode != rules.SyncModeAppendOnly && syncMode != rules.SyncModeOrderedCRUD {
		t.Fatal("NODEBRIDGE_DISPATCH_SYNC_MODE must be append_only or crud_ordered")
	}
	conn, err := amqp091.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	if err := ch.ExchangeDeclare(name, "direct", true, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	defer ch.ExchangeDelete(name, false, false)
	queue, err := ch.QueueDeclare(name, true, false, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ch.QueueDelete(queue.Name, false, false, false)
	if err := ch.QueueBind(queue.Name, "perf-edge.downlink", name, false, nil); err != nil {
		t.Fatal(err)
	}
	pub, err := rabbitmq.NewPublisher(ch)
	if err != nil {
		t.Fatal(err)
	}
	timedPub := &dispatchTimedPublisher{Publisher: pub}
	timedStore := &dispatchTimedStore{Store: syncstore.New(db)}
	const count = 1000
	mixed := os.Getenv("NODEBRIDGE_DISPATCH_MIXED") == "1"
	includeReplay := os.Getenv("NODEBRIDGE_DISPATCH_INCLUDE_REPLAY") == "1"
	replayCount := 0
	source := &fakeCanalBatchSource{offset: cdc.Offset{BatchID: 1}}
	for i := 0; i < count; i++ {
		change := cdc.ChangeEvent{
			DatabaseName: name, TableName: "perf_source", Operation: cdc.OperationInsert,
			PrimaryKey: map[string]any{"id": i}, After: map[string]any{"id": i, "payload": strings.Repeat("x", 128)},
			BinlogFile: "perf.000001", BinlogPos: uint32(i + 4), EventTime: time.Now(),
		}
		if os.Getenv("NODEBRIDGE_DISPATCH_WIDE_UPDATE") == "1" || mixed {
			if !mixed || i%60 >= 40 {
				change.Operation = cdc.OperationUpdate
			}
			if mixed && change.Operation == cdc.OperationUpdate {
				change.TableName = "perf_state"
			}
			for col := 0; col < 48; col++ {
				change.After[fmt.Sprintf("field_%02d", col)] = fmt.Sprintf("wide-%d-%d-%s", i, col, strings.Repeat("x", 24))
			}
		}
		source.changes = append(source.changes, change)
		if includeReplay && change.Operation == cdc.OperationUpdate {
			replayID := fmt.Sprintf("perf-replay-%d", i)
			if _, err := db.ExecContext(ctx, `INSERT INTO sync_apply_log
(event_id,origin_node_id,source_node_id,target_node_id,database_name,table_name,pk_value,op_type,applied_at)
VALUES (?,'perf-edge','perf-edge','perf-server',?,?,'1','UPDATE',NOW(3))`, replayID, name, change.TableName); err != nil {
				t.Fatal(err)
			}
			replay := change
			replay.After = make(map[string]any, len(change.After)+2)
			for key, value := range change.After {
				replay.After[key] = value
			}
			replay.After["last_event_id"], replay.After["updated_by_node"] = replayID, "perf-edge"
			source.changes = append(source.changes, replay)
			replayCount++
		}
	}
	set := &rules.RuleSet{Rules: []rules.SyncRule{{
		DatabaseName: name, TableName: "perf_source", TargetDatabaseName: name, TargetTableName: "perf_target",
		Enable: true, PrimaryKeys: []string{"id"}, Direction: rules.DirectionServerToEdge,
		DispatchTarget: rules.DispatchSelectedEdges, DispatchNodeIDs: []string{"perf-edge"},
		SyncMode: syncMode,
	}}}
	if mixed {
		set.Rules[0].SyncMode = rules.SyncModeAppendOnly
		stateRule := set.Rules[0]
		stateRule.TableName, stateRule.TargetTableName = "perf_state", "perf_state_target"
		stateRule.SyncMode = rules.SyncModeOrderedCRUD
		set.Rules = append(set.Rules, stateRule)
	}
	runtime := &ServerCanalDispatchRuntime{
		Source: source, Decider: loop.NewSuppressor("perf-server", *set, timedStore),
		Normalizer: normalizer.New(normalizer.Options{NodeID: "perf-server", SchemaVersion: 1}),
		Rules:      set, EventStore: timedStore,
		Dispatcher: RoutingDownlinkDispatcher{Publisher: timedPub, Exchange: name},
	}
	if os.Getenv("NODEBRIDGE_DISPATCH_FORCE_SINGLE") == "1" {
		runtime.Dispatcher = singleConfirmDispatcher{runtime.Dispatcher}
	}
	started := time.Now()
	result, err := runtime.RunOnce(ctx)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if !source.committed || result.DispatchCount != count {
		t.Fatalf("uncommitted or incomplete dispatch: %+v", result)
	}
	if timedStore.lookupCalls != replayCount {
		t.Fatalf("replay lookup count=%d want=%d", timedStore.lookupCalls, replayCount)
	}
	var logged int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_event_log WHERE table_name IN ('perf_source','perf_state') AND status='SUCCESS'").Scan(&logged); err != nil || logged != count {
		t.Fatalf("logged=%d err=%v", logged, err)
	}
	seen := make(map[string]bool)
	for i := 0; i < count; i++ {
		msg, ok, err := ch.Get(queue.Name, false)
		if err != nil || !ok {
			t.Fatalf("missing message %d: %v", i, err)
		}
		var evt event.SyncEvent
		if err := json.Unmarshal(msg.Body, &evt); err != nil || seen[evt.EventID] {
			t.Fatalf("invalid or duplicate message %d: %v", i, err)
		}
		if fmt.Sprint(evt.PrimaryKey["id"]) != strconv.Itoa(i) {
			t.Fatalf("message order changed at %d: %+v", i, evt.PrimaryKey)
		}
		if mixed {
			wantType, wantTable := event.TypeInsert, "perf_source"
			if i%60 >= 40 {
				wantType, wantTable = event.TypeUpdate, "perf_state"
			}
			if evt.EventType != wantType || evt.TableName != wantTable || len(evt.After) != 50 {
				t.Fatalf("mixed message %d: type=%s table=%s columns=%d", i, evt.EventType, evt.TableName, len(evt.After))
			}
		}
		seen[evt.EventID] = true
		if err := msg.Ack(false); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("mode=%s history=%d events=%d elapsed=%s rate=%.2f/s log_calls=%d log_time=%s publish_calls=%d publish_time=%s",
		syncMode, history, count, elapsed, float64(count)/elapsed.Seconds(), timedStore.calls, timedStore.elapsed, timedPub.calls, timedPub.elapsed)
	t.Logf("replays=%d replay_lookup_time=%s", replayCount, timedStore.lookupElapsed)
}

type singleConfirmDispatcher struct{ DownlinkDispatcher }

func seedDispatchHistory(t *testing.T, ctx context.Context, db *sql.DB, count int) {
	t.Helper()
	if count <= 0 {
		return
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sync_event_log
(event_id,origin_node_id,source_node_id,database_name,table_name,pk_value,op_type,direction,status,event_time,received_at,event_payload)
VALUES ('history-1','history','history','history','history','1','INSERT','SERVER_TO_EDGE','SUCCESS',NOW(3),NOW(3),REPEAT('x',512))`); err != nil {
		t.Fatal(err)
	}
	for rows := 1; rows < count; {
		add := min(rows, count-rows)
		_, err := db.ExecContext(ctx, `INSERT INTO sync_event_log
(event_id,origin_node_id,source_node_id,database_name,table_name,pk_value,op_type,direction,status,event_time,received_at,event_payload)
SELECT CONCAT('history-', ROW_NUMBER() OVER (ORDER BY id)+?),origin_node_id,source_node_id,database_name,table_name,pk_value,op_type,direction,status,event_time,received_at,event_payload
FROM sync_event_log ORDER BY id LIMIT ?`, rows, add)
		if err != nil {
			t.Fatal(err)
		}
		rows += add
	}
}

type dispatchTimedStore struct {
	*syncstore.Store
	calls         int
	elapsed       time.Duration
	lookupCalls   int
	lookupElapsed time.Duration
}

func (s *dispatchTimedStore) Exists(ctx context.Context, eventID string) (bool, error) {
	start := time.Now()
	found, err := s.Store.Exists(ctx, eventID)
	s.lookupCalls++
	s.lookupElapsed += time.Since(start)
	return found, err
}

func (s *dispatchTimedStore) UpsertEventLog(ctx context.Context, record syncstore.EventLogRecord) error {
	start := time.Now()
	err := s.Store.UpsertEventLog(ctx, record)
	s.calls++
	s.elapsed += time.Since(start)
	return err
}

func (s *dispatchTimedStore) UpsertEventLogs(ctx context.Context, records []syncstore.EventLogRecord) error {
	start := time.Now()
	err := s.Store.UpsertEventLogs(ctx, records)
	s.calls++
	s.elapsed += time.Since(start)
	return err
}

type dispatchTimedPublisher struct {
	*rabbitmq.Publisher
	calls   int
	elapsed time.Duration
}

func (p *dispatchTimedPublisher) Publish(ctx context.Context, req rabbitmq.PublishRequest) error {
	start := time.Now()
	err := p.Publisher.Publish(ctx, req)
	p.calls++
	p.elapsed += time.Since(start)
	return err
}

func (p *dispatchTimedPublisher) PublishBatch(ctx context.Context, reqs []rabbitmq.PublishRequest) error {
	start := time.Now()
	err := p.Publisher.PublishBatch(ctx, reqs)
	p.calls++
	p.elapsed += time.Since(start)
	return err
}
