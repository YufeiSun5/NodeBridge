package alignment

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func topologyFixture(t *testing.T) []CutoverProof {
	t.Helper()
	var proofs []CutoverProof
	for i, node := range []string{"edge-1", "edge-2"} {
		rule, left, right := fixture()
		rule.Direction, rule.ConflictPolicy = rules.DirectionBidirectional, rules.ConflictLastWriteWin
		rule.ID, rule.SourceNodeIDs = "pair-"+node, []string{node}
		left.NodeID = node
		left.Schema.Database = "db_" + strings.ReplaceAll(node, "-", "_")
		rule.DatabaseName = left.Schema.Database
		metadata := []rulecheck.Column{{Name: "last_event_id", Type: "varchar(128)"}, {Name: "updated_by_node", Type: "varchar(64)"}}
		left.Schema.Columns = append(left.Schema.Columns, metadata...)
		right.Schema.Columns = append(right.Schema.Columns, metadata...)
		right.HasRows = true
		plan, err := BuildPlan(rule, left, right, time.Now().Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		result := &CopyResult{PlanID: plan.ID, Rows: 2, Digest: strings.Repeat("a", 64)}
		var jobs []SnapshotJob
		for _, observation := range []Observation{plan.Source, plan.Target} {
			boundary := boundaryFixture()
			boundary.OriginNodeID, boundary.DatabaseName, boundary.TableName = observation.NodeID, observation.Schema.Database, observation.Schema.Table
			boundary.BinlogPos = uint32(1000 + i*100)
			id, _, role, _ := jobIdentity(plan, observation.NodeID)
			phase := JobTargetConfirmed
			if role == "TARGET" {
				phase = JobTargetCommitted
			}
			jobs = append(jobs, SnapshotJob{ID: id, NodeID: observation.NodeID, Role: role, Phase: phase, Plan: plan, Result: result, CaptureMarker: boundary.Token, CaptureBoundary: &boundary})
		}
		proof, err := BuildCutoverProof(rule, jobs[0], jobs[1])
		if err != nil {
			t.Fatal(err)
		}
		proofs = append(proofs, proof)
	}
	return proofs
}

func TestConfiguredTopologyMatchesEveryExplicitPeer(t *testing.T) {
	proofs := topologyFixture(t)
	one, two := proofs[0].Rule, proofs[1].Rule
	set := rules.RuleSet{Rules: []rules.SyncRule{one, two}}
	intent, err := configuredTopologyIntent("server", set, one, []string{"edge-2", "edge-1"})
	if err != nil || len(intent.Members) != 2 || intent.Members[0].Rule.ID != one.ID || intent.Members[1].Rule.ID != two.ID {
		t.Fatal("heterogeneous peers were not matched by source scope", intent, err)
	}
	shared := one
	shared.SourceNodeIDs = []string{"edge-1", "edge-2"}
	intent, err = configuredTopologyIntent("server", rules.RuleSet{Rules: []rules.SyncRule{shared}}, shared, []string{"edge-2", "edge-1"})
	if err != nil || intent.Members[0].Rule.ID != intent.Members[1].Rule.ID {
		t.Fatal("shared same-schema rule was rejected", intent, err)
	}
	for _, peers := range [][]string{{"unknown"}, {"edge-1", "edge-1"}, {"server"}, nil} {
		if _, err := configuredTopologyIntent("server", set, one, peers); err == nil {
			t.Fatal("invalid membership accepted", peers)
		}
	}
	ambiguous := two
	ambiguous.SourceNodeIDs = nil
	if _, err := configuredTopologyIntent("server", rules.RuleSet{Rules: []rules.SyncRule{one, ambiguous}}, one, []string{"edge-1"}); err == nil {
		t.Fatal("ambiguous source scope accepted")
	}
	shared.DispatchTarget, shared.DispatchNodeIDs = rules.DispatchSelectedEdges, []string{"edge-1"}
	if _, err := configuredTopologyIntent("server", rules.RuleSet{Rules: []rules.SyncRule{shared}}, shared, []string{"edge-1", "edge-2"}); err == nil {
		t.Fatal("member outside dispatch scope accepted")
	}
	one.DispatchTarget, one.DispatchNodeIDs = rules.DispatchSelectedEdges, []string{"edge-1"}
	two.DispatchTarget, two.DispatchNodeIDs = rules.DispatchSelectedEdges, []string{"edge-2"}
	if _, err := configuredTopologyIntent("server", rules.RuleSet{Rules: []rules.SyncRule{one, two}}, one, []string{"edge-1", "edge-2"}); err == nil {
		t.Fatal("mutually isolated edges accepted as a convergent group")
	}
	one.DispatchNodeIDs, two.DispatchNodeIDs = []string{"edge-1", "edge-2"}, []string{"edge-1", "edge-2"}
	if _, err := configuredTopologyIntent("server", rules.RuleSet{Rules: []rules.SyncRule{one, two}}, one, []string{"edge-1", "edge-2"}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTopologyRequiresCompleteDurableLocalAndRemoteEvidence(t *testing.T) {
	proofs := topologyFixture(t)
	topology, _ := BuildTopologyProof(proofs)
	intent := TopologyIntent{ServerNode: "server", ServerDatabase: proofs[0].ObservedPair().Server.Database, ServerTable: proofs[0].ObservedPair().Server.Table}
	for _, proof := range proofs {
		intent.Members = append(intent.Members, TopologyMember{EdgeNode: proof.ObservedPair().EdgeNode, Rule: proof.Rule})
	}
	intent, _ = sealTopologyIntent(intent)
	encodedIntent, _ := json.Marshal(intent)
	encodedTopology, _ := json.Marshal(topology)
	for _, mode := range []string{"active", "pending", "ready", "corrupt", "missing_local", "foreign_node", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			phase, node, encoded := CutoverActive, "edge-1", encodedTopology
			local := proofs[:1]
			switch mode {
			case "pending":
				phase = topologyPending
			case "ready":
				phase = CutoverReady
			case "corrupt":
				encoded = []byte(`{}`)
			case "missing_local":
				local = nil
			case "foreign_node":
				node = "unknown"
			case "incomplete":
				partial, _ := BuildTopologyProof(proofs[:1])
				encoded, _ = json.Marshal(partial)
			}
			mock.ExpectQuery("SELECT node_id,intent_json,topology_json,phase FROM sync_alignment_topology").WillReturnRows(sqlmock.NewRows([]string{"node", "intent", "topology", "phase"}).AddRow(node, encodedIntent, encoded, phase))
			loaded, err := loadActiveTopologyProofs(context.Background(), db, local)
			if mode == "active" {
				if err != nil || len(loaded) != 2 {
					t.Fatal("remote proof missing", loaded, err)
				}
			} else if err == nil {
				t.Fatal("invalid durable group accepted", mode)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTopologyUsesStableSourceEpochAndPreservesInterCopyVersionHistory(t *testing.T) {
	proofs := topologyFixture(t)
	for _, reverse := range []bool{false, true} {
		ordered := slices.Clone(proofs)
		if reverse {
			slices.Reverse(ordered)
		}
		server, err := NewCutoverFilter("server", nil, ordered)
		if err != nil {
			t.Fatal(err)
		}
		boundary, _ := proofs[0].Local("server")
		evt := event.SyncEvent{EventID: "native-event", OriginNodeID: "server", SourceNodeID: "server", DatabaseName: boundary.DatabaseName, TableName: boundary.TableName, BinlogFile: boundary.BinlogFile, BinlogPos: 1050}
		evt, err = server.Stamp(evt)
		if err != nil || evt.Headers[EpochHeader] != proofs[0].ID {
			t.Fatal("source namespace changed with membership/order", evt, err)
		}
		for _, node := range []string{"edge-1", "edge-2"} {
			receiver, err := NewCutoverFilter(node, nil, ordered)
			if err != nil {
				t.Fatal(err)
			}
			_, old, err := receiver.Superseded(evt)
			if err != nil || old {
				t.Fatal("inter-copy conflict version history was discarded", node, old, err)
			}
		}
	}
}

func TestTopologyRelayKeepsOriginalIdentityAndRejectsUnknownEpoch(t *testing.T) {
	proofs := topologyFixture(t)
	source, _ := NewCutoverFilter("edge-1", nil, proofs)
	target, _ := NewCutoverFilter("edge-2", nil, proofs)
	b, _ := proofs[0].Local("edge-1")
	before := event.SyncEvent{EventID: "edge-update", OriginNodeID: "edge-1", SourceNodeID: "edge-1", DatabaseName: b.DatabaseName, TableName: b.TableName, BinlogFile: b.BinlogFile, BinlogPos: 1005}
	evt, err := source.Stamp(before)
	if err != nil {
		t.Fatal(err)
	}
	for _, sender := range []string{"edge-1", "server"} {
		evt.SourceNodeID = sender
		if _, old, err := target.Superseded(evt); err != nil || old {
			t.Fatal("valid relay was discarded", sender, old, err)
		}
	}
	evt.SourceNodeID = "edge-2"
	if _, _, err := target.Superseded(evt); err == nil {
		t.Fatal("unrelated relay sender accepted")
	}
	evt.SourceNodeID = "server"
	evt.Headers[EpochHeader] = proofs[1].ID
	if _, _, err := target.Superseded(evt); err == nil {
		t.Fatal("another member's epoch accepted")
	}
}

func TestTopologyProofAndObservationBinding(t *testing.T) {
	proofs := topologyFixture(t)
	topology, err := BuildTopologyProof(proofs)
	if err != nil {
		t.Fatal(err)
	}
	other := slices.Clone(proofs)
	slices.Reverse(other)
	reordered, err := BuildTopologyProof(other)
	if err != nil || reordered.ID != topology.ID {
		t.Fatal("topology depends on enumeration order")
	}
	if err := VerifyObservedPairs(topology.Observations(), proofs); err != nil {
		t.Fatal(err)
	}
	pairs := topology.Observations()
	pairs[0].Edge.Columns = slices.Clone(pairs[0].Edge.Columns)
	pairs[0].Edge.Columns[0].Type = "int"
	if err := VerifyObservedPairs(pairs, proofs); err == nil {
		t.Fatal("edited remote observation bypassed durable proof")
	}
	if _, err := BuildTopologyProof(append(proofs, proofs[0])); err == nil {
		t.Fatal("duplicate member accepted")
	}
	if _, err := NewCutoverFilter("unknown", nil, proofs); err == nil {
		t.Fatal("nonmember can load topology")
	}
}

func TestTopologyActivationRequiresEveryDistinctMemberReceipt(t *testing.T) {
	proofs := topologyFixture(t)
	topology, _ := BuildTopologyProof(proofs)
	intent := TopologyIntent{ServerNode: "server", ServerDatabase: proofs[0].ObservedPair().Server.Database, ServerTable: proofs[0].ObservedPair().Server.Table}
	for _, proof := range proofs {
		intent.Members = append(intent.Members, TopologyMember{EdgeNode: proof.ObservedPair().EdgeNode, Rule: proof.Rule})
	}
	intent, err := sealTopologyIntent(intent)
	if err != nil {
		t.Fatal(err)
	}
	receipts := []TopologyReceipt{}
	for _, node := range []string{"server", "edge-1", "edge-2"} {
		receipts = append(receipts, TopologyReceipt{IntentID: intent.ID, TopologyID: topology.ID, NodeID: node, Phase: CutoverReady})
	}
	if err := validateTopologyReceipts(intent, topology, receipts); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing", "duplicate", "foreign", "pending", "wrong_topology"} {
		bad := slices.Clone(receipts)
		switch mode {
		case "missing":
			bad = bad[:2]
		case "duplicate":
			bad[2] = bad[1]
		case "foreign":
			bad[2].NodeID = "unknown"
		case "pending":
			bad[2].Phase = topologyPending
		case "wrong_topology":
			bad[2].TopologyID = "different"
		}
		if err := validateTopologyReceipts(intent, topology, bad); err == nil {
			t.Fatal("invalid receipt certificate accepted", mode)
		}
	}
	if err := checkTopologyExtension(topology.Observations(), intent); err != nil {
		t.Fatal(err)
	}
	removed := intent
	removed.Members = removed.Members[:1]
	removed, _ = sealTopologyIntent(removed)
	if err := checkTopologyExtension(topology.Observations(), removed); err == nil {
		t.Fatal("existing member was silently removed")
	}
}
