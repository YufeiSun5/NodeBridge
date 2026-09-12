package rulecheck_test

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncruntime"
)

type graphPublisher struct{ requests []rabbitmq.PublishRequest }

func (p *graphPublisher) Publish(_ context.Context, request rabbitmq.PublishRequest) error {
	p.requests = append(p.requests, request)
	return nil
}

func TestBidirectionalGraphSerializedDownlink(t *testing.T) {
	observations := graphFixture()
	graph, err := rulecheck.BuildBidirectionalGraph(observations)
	if err != nil {
		t.Fatal(err)
	}
	for _, batch := range []bool{false, true} {
		for _, operation := range []string{event.TypeInsert, event.TypeUpdate, event.TypeDelete} {
			for _, source := range []struct{ node, database, table, key, value string }{{"edge-1", "edge_db", "source_rows", "id", "value"}, {"edge-2", "other_db", "other_rows", "other_id", "other_value"}, {"server-1", "server_db", "target_rows", "row_id", "payload"}} {
				key := json.Number("18446744073709551615")
				row := map[string]any{source.key: key, source.value: rowvalue.Binary{0, 128, 255}, "last_event_id": "", "updated_by_node": ""}
				raw := event.SyncEvent{EventID: "source-event", EventType: operation, OriginNodeID: source.node, SourceNodeID: source.node, DatabaseName: source.database, TableName: source.table, PrimaryKey: map[string]any{source.key: key}}
				if operation != event.TypeInsert {
					raw.Before = row
				}
				if operation != event.TypeDelete {
					raw.After = row
				}
				for _, destination := range observations {
					if destination.EdgeNode == source.node {
						continue
					}
					publisher := &graphPublisher{}
					dispatcher := syncruntime.RoutingDownlinkDispatcher{Publisher: publisher, Exchange: "test"}
					if batch {
						err = dispatcher.DispatchBatch(context.Background(), []syncruntime.DownlinkRequest{{Event: raw, TargetNodeID: destination.EdgeNode}})
					} else {
						err = dispatcher.Dispatch(context.Background(), raw, destination.EdgeNode)
					}
					if err != nil || len(publisher.requests) != 1 {
						t.Fatalf("dispatch: %v", err)
					}
					var received event.SyncEvent
					if err := json.Unmarshal(publisher.requests[0].Body, &received); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(received, raw) {
						t.Fatalf("relay changed raw source event: %+v", received)
					}
					projection := graph[destination.EdgeNode].Incoming.FindForNode(received.DatabaseName, received.TableName, received.OriginNodeID, received.SourceNodeID)
					if projection == nil {
						t.Fatal("serialized source route missing")
					}
					mapped, err := mapper.MapEvent(received, *projection)
					if err != nil {
						t.Fatal(err)
					}
					if mapped.TargetDatabase != destination.Edge.Database || mapped.TargetTable != destination.Edge.Table || mapped.TargetPrimaryKey[destination.Edge.PrimaryKeys[0]] != key || mapped.Event.EventType != operation {
						t.Fatalf("bad downlink projection: %+v", mapped)
					}
				}
			}
		}
	}
}

func graphFixture() []rulecheck.ObservedPair {
	r1, e1, s1 := pairFixture()
	r2, e2, s2 := pairFixture()
	r2.ID, r2.DatabaseName, r2.TableName = "pair-2", "other_db", "other_rows"
	r2.PrimaryKeys = []string{"other_id"}
	r2.ColumnMappings = []rules.ColumnMapping{{SourceColumn: "other_value", TargetColumn: "payload"}}
	e2.Database, e2.Table, e2.PrimaryKeys = r2.DatabaseName, r2.TableName, []string{"other_id"}
	e2.Columns[0].Name, e2.Columns[1].Name = "other_id", "other_value"
	return []rulecheck.ObservedPair{{Rule: r1, EdgeNode: "edge-1", ServerNode: "server-1", Edge: e1, Server: s1}, {Rule: r2, EdgeNode: "edge-2", ServerNode: "server-1", Edge: e2, Server: s2}}
}

