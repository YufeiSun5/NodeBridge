package rulecheck_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func pairFixture() (rules.SyncRule, rulecheck.Schema, rulecheck.Schema) {
	r := rules.SyncRule{ID: "pair", DatabaseName: "edge_db", TableName: "source_rows", TargetDatabaseName: "server_db", TargetTableName: "target_rows", Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteHard, PrimaryKeys: []string{"id"}, TargetPrimaryKeys: []string{"row_id"}, SourceNodeIDs: []string{"edge-1", "edge-2"}, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "value", TargetColumn: "payload"}}}
	e := rulecheck.Schema{Database: r.DatabaseName, Table: r.TableName, Engine: "InnoDB", PrimaryKeys: []string{"id"}, Columns: []rulecheck.Column{{Name: "id", Type: "bigint unsigned"}, {Name: "value", Type: "varbinary(32)", Nullable: true}, {Name: "last_event_id", Type: "varchar(128)", Collation: "utf8mb4_bin"}, {Name: "updated_by_node", Type: "varchar(64)", Collation: "utf8mb4_bin"}}}
	s := e
	s.Database, s.Table, s.PrimaryKeys = r.TargetDatabaseName, r.TargetTableName, []string{"row_id"}
	s.Columns = append([]rulecheck.Column(nil), e.Columns...)
	s.Columns[0].Name, s.Columns[1].Name = "row_id", "payload"
	return r, e, s
}

func TestBidirectionalPairRoundTripAndRelay(t *testing.T) {
	r, edge, server := pairFixture()
	before, _ := json.Marshal(r)
	pair, err := rulecheck.BuildBidirectionalPair(r, "edge-1", "server-1", edge, server)
	if err != nil {
		t.Fatal(err)
	}
	key := json.Number("18446744073709551615")
	evt := event.SyncEvent{EventID: "original-id", EventType: event.TypeInsert, OriginNodeID: "edge-1", SourceNodeID: "edge-1", DatabaseName: edge.Database, TableName: edge.Table, PrimaryKey: map[string]any{"id": key}, After: map[string]any{"id": key, "value": rowvalue.Binary{0, 128, 255}, "last_event_id": "", "updated_by_node": ""}}
	forward, err := mapper.MapEvent(evt, pair.Forward)
	if err != nil {
		t.Fatal(err)
	}
	if forward.TargetDatabase != server.Database || forward.TargetTable != server.Table || forward.TargetPrimaryKey["row_id"] != key {
		t.Fatalf("forward: %+v", forward)
	}
	reverse, err := mapper.MapEvent(forward.Event, pair.Reverse)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reverse.Event, evt) {
		t.Fatalf("round trip changed event: %+v", reverse.Event)
	}
	relay, err := mapper.MapEvent(evt, pair.Relay)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(relay.Event, evt) {
		t.Fatalf("relay remapped edge event: %+v", relay.Event)
	}
	if !reflect.DeepEqual(pair.Reverse.SourceNodeIDs, []string{"server-1"}) || !reflect.DeepEqual(pair.Reverse.DispatchNodeIDs, []string{"edge-1"}) {
		t.Fatalf("reverse widened scope: %+v", pair.Reverse)
	}
	pair.Forward.PrimaryKeys[0] = "mutated"
	pair.Reverse.DispatchNodeIDs[0] = "mutated"
	after, _ := json.Marshal(r)
	if string(before) != string(after) || pair.Relay.PrimaryKeys[0] != "id" || pair.Forward.DispatchNodeIDs[0] != "edge-1" {
		t.Fatal("projections share mutable slices")
	}
}

