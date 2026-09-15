package alignment

import (
	"errors"
	"fmt"
	"sort"

	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// TopologyProof is the complete star for one central table, not a pair count.
// Local durable READY receipts are required before its ACTIVE transition.
type TopologyProof struct {
	ID     string         `json:"topology_id"`
	Proofs []CutoverProof `json:"proofs"`
}

func (p CutoverProof) ObservedPair() rulecheck.ObservedPair {
	edge, server := p.Plan.Source, p.Plan.Target
	if p.Plan.Direction == ToLeft {
		edge, server = server, edge
	}
	return rulecheck.ObservedPair{Rule: p.Rule, EdgeNode: edge.NodeID, ServerNode: server.NodeID, Edge: edge.Schema, Server: server.Schema}
}

func BuildTopologyProof(proofs []CutoverProof) (TopologyProof, error) {
	topology := TopologyProof{Proofs: append([]CutoverProof(nil), proofs...)}
	sort.Slice(topology.Proofs, func(i, j int) bool { return topology.Proofs[i].ID < topology.Proofs[j].ID })
	topology.ID = hash(topology)
	return topology, topology.Validate()
}

func (t TopologyProof) Validate() error {
	unsigned := t
	unsigned.ID = ""
	if t.ID == "" || hash(unsigned) != t.ID || len(t.Proofs) == 0 || len(t.Proofs) > 256 {
		return errors.New("alignment_topology_identity_invalid")
	}
	var observations []rulecheck.ObservedPair
	serverScope := ""
	streams := map[string]SnapshotBoundary{}
	for i, proof := range t.Proofs {
		if err := proof.Validate(); err != nil {
			return err
		}
		if i > 0 && t.Proofs[i-1].ID >= proof.ID {
			return errors.New("alignment_topology_proof_order_invalid")
		}
		pair := proof.ObservedPair()
		if pair.Rule.Direction != rules.DirectionBidirectional {
			return errors.New("alignment_topology_bidirectional_required")
		}
		scope := pair.ServerNode + ":" + pair.Server.Database + "." + pair.Server.Table
		if serverScope != "" && serverScope != scope {
			return errors.New("alignment_topology_central_scope_changed")
		}
		serverScope = scope
		observations = append(observations, pair)
		for _, boundary := range []SnapshotBoundary{proof.Source, proof.Target} {
			key := boundary.OriginNodeID + ":" + boundary.DatabaseName + "." + boundary.TableName
			if previous, ok := streams[key]; ok {
				if previous.MySQLServerUUID != boundary.MySQLServerUUID {
					return errors.New("alignment_topology_source_identity_changed")
				}
				if _, err := previous.BeforePosition(boundary.BinlogFile, boundary.BinlogPos); err != nil {
					return err
				}
			}
			streams[key] = boundary
		}
	}
	_, err := rulecheck.BuildBidirectionalGraph(observations)
	return err
}

func (t TopologyProof) Observations() []rulecheck.ObservedPair {
	pairs := make([]rulecheck.ObservedPair, 0, len(t.Proofs))
	for _, proof := range t.Proofs {
		pairs = append(pairs, proof.ObservedPair())
	}
	return pairs
}

func (t TopologyProof) Local(node string) (SnapshotBoundary, error) {
	for _, proof := range t.Proofs {
		if local, err := proof.Local(node); err == nil {
			return local, nil
		}
	}
	return SnapshotBoundary{}, fmt.Errorf("alignment_topology_node_missing: %s", node)
}

// Every observation, including remote mappings, must be backed by a proof
// loaded from local durable state, never merely by an editable manifest file.
func VerifyObservedPairs(pairs []rulecheck.ObservedPair, proofs []CutoverProof) error {
	for _, pair := range pairs {
		pair.Rule = canonicalCutoverRule(pair.Rule)
		found := false
		for _, proof := range proofs {
			observed := proof.ObservedPair()
			observed.Rule = canonicalCutoverRule(observed.Rule)
			if hash(observed) == hash(pair) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("alignment_observation_proof_required: %s", pair.EdgeNode)
		}
	}
	for _, proof := range proofs {
		if proof.Rule.Direction != rules.DirectionBidirectional {
			continue
		}
		observed := proof.ObservedPair()
		observed.Rule = canonicalCutoverRule(observed.Rule)
		found := false
		for _, pair := range pairs {
			pair.Rule = canonicalCutoverRule(pair.Rule)
			found = found || hash(pair) == hash(observed)
		}
		if !found {
			return errors.New("alignment_topology_manifest_incomplete")
		}
	}
	return nil
}