func TestBidirectionalGraphDifferentEdgeNames(t *testing.T) {
	observations := graphFixture()
	before, _ := json.Marshal(observations)
	graph, err := rulecheck.BuildBidirectionalGraph(observations)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph) != 3 || len(graph["server-1"].Capture.Rules) != 1 || len(graph["server-1"].Incoming.Rules) != 2 {
		t.Fatalf("graph: %+v", graph)
	}
	key := json.Number("18446744073709551615")
	raw := event.SyncEvent{EventID: "original", EventType: event.TypeInsert, OriginNodeID: "edge-1", SourceNodeID: "edge-1", DatabaseName: "edge_db", TableName: "source_rows", PrimaryKey: map[string]any{"id": key}, After: map[string]any{"id": key, "value": rowvalue.Binary{0, 128, 255}, "last_event_id": "", "updated_by_node": ""}}
	for _, tc := range []struct{ node, database, table, key, value string }{{"server-1", "server_db", "target_rows", "row_id", "payload"}, {"edge-2", "other_db", "other_rows", "other_id", "other_value"}} {
		rule := graph[tc.node].Incoming.FindForNode(raw.DatabaseName, raw.TableName, raw.OriginNodeID, raw.SourceNodeID)
		if rule == nil {
			t.Fatalf("missing projection for %s", tc.node)
		}
		mapped, err := mapper.MapEvent(raw, *rule)
		if err != nil {
			t.Fatal(err)
		}
		if mapped.TargetDatabase != tc.database || mapped.TargetTable != tc.table || mapped.TargetPrimaryKey[tc.key] != key || !reflect.DeepEqual(mapped.Event.After[tc.value], raw.After["value"]) || mapped.Event.EventID != raw.EventID || mapped.Event.OriginNodeID != raw.OriginNodeID {
			t.Fatalf("bad projection for %s: %+v", tc.node, mapped)
		}
		if rule := graph[tc.node].Incoming.FindForNode(raw.DatabaseName, raw.TableName, "unknown", "unknown"); rule != nil {
			t.Fatal("unobserved source admitted")
		}
	}
	serverCapture := graph["server-1"].Capture.Rules[0]
	if serverCapture.TargetDatabaseName != serverCapture.DatabaseName || serverCapture.TargetTableName != serverCapture.TableName || len(serverCapture.ColumnMappings) != 0 || !reflect.DeepEqual(serverCapture.DispatchNodeIDs, []string{"edge-1", "edge-2"}) {
		t.Fatalf("server capture is not local identity: %+v", serverCapture)
	}
	for _, source := range observations {
		incoming := graph["server-1"].Incoming.FindForNode(source.Edge.Database, source.Edge.Table, source.EdgeNode, source.EdgeNode)
		if incoming == nil || slices.Contains(incoming.DispatchNodeIDs, source.EdgeNode) || len(incoming.DispatchNodeIDs) != 1 {
			t.Fatal("dispatch must contain only the other observed edge")
		}
	}
	graph["edge-1"].Capture.Rules[0].ColumnMappings[0].TargetColumn = "changed"
	graph["server-1"].Capture.Rules[0].DispatchNodeIDs[0] = "changed"
	after, _ := json.Marshal(observations)
	if string(before) != string(after) || graph["edge-2"].Incoming.FindForNode("edge_db", "source_rows", "edge-1", "edge-1").ColumnMappings[0].TargetColumn != "other_id" {
		t.Fatal("graph aliases inputs or other projections")
	}
}