func TestBidirectionalPairRejectsUnsafeProjection(t *testing.T) {
	cases := map[string]func(*rules.SyncRule, *rulecheck.Schema, *rulecheck.Schema){
		"wrong direction":      func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.Direction = rules.DirectionEdgeToServer },
		"unsupported conflict": func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.ConflictPolicy = rules.ConflictServerWin },
		"wrong scope":          func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.SourceNodeIDs = []string{"another-edge"} },
		"no downlink":          func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.DispatchTarget = rules.DispatchNone },
		"excluded receiver": func(r *rules.SyncRule, e, s *rulecheck.Schema) {
			r.DispatchTarget = rules.DispatchSelectedEdges
			r.DispatchNodeIDs = []string{"edge-2"}
		},
		"append only":     func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.SyncMode = rules.SyncModeAppendOnly },
		"compaction":      func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.SyncMode = rules.SyncModeCRUDCompact },
		"ddl":             func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.SchemaSync.AddColumns = true },
		"wrong database":  func(r *rules.SyncRule, e, s *rulecheck.Schema) { s.Database = "wrong" },
		"engine":          func(r *rules.SyncRule, e, s *rulecheck.Schema) { e.Engine = "MyISAM" },
		"actual key":      func(r *rules.SyncRule, e, s *rulecheck.Schema) { s.PrimaryKeys = []string{"payload"} },
		"partial include": func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.IncludeColumns = []string{"id"} },
		"excluded column": func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.ExcludeColumns = []string{"value"} },
		"implicit collision": func(r *rules.SyncRule, e, s *rulecheck.Schema) {
			e.Columns = append(e.Columns, rulecheck.Column{Name: "payload", Type: "varbinary(32)", Nullable: true})
		},
		"unmapped target": func(r *rules.SyncRule, e, s *rulecheck.Schema) {
			s.Columns = append(s.Columns, rulecheck.Column{Name: "extra", Type: "int", Nullable: true})
		},
		"missing target":   func(r *rules.SyncRule, e, s *rulecheck.Schema) { s.Columns[1].Name = "missing" },
		"precision":        func(r *rules.SyncRule, e, s *rulecheck.Schema) { s.Columns[0].Type = "bigint" },
		"collation":        func(r *rules.SyncRule, e, s *rulecheck.Schema) { s.Columns[2].Collation = "utf8mb4_general_ci" },
		"nullability":      func(r *rules.SyncRule, e, s *rulecheck.Schema) { s.Columns[1].Nullable = false },
		"generated key":    func(r *rules.SyncRule, e, s *rulecheck.Schema) { e.Columns[0].Extra = "STORED GENERATED" },
		"generated target": func(r *rules.SyncRule, e, s *rulecheck.Schema) { s.Columns[1].Extra = "STORED GENERATED" },
		"unknown mapping": func(r *rules.SyncRule, e, s *rulecheck.Schema) {
			r.ColumnMappings = append(r.ColumnMappings, rules.ColumnMapping{SourceColumn: "missing", TargetColumn: "other"})
		},
		"unknown selector":        func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.ExcludeColumns = []string{"missing"} },
		"missing marker":          func(r *rules.SyncRule, e, s *rulecheck.Schema) { e.Columns = e.Columns[:3]; s.Columns = s.Columns[:3] },
		"soft missing columns":    func(r *rules.SyncRule, e, s *rulecheck.Schema) { r.DeleteMode = rules.DeleteSoft },
		"duplicate schema column": func(r *rules.SyncRule, e, s *rulecheck.Schema) { e.Columns = append(e.Columns, e.Columns[0]) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r, e, s := pairFixture()
			mutate(&r, &e, &s)
			if _, err := rulecheck.BuildBidirectionalPair(r, "edge-1", "server-1", e, s); err == nil {
				t.Fatal("unsafe pair accepted")
			}
		})
	}
	for _, nodes := range [][2]string{{"", "server-1"}, {"edge-1", ""}, {"edge-1", "edge-1"}, {"edge-1", "invalid.node"}} {
		r, e, s := pairFixture()
		if _, err := rulecheck.BuildBidirectionalPair(r, nodes[0], nodes[1], e, s); err == nil {
			t.Fatalf("invalid nodes: %v", nodes)
		}
	}
}

