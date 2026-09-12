package conflict_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// Serial SQL fixture: proves projections against observed schemas, not CDC routing.
func verifySQLBidirectionalPair(t *testing.T, ctx context.Context, db *sql.DB, database string) {
	t.Helper()
	for _, statement := range []string{
		"CREATE TABLE pair_edge (edge_id BIGINT UNSIGNED PRIMARY KEY,edge_value DECIMAL(30,10),raw_bytes VARBINARY(8),last_event_id VARCHAR(128),updated_by_node VARCHAR(64),computed BIGINT AS (1) VIRTUAL) ENGINE=InnoDB",
		"CREATE TABLE pair_server (server_id BIGINT UNSIGNED PRIMARY KEY,server_value DECIMAL(30,10),raw_bytes VARBINARY(8),last_event_id VARCHAR(128),updated_by_node VARCHAR(64),other_computed BIGINT AS (2) VIRTUAL) ENGINE=InnoDB",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	edge, err := rulecheck.ReadSchema(ctx, db, database, "pair_edge")
	if err != nil {
		t.Fatal(err)
	}
	server, err := rulecheck.ReadSchema(ctx, db, database, "pair_server")
	if err != nil {
		t.Fatal(err)
	}
	rule := rules.SyncRule{ID: "sql-pair", DatabaseName: database, TableName: edge.Table, TargetDatabaseName: database, TargetTableName: server.Table, PrimaryKeys: []string{"edge_id"}, TargetPrimaryKeys: []string{"server_id"}, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteHard, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "edge_value", TargetColumn: "server_value"}}}
	pair, err := rulecheck.BuildBidirectionalPair(rule, "pair-edge", "pair-server", edge, server)
	if err != nil {
		t.Fatal(err)
	}
	observations := []rulecheck.ObservedPair{{Rule: rule, EdgeNode: "pair-edge", ServerNode: "pair-server", Edge: edge, Server: server}}
	for _, node := range []string{"pair-edge", "pair-server"} {
		if err := rulecheck.VerifyLocalObservations(ctx, db, node, observations); err != nil {
			t.Fatal(err)
		}
	}
	stale := append([]rulecheck.ObservedPair(nil), observations...)
	stale[0].Edge.Engine = "MyISAM"
	if err := rulecheck.VerifyLocalObservations(ctx, db, "pair-edge", stale); err == nil {
		t.Fatal("accepted stale local schema")
	}
	worker := apply.NewCheckedSQLWorker(db)
	worker.CaptureFence = quietSQLFixtureFence{}
	for i, projection := range []rules.SyncRule{pair.Forward, pair.Reverse, pair.Relay} {
		key, value := "edge_id", "edge_value"
		if i == 1 {
			key, value = "server_id", "server_value"
		}
		row := map[string]any{key: "18446744073709551615", value: "12345678901234567890.1234567890", "raw_bytes": rowvalue.Binary{0, 128, 255}, "last_event_id": "", "updated_by_node": "", "computed": "1", "other_computed": "2"}
		evt := event.SyncEvent{EventID: []string{"pair-forward", "pair-reverse", "pair-relay"}[i], EventType: event.TypeInsert, OriginNodeID: projection.SourceNodeIDs[0], SourceNodeID: projection.SourceNodeIDs[0], DatabaseName: projection.DatabaseName, TableName: projection.TableName, PrimaryKey: map[string]any{key: row[key]}, After: row, EventTime: time.Unix(9000+int64(i), 0).UTC(), BinlogFile: "mysql-bin.000001", BinlogPos: uint32(100 + i), Headers: map[string]string{"event_time_source": conflict.SourceBinlog}}
		mapped, err := mapper.MapEvent(evt, projection)
		if err != nil {
			t.Fatal(err)
		}
		result, err := worker.Apply(ctx, mapped)
		if err != nil || result.ConflictDecision != conflict.Apply {
			t.Fatalf("projection %d: %+v %v", i, result, err)
		}
	}
	for _, table := range []struct{ name, value string }{{"pair_edge", "edge_value"}, {"pair_server", "server_value"}} {
		var amount, raw string
		if err := db.QueryRowContext(ctx, "SELECT CAST("+table.value+" AS CHAR),HEX(raw_bytes) FROM "+table.name).Scan(&amount, &raw); err != nil {
			t.Fatal(err)
		}
		if amount != "12345678901234567890.1234567890" || raw != "0080FF" {
			t.Fatalf("projection data loss: %s %s", amount, raw)
		}
	}
}