func TestBidirectionalGraphScopeAndOrder(t *testing.T) {
	observations := graphFixture()
	observations[0].Rule.DispatchTarget = rules.DispatchSelectedEdges
	observations[0].Rule.DispatchNodeIDs = []string{"edge-1", "unobserved-edge"}
	graph, err := rulecheck.BuildBidirectionalGraph(observations)
	if err != nil {
		t.Fatal(err)
	}
	incoming := graph["server-1"].Incoming.FindForNode("edge_db", "source_rows", "edge-1", "edge-1")
	if incoming.DispatchTarget != rules.DispatchNone || len(incoming.DispatchNodeIDs) != 0 || graph["edge-2"].Incoming.FindForNode("edge_db", "source_rows", "edge-1", "edge-1") != nil {
		t.Fatal("explicit dispatch scope widened")
	}
	slices.Reverse(observations)
	reordered, err := rulecheck.BuildBidirectionalGraph(observations)
	if err != nil || !reflect.DeepEqual(graph, reordered) {
		t.Fatalf("observation order changed graph: %v", err)
	}
	for _, observation := range observations {
		if graph[observation.EdgeNode].Incoming.FindForNode("server_db", "target_rows", "server-1", "server-1") == nil {
			t.Fatal("server downlink missing")
		}
	}
}

func TestBidirectionalGraphRejectsAmbiguity(t *testing.T) {
	cases := map[string]func([]rulecheck.ObservedPair) []rulecheck.ObservedPair{
		"duplicate":       func(o []rulecheck.ObservedPair) []rulecheck.ObservedPair { return append(o, o[0]) },
		"multiple server": func(o []rulecheck.ObservedPair) []rulecheck.ObservedPair { o[1].ServerNode = "server-2"; return o },
		"inconsistent observed schema": func(o []rulecheck.ObservedPair) []rulecheck.ObservedPair {
			o[1].Server.Columns[0].HasDefault = true
			return o
		},
		"mixed activation":            func(o []rulecheck.ObservedPair) []rulecheck.ObservedPair { o[1].Rule.Enable = true; return o },
		"same edge two source tables": func(o []rulecheck.ObservedPair) []rulecheck.ObservedPair { o[1].EdgeNode = o[0].EdgeNode; return o },
		"one edge table two server destinations": func(o []rulecheck.ObservedPair) []rulecheck.ObservedPair {
			o[1] = o[0]
			o[1].Rule.TargetTableName, o[1].Server.Table = "another_target", "another_target"
			return o
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if graph, err := rulecheck.BuildBidirectionalGraph(mutate(graphFixture())); err == nil || graph != nil {
				t.Fatalf("unsafe graph returned: %v %+v", err, graph)
			}
		})
	}
}

func TestBidirectionalGraphSameNamesRemainNodeScoped(t *testing.T) {
	r, e, _ := pairFixture()
	s := e
	r.TargetDatabaseName, r.TargetTableName, r.TargetPrimaryKeys, r.ColumnMappings = "", "", nil, nil
	observations := []rulecheck.ObservedPair{{r, "edge-1", "server-1", e, s}, {r, "edge-2", "server-1", e, s}}
	graph, err := rulecheck.BuildBidirectionalGraph(observations)
	if err != nil {
		t.Fatal(err)
	}
	for node, endpoint := range graph {
		if len(endpoint.Capture.Rules) != 1 || len(endpoint.Incoming.Rules) != 2 {
			t.Fatalf("same names collapsed node routes: %s %+v", node, endpoint)
		}
		if endpoint.Incoming.FindForNode(e.Database, e.Table, node, node) != nil {
			t.Fatal("own source has an incoming route")
		}
	}
}

func TestBidirectionalGraphDoesNotAuthorizeActivation(t *testing.T) {
	observations := graphFixture()
	for i := range observations {
		observations[i].Rule.Enable = true
	}
	graph, err := rulecheck.BuildBidirectionalGraph(observations)
	if err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range graph {
		for _, set := range []rules.RuleSet{endpoint.Capture, endpoint.Incoming} {
			if err := set.Validate(); err == nil {
				t.Fatal("compiled rules bypass public activation gate")
			}
		}
	}
}