func TestBidirectionalPairDoesNotActivateRuntime(t *testing.T) {
	r, e, s := pairFixture()
	r.Enable = true
	pair, err := rulecheck.BuildBidirectionalPair(r, "edge-1", "server-1", e, s)
	if err != nil {
		t.Fatal(err)
	}
	for _, projection := range []rules.SyncRule{pair.Forward, pair.Reverse, pair.Relay} {
		if err := (rules.RuleSet{Rules: []rules.SyncRule{projection}}).Validate(); err == nil {
			t.Fatal("runtime gate was removed")
		}
	}
	if !r.Enable {
		t.Fatal("input enable changed")
	}
}

func TestBidirectionalPairSameNamesCompoundKeyAndGeneratedColumns(t *testing.T) {
	r, e, _ := pairFixture()
	r.TargetDatabaseName, r.TargetTableName, r.TargetPrimaryKeys = "", "", nil
	r.ColumnMappings = nil
	r.PrimaryKeys = []string{"value", "id"}
	e.PrimaryKeys = []string{"value", "id"}
	e.Columns[1].Nullable = false
	e.Columns = append(e.Columns, rulecheck.Column{Name: "calculated", Type: "bigint", Extra: "VIRTUAL GENERATED"})
	s := e
	s.Columns = append([]rulecheck.Column(nil), e.Columns...)
	s.Columns[len(s.Columns)-1].Name = "different_calculated"
	p, err := rulecheck.BuildBidirectionalPair(r, "edge-1", "server-1", e, s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Reverse.PrimaryKeys, []string{"value", "id"}) {
		t.Fatal("key order lost")
	}
	for _, rule := range []rules.SyncRule{p.Forward, p.Reverse, p.Relay} {
		evt := event.SyncEvent{EventType: event.TypeInsert, DatabaseName: e.Database, TableName: e.Table, PrimaryKey: map[string]any{"value": "k", "id": 1}, After: map[string]any{"value": "k", "id": 1, "last_event_id": "", "updated_by_node": "", "calculated": 2, "different_calculated": 3}}
		m, err := mapper.MapEvent(evt, rule)
		if err != nil {
			t.Fatal(err)
		}
		if len(m.TargetAfter) != 4 {
			t.Fatalf("generated columns leaked: %v", m.TargetAfter)
		}
	}
	s.PrimaryKeys = []string{"id", "value"}
	if _, err := rulecheck.BuildBidirectionalPair(r, "edge-1", "server-1", e, s); err == nil {
		t.Fatal("changed actual composite key order accepted")
	}
}

func TestBidirectionalPairSoftDeleteBothEndpoints(t *testing.T) {
	r, e, s := pairFixture()
	r.DeleteMode = rules.DeleteSoft
	columns := []rulecheck.Column{{Name: "is_deleted", Type: "tinyint"}, {Name: "deleted_at", Type: "datetime(6)", Nullable: true}, {Name: "deleted_by_node", Type: "varchar(64)", Nullable: true, Collation: "utf8mb4_bin"}}
	e.Columns = append(e.Columns, columns...)
	s.Columns = append(s.Columns, columns...)
	p, err := rulecheck.BuildBidirectionalPair(r, "edge-1", "server-1", e, s)
	if err != nil {
		t.Fatal(err)
	}
	if p.Reverse.EffectiveDeleteMode() != rules.DeleteSoft {
		t.Fatal("soft delete lost")
	}
	// An alias cannot remove the reserved replay fields on either endpoint.
	r.ColumnMappings = append(r.ColumnMappings, rules.ColumnMapping{SourceColumn: "updated_by_node", TargetColumn: "writer"})
	s.Columns[3].Name = "writer"
	if _, err := rulecheck.BuildBidirectionalPair(r, "edge-1", "server-1", e, s); err == nil {
		t.Fatal("noncanonical replay fields accepted")
	}
}
