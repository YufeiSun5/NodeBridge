package alignment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

const topologyPending = "PENDING"

type TopologyMember struct {
	EdgeNode string         `json:"edge_node_id"`
	Rule     rules.SyncRule `json:"rule"`
}

type TopologyIntent struct {
	ID             string           `json:"intent_id"`
	ServerNode     string           `json:"server_node_id"`
	ServerDatabase string           `json:"server_database"`
	ServerTable    string           `json:"server_table"`
	Members        []TopologyMember `json:"members"`
}

type TopologyReceipt struct {
	IntentID   string `json:"intent_id"`
	TopologyID string `json:"topology_id"`
	NodeID     string `json:"node_id"`
	Phase      string `json:"phase"`
}

func sealTopologyIntent(intent TopologyIntent) (TopologyIntent, error) {
	intent.Members = slices.Clone(intent.Members)
	for i := range intent.Members {
		intent.Members[i].Rule = canonicalCutoverRule(intent.Members[i].Rule)
	}
	sort.Slice(intent.Members, func(i, j int) bool { return intent.Members[i].EdgeNode < intent.Members[j].EdgeNode })
	intent.ID = ""
	intent.ID = hash(intent)
	return intent, intent.Validate()
}

func (in TopologyIntent) Validate() error {
	unsigned := in
	unsigned.ID = ""
	if in.ID == "" || hash(unsigned) != in.ID || in.ServerNode == "" || len(in.Members) == 0 || len(in.Members) > 256 {
		return errors.New("alignment_topology_intent_invalid")
	}
	if err := validateTable(in.ServerDatabase, in.ServerTable); err != nil {
		return err
	}
	for i, member := range in.Members {
		rule := member.Rule
		if member.EdgeNode == "" || member.EdgeNode == in.ServerNode || i > 0 && in.Members[i-1].EdgeNode >= member.EdgeNode || rule.Enable || rule.Direction != rules.DirectionBidirectional || rule.ConflictPolicy != rules.ConflictLastWriteWin {
			return errors.New("alignment_topology_member_invalid")
		}
		if err := (rules.RuleSet{Rules: []rules.SyncRule{rule}}).ValidateStructure(); err != nil {
			return err
		}
		database, table := rule.TargetDatabaseName, rule.TargetTableName
		if database == "" {
			database = rule.DatabaseName
		}
		if table == "" {
			table = rule.TableName
		}
		if database != in.ServerDatabase || table != in.ServerTable || rule.InitialAlignment.EffectivePolicy() != rules.AlignmentManual {
			return errors.New("alignment_topology_rule_scope_invalid")
		}
		if len(rule.SourceNodeIDs) > 0 && !slices.Contains(rule.SourceNodeIDs, member.EdgeNode) || rule.DispatchTarget == rules.DispatchNone || rule.DispatchTarget == rules.DispatchSelectedEdges && !slices.Contains(rule.DispatchNodeIDs, member.EdgeNode) {
			return errors.New("alignment_topology_member_outside_rule_scope")
		}
		if rule.DispatchTarget == rules.DispatchSelectedEdges {
			for _, destination := range in.Members {
				if !slices.Contains(rule.DispatchNodeIDs, destination.EdgeNode) {
					return errors.New("alignment_topology_incomplete_dispatch_scope")
				}
			}
		}
	}
	return nil
}

func (in TopologyIntent) localScope(node string) (string, string, error) {
	if node == in.ServerNode {
		return in.ServerDatabase, in.ServerTable, nil
	}
	for _, member := range in.Members {
		if member.EdgeNode == node {
			return member.Rule.DatabaseName, member.Rule.TableName, nil
		}
	}
	return "", "", errors.New("alignment_topology_node_missing")
}

func (in TopologyIntent) validateProof(topology TopologyProof) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if err := topology.Validate(); err != nil {
		return err
	}
	if len(topology.Proofs) != len(in.Members) {
		return errors.New("alignment_topology_incomplete")
	}
	for _, member := range in.Members {
		found := false
		for _, proof := range topology.Proofs {
			pair := proof.ObservedPair()
			if pair.EdgeNode == member.EdgeNode && pair.ServerNode == in.ServerNode && proof.MatchesRule(member.Rule) {
				found = true
				break
			}
		}
		if !found {
			return errors.New("alignment_topology_member_proof_missing")
		}
	}
	return nil
}

// The unique nullable reservation prevents overlapping unfinished group intents.
// Old ACTIVE records remain immutable evidence when a membership is extended.
func prepareTopology(ctx context.Context, db *sql.DB, node string, intent TopologyIntent) error {
	if err := intent.Validate(); err != nil {
		return err
	}
	database, table, err := intent.localScope(node)
	if err != nil {
		return err
	}
	var folding int
	if err := db.QueryRowContext(ctx, "SELECT @@lower_case_table_names").Scan(&folding); err != nil {
		return err
	}
	scope := canonicalJobScope(database, table, folding)
	data, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "INSERT INTO sync_alignment_topology (intent_id,scope_hash,pending_scope,node_id,intent_json,phase,updated_at) VALUES (?,?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE intent_id=intent_id", intent.ID, scope, scope, node, string(data), topologyPending)
	if err != nil {
		return err
	}
	var actualNode string
	var actual []byte
	if err := db.QueryRowContext(ctx, "SELECT node_id,intent_json FROM sync_alignment_topology WHERE intent_id=?", intent.ID).Scan(&actualNode, &actual); err != nil {
		return fmt.Errorf("alignment_another_topology_pending: %w", err)
	}
	var stored TopologyIntent
	if json.Unmarshal(actual, &stored) != nil || hash(stored) != hash(intent) || actualNode != node {
		return errors.New("alignment_topology_intent_changed")
	}
	return nil
}

