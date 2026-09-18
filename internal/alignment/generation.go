package alignment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type generationState struct {
	Plan    RebaselinePlan `json:"plan"`
	Retired []CutoverProof `json:"retired"`
}

func readGeneration(ctx context.Context, db *sql.DB) (*generationState, error) {
	var data []byte
	var prepared bool
	err := db.QueryRowContext(ctx, "SELECT plan_json,prepared FROM sync_rebaseline_generation WHERE singleton=1").Scan(&data, &prepared)
	if errors.Is(err, sql.ErrNoRows) || isMissingTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state generationState
	if json.Unmarshal(data, &state.Plan) != nil || state.Plan.Validate() != nil {
		return nil, errors.New("rebaseline_record_invalid")
	}
	if !prepared {
		return nil, errors.New("rebaseline_preparation_incomplete")
	}
	state.Retired, err = readRetiredProofs(ctx, db)
	return &state, err
}

func readRetiredProofs(ctx context.Context, db *sql.DB) ([]CutoverProof, error) {
	rows, err := db.QueryContext(ctx, "SELECT proof_json FROM sync_rebaseline_retired ORDER BY proof_id")
	if isMissingTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []CutoverProof
	for rows.Next() {
		var data []byte
		var proof CutoverProof
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if json.Unmarshal(data, &proof) != nil || proof.Validate() != nil {
			return nil, errors.New("rebaseline_retired_proof_invalid")
		}
		result = append(result, proof)
	}
	return result, rows.Err()
}

func exchangeGeneration(ctx context.Context, db *sql.DB, control *PairControl, rule rules.SyncRule, node, peer string) error {
	local, err := readGeneration(ctx, db)
	if err != nil {
		return err
	}
	var remote *generationState
	if err := control.Exchange(ctx, "generation", local, &remote); err != nil {
		return err
	}
	if local == nil && remote == nil {
		return nil
	}
	if local == nil || remote == nil {
		return errors.New("rebaseline_both_nodes_must_prepare")
	}
	if err := validateGenerationPeer(local.Plan, remote.Plan, node, peer, rule.ID); err != nil {
		return err
	}
	if !slices.ContainsFunc(local.Plan.Rules, func(item RebaselineRule) bool {
		return hash(canonicalCutoverRule(item.Rule)) == hash(canonicalCutoverRule(rule))
	}) {
		return errors.New("rebaseline_rule_changed_after_prepare")
	}
	for _, proof := range remote.Retired {
		if err := validateRetiredPair(proof, local.Plan.EdgeNode, local.Plan.ServerNode); err != nil {
			return err
		}
		data, err := json.Marshal(proof)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO sync_rebaseline_retired (proof_id,proof_json) VALUES (?,?) ON DUPLICATE KEY UPDATE proof_id=proof_id", proof.ID, string(data)); err != nil {
			return err
		}
	}
	return nil
}

func generationRulesHash(items []RebaselineRule) string {
	canonical := make([]rules.SyncRule, len(items))
	for i, item := range items {
		canonical[i] = canonicalCutoverRule(item.Rule)
	}
	return hash(canonical)
}

func validateGenerationPeer(local, peer RebaselinePlan, node, peerNode, ruleID string) error {
	if local.Validate() != nil || peer.Validate() != nil || local.NodeID != node || peer.NodeID != peerNode || local.MigrationID != peer.MigrationID || local.EdgeNode != peer.EdgeNode || local.ServerNode != peer.ServerNode || generationRulesHash(local.Rules) != generationRulesHash(peer.Rules) {
		return errors.New("rebaseline_generation_mismatch")
	}
	if !slices.ContainsFunc(local.Rules, func(r RebaselineRule) bool { return r.Rule.ID == ruleID }) {
		return errors.New("rebaseline_rule_not_in_plan")
	}
	return nil
}

func validateRetiredPair(proof CutoverProof, edge, server string) error {
	if err := proof.Validate(); err != nil {
		return err
	}
	pair := proof.ObservedPair()
	if pair.EdgeNode != edge || pair.ServerNode != server {
		return errors.New("rebaseline_existing_other_member: complete topology migration required")
	}
	return nil
}

// A new metadata generation never authorizes a partial set to start. Every
// planned rule must have completed the ordinary durable cutover protocol.
func LoadGenerationFilter(ctx context.Context, f *CutoverFilter) error {
	state, err := readGeneration(ctx, f.DB)
	if err != nil || state == nil {
		return err
	}
	if state.Plan.NodeID != f.NodeID {
		return errors.New("rebaseline_node_changed")
	}
	for _, item := range state.Plan.Rules {
		found := false
		for _, proof := range f.Proofs {
			found = found || proof.MatchesRule(item.Rule)
		}
		if !found {
			return fmt.Errorf("rebaseline_alignment_incomplete: %s", item.Rule.ID)
		}
	}
	f.Retired = state.Retired
	return nil
}

func (f *CutoverFilter) retiredEvent(evt event.SyncEvent) (*CutoverProof, bool, error) {
	for i := range f.Retired {
		proof := &f.Retired[i]
		if proof.ID != evt.Headers[EpochHeader] {
			continue
		}
		pair := proof.ObservedPair()
		for _, boundary := range []SnapshotBoundary{proof.Source, proof.Target} {
			if boundary.OriginNodeID != evt.OriginNodeID || boundary.DatabaseName != evt.DatabaseName || boundary.TableName != evt.TableName {
				continue
			}
			if evt.EventID == "" || len(evt.EventID) > 128 || evt.Headers[SourceUUIDHeader] != boundary.MySQLServerUUID || evt.TargetNodeID != "" && evt.TargetNodeID != f.NodeID || evt.SourceNodeID != evt.OriginNodeID && evt.SourceNodeID != pair.ServerNode {
				return nil, false, errors.New("rebaseline_retired_event_identity_invalid")
			}
			return proof, true, nil
		}
		return nil, false, errors.New("rebaseline_retired_event_scope_invalid")
	}
	return nil, false, nil
}