func readyTopology(ctx context.Context, db *sql.DB, node string, intent TopologyIntent, topology TopologyProof) (TopologyReceipt, error) {
	var receipt TopologyReceipt
	if err := intent.validateProof(topology); err != nil {
		return receipt, err
	}
	for _, proof := range topology.Proofs {
		if _, err := proof.Local(node); err != nil {
			continue
		}
		id, _, _, _ := jobIdentity(proof.Plan, node)
		stored, status, _, err := readCutover(ctx, db, id)
		if err != nil || status.Phase != CutoverActive || stored.ID != proof.ID {
			return receipt, errors.Join(errors.New("alignment_topology_local_copy_not_active"), err)
		}
	}
	data, err := json.Marshal(topology)
	if err != nil {
		return receipt, err
	}
	_, err = db.ExecContext(ctx, "UPDATE sync_alignment_topology SET topology_json=?,phase=?,updated_at=UTC_TIMESTAMP(6) WHERE intent_id=? AND node_id=? AND phase=?", string(data), CutoverReady, intent.ID, node, topologyPending)
	if err != nil {
		return receipt, err
	}
	var stored []byte
	var phase string
	if err := db.QueryRowContext(ctx, "SELECT topology_json,phase FROM sync_alignment_topology WHERE intent_id=? AND node_id=?", intent.ID, node).Scan(&stored, &phase); err != nil {
		return receipt, err
	}
	var actual TopologyProof
	if json.Unmarshal(stored, &actual) != nil || actual.ID != topology.ID || actual.Validate() != nil || phase != CutoverReady && phase != CutoverActive {
		return receipt, errors.New("alignment_topology_ready_changed")
	}
	return TopologyReceipt{IntentID: intent.ID, TopologyID: topology.ID, NodeID: node, Phase: phase}, nil
}

func validateTopologyReceipts(intent TopologyIntent, topology TopologyProof, receipts []TopologyReceipt) error {
	if err := intent.validateProof(topology); err != nil {
		return err
	}
	if len(receipts) != len(intent.Members)+1 {
		return errors.New("alignment_topology_receipts_incomplete")
	}
	seen := map[string]bool{}
	for _, receipt := range receipts {
		if receipt.IntentID != intent.ID || receipt.TopologyID != topology.ID || receipt.Phase != CutoverReady && receipt.Phase != CutoverActive || seen[receipt.NodeID] {
			return errors.New("alignment_topology_receipt_invalid")
		}
		if _, _, err := intent.localScope(receipt.NodeID); err != nil {
			return err
		}
		seen[receipt.NodeID] = true
	}
	return nil
}

func activateTopology(ctx context.Context, db *sql.DB, node string, intent TopologyIntent, topology TopologyProof, receipts []TopologyReceipt) error {
	if err := validateTopologyReceipts(intent, topology, receipts); err != nil {
		return err
	}
	if _, err := readyTopology(ctx, db, node, intent, topology); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, "UPDATE sync_alignment_topology SET phase=?,pending_scope=NULL,updated_at=UTC_TIMESTAMP(6) WHERE intent_id=? AND node_id=? AND phase=?", CutoverActive, intent.ID, node, CutoverReady)
	return err
}

func loadActiveTopologyProofs(ctx context.Context, db *sql.DB, localProofs []CutoverProof, scopes ...string) ([]CutoverProof, error) {
	clause, args := scopePredicate(scopes)
	if clause != "" {
		clause = " WHERE " + clause[len(" AND "):]
	}
	rows, err := db.QueryContext(ctx, "SELECT node_id,intent_json,topology_json,phase FROM sync_alignment_topology"+clause, args...)
	if isMissingTable(err) {
		return localProofs, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := append([]CutoverProof(nil), localProofs...)
	known := map[string]bool{}
	for _, proof := range all {
		known[proof.ID] = true
	}
	for rows.Next() {
		var node, phase string
		var encodedIntent, encodedTopology []byte
		if err := rows.Scan(&node, &encodedIntent, &encodedTopology, &phase); err != nil {
			return nil, err
		}
		if phase != CutoverActive {
			return nil, errors.New("alignment_topology_pending: every member must be durably ready")
		}
		var intent TopologyIntent
		var topology TopologyProof
		if json.Unmarshal(encodedIntent, &intent) != nil || json.Unmarshal(encodedTopology, &topology) != nil || intent.validateProof(topology) != nil {
			return nil, errors.New("alignment_topology_record_invalid")
		}
		if _, _, err := intent.localScope(node); err != nil {
			return nil, err
		}
		for _, proof := range topology.Proofs {
			if _, err := proof.Local(node); err == nil {
				found := false
				for _, local := range localProofs {
					found = found || local.ID == proof.ID
				}
				if !found {
					return nil, errors.New("alignment_topology_local_proof_missing")
				}
			}
			if !known[proof.ID] {
				all = append(all, proof)
				known[proof.ID] = true
			}
		}
	}
	return all, rows.Err()
}
