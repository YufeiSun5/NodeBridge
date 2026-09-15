package alignment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
)

const CutoverReady = "READY"
const CutoverActive = "ACTIVE"

// CutoverProof binds the committed snapshot to both original capture streams.
// It is installed while both Agents are exclusively stopped, before either runs.
type CutoverProof struct {
	ID     string           `json:"proof_id"`
	Plan   Plan             `json:"plan"`
	Rule   rules.SyncRule   `json:"rule"`
	Result CopyResult       `json:"result"`
	Source SnapshotBoundary `json:"source"`
	Target SnapshotBoundary `json:"target"`
}

type CutoverReceipt struct {
	ProofID string `json:"proof_id"`
	NodeID  string `json:"node_id"`
	Phase   string `json:"phase"`
}

func BuildCutoverProof(rule rules.SyncRule, source, target SnapshotJob) (CutoverProof, error) {
	var proof CutoverProof
	if source.Role != "SOURCE" || source.Phase != JobTargetConfirmed || target.Role != "TARGET" || target.Phase != JobTargetCommitted || source.Result == nil || target.Result == nil || *source.Result != *target.Result || source.CaptureBoundary == nil || target.CaptureBoundary == nil || hash(source.Plan) != hash(target.Plan) {
		return proof, errors.New("alignment_cutover_copy_incomplete")
	}
	for _, job := range []SnapshotJob{source, target} {
		id, _, role, err := jobIdentity(job.Plan, job.NodeID)
		if err != nil || id != job.ID || role != job.Role || job.CaptureMarker != job.CaptureBoundary.Token || job.NodeID != job.CaptureBoundary.OriginNodeID {
			return proof, errors.New("alignment_cutover_job_mismatch")
		}
	}
	proof = CutoverProof{Plan: source.Plan, Rule: rule, Result: *source.Result, Source: *source.CaptureBoundary, Target: *target.CaptureBoundary}
	proof.ID = hash(proof)
	return proof, proof.Validate()
}

func (p CutoverProof) Validate() error {
	unsigned := p
	unsigned.ID = ""
	if p.ID == "" || p.ID != hash(unsigned) || p.Rule.Enable {
		return errors.New("alignment_cutover_proof_invalid")
	}
	if err := p.Plan.Validate(p.Rule, p.Plan.CreatedAt, true); err != nil {
		return err
	}
	if p.Result.PlanID != p.Plan.ID {
		return errors.New("alignment_cutover_result_mismatch")
	}
	if err := validateFrame(SnapshotFrame{PlanID: p.Plan.ID, Sequence: p.Result.Rows, End: &p.Result}); err != nil {
		return err
	}
	for _, side := range []struct {
		boundary SnapshotBoundary
		observed Observation
	}{{p.Source, p.Plan.Source}, {p.Target, p.Plan.Target}} {
		b, o := side.boundary, side.observed
		if b.Validate() != nil || b.OriginNodeID != o.NodeID || b.DatabaseName != o.Schema.Database || b.TableName != o.Schema.Table {
			return errors.New("alignment_cutover_boundary_mismatch")
		}
	}
	return nil
}

func (p CutoverProof) Local(node string) (SnapshotBoundary, error) {
	switch node {
	case p.Source.OriginNodeID:
		return p.Source, nil
	case p.Target.OriginNodeID:
		return p.Target, nil
	default:
		return SnapshotBoundary{}, errors.New("alignment_cutover_node_mismatch")
	}
}

func (p CutoverProof) Peer(node string) (string, error) {
	if _, err := p.Local(node); err != nil {
		return "", err
	}
	if node == p.Source.OriginNodeID {
		return p.Target.OriginNodeID, nil
	}
	return p.Source.OriginNodeID, nil
}

func (p CutoverProof) MatchesRule(rule rules.SyncRule) bool {
	return hash(canonicalCutoverRule(p.Rule)) == hash(canonicalCutoverRule(rule))
}

func canonicalCutoverRule(rule rules.SyncRule) rules.SyncRule {
	rule.Name = ""
	rule.Enable = false
	rule.DeleteMode = rule.EffectiveDeleteMode()
	for _, values := range []*[]string{&rule.SourceNodeIDs, &rule.DispatchNodeIDs, &rule.PrimaryKeys, &rule.TargetPrimaryKeys, &rule.IncludeColumns, &rule.ExcludeColumns} {
		if len(*values) == 0 {
			*values = nil
		}
	}
	if len(rule.ColumnMappings) == 0 {
		rule.ColumnMappings = nil
	}
	return rule
}

// InstallCutover never enables an Agent. The caller also installs the matching
// rule observations while holding its existing configuration process lock.
func InstallCutover(ctx context.Context, db *sql.DB, proof CutoverProof, node string) (CutoverReceipt, error) {
	if err := proof.Validate(); err != nil {
		return CutoverReceipt{}, err
	}
	if db == nil {
		return CutoverReceipt{}, errors.New("alignment_endpoint_required")
	}
	job, err := ReadSnapshotJob(ctx, db, proof.Plan, node)
	if err != nil {
		return CutoverReceipt{}, err
	}
	boundary, err := proof.Local(node)
	if err != nil {
		return CutoverReceipt{}, err
	}
	if job.Result == nil || *job.Result != proof.Result || job.CaptureBoundary == nil || *job.CaptureBoundary != boundary || (job.Role == "SOURCE" && job.Phase != JobTargetConfirmed) || (job.Role == "TARGET" && job.Phase != JobTargetCommitted) {
		return CutoverReceipt{}, errors.New("alignment_cutover_local_commit_incomplete")
	}
	data, err := json.Marshal(proof)
	if err != nil {
		return CutoverReceipt{}, err
	}
	_, err = db.ExecContext(ctx, "INSERT INTO sync_alignment_cutover (job_id,proof_id,proof_json,phase,updated_at) VALUES (?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE job_id=job_id", job.ID, proof.ID, string(data), CutoverReady)
	if err != nil {
		return CutoverReceipt{}, err
	}
	stored, receipt, _, err := readCutover(ctx, db, job.ID)
	if err != nil || hash(stored) != hash(proof) {
		return CutoverReceipt{}, errors.Join(errors.New("alignment_cutover_install_mismatch"), err)
	}
	receipt.NodeID = node
	return receipt, nil
}

// ActivateCutover requires the peer's durable READY receipt, not merely a copy
// receipt. A lost response is recoverable without repeating the snapshot.
func ActivateCutover(ctx context.Context, db *sql.DB, proof CutoverProof, node string, peer CutoverReceipt) error {
	if err := proof.Validate(); err != nil {
		return err
	}
	peerNode, err := proof.Peer(node)
	if err != nil {
		return err
	}
	if peer.NodeID != peerNode || peer.ProofID != proof.ID || (peer.Phase != CutoverReady && peer.Phase != CutoverActive) {
		return errors.New("alignment_cutover_peer_not_ready")
	}
	local, err := InstallCutover(ctx, db, proof, node)
	if err != nil {
		return err
	}
	id, _, _, _ := jobIdentity(proof.Plan, node)
	if local.Phase == CutoverActive {
		_, _, existingPeer, err := readCutover(ctx, db, id)
		if err != nil || existingPeer != peerNode {
			return errors.Join(errors.New("alignment_cutover_peer_changed"), err)
		}
		return nil
	}
	result, err := db.ExecContext(ctx, "UPDATE sync_alignment_cutover SET phase=?,peer_node_id=?,updated_at=UTC_TIMESTAMP(6) WHERE job_id=? AND proof_id=? AND phase=?", CutoverActive, peerNode, id, proof.ID, CutoverReady)
	return requireJobUpdate(result, err)
}

func readCutover(ctx context.Context, db *sql.DB, jobID string) (CutoverProof, CutoverReceipt, string, error) {
	var proof CutoverProof
	var receipt CutoverReceipt
	var data []byte
	var peer sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT proof_id,proof_json,phase,peer_node_id FROM sync_alignment_cutover WHERE job_id=?", jobID).Scan(&receipt.ProofID, &data, &receipt.Phase, &peer); err != nil {
		return proof, receipt, "", err
	}
	if err := json.Unmarshal(data, &proof); err != nil {
		return proof, receipt, "", err
	}
	if proof.Validate() != nil || receipt.ProofID != proof.ID || (receipt.Phase != CutoverReady && receipt.Phase != CutoverActive) || (receipt.Phase == CutoverActive && !peer.Valid) {
		return proof, receipt, "", errors.New("alignment_cutover_record_invalid")
	}
	return proof, receipt, peer.String, nil
}

// LoadActiveCutovers checks every durable job, including jobs for disabled rules.
// Missing migrations on a database that already has a job never permit startup.
func LoadActiveCutovers(ctx context.Context, db *sql.DB) ([]CutoverProof, error) {
	return loadActiveCutovers(ctx, db, nil)
}

func loadActiveCutovers(ctx context.Context, db *sql.DB, scopes []string, folding ...int) ([]CutoverProof, error) {
	clause, args := scopePredicate(scopes)
	query := "SELECT job_id,node_id FROM sync_alignment_job WHERE phase<>'CANCELLED'"
	if len(scopes) > 0 {
		query = "SELECT job_id,node_id,scope_hash,plan_json FROM sync_alignment_job WHERE phase<>'CANCELLED' AND (" + clause[len(" AND "):] + " OR phase IN ('TARGET_COMMITTED','TARGET_CONFIRMED'))"
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if isMissingTable(err) {
		if len(scopes) > 0 {
			return loadActiveTopologyProofs(ctx, db, nil, scopes...)
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type identity struct{ id, node string }
	var jobs []identity
	for rows.Next() {
		var item identity
		var scanErr error
		if len(scopes) == 0 {
			scanErr = rows.Scan(&item.id, &item.node)
		} else {
			var scope string
			var data []byte
			scanErr = rows.Scan(&item.id, &item.node, &scope, &data)
			if scanErr == nil && !scopeIncluded(scopes, scope) {
				var plan Plan
				if err := json.Unmarshal(data, &plan); err != nil {
					rows.Close()
					return nil, errors.New("alignment_job_scope_invalid")
				}
				local := plan.Source
				if item.node == plan.Target.NodeID {
					local = plan.Target
				} else if item.node != plan.Source.NodeID {
					rows.Close()
					return nil, errors.New("alignment_job_scope_invalid")
				}
				mode := 0
				if len(folding) > 0 {
					mode = folding[0]
				}
				if !scopeIncluded(scopes, canonicalJobScope(local.Schema.Database, local.Schema.Table, mode)) {
					continue
				}
			}
		}
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		jobs = append(jobs, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var proofs []CutoverProof
	for _, item := range jobs {
		proof, receipt, peer, err := readCutover(ctx, db, item.id)
		if err != nil || receipt.Phase != CutoverActive {
			return nil, fmt.Errorf("alignment_cutover_pending: job_id=%s: %w", item.id, errors.Join(errors.New("active proof required"), err))
		}
		id, _, _, err := jobIdentity(proof.Plan, item.node)
		peerNode, peerErr := proof.Peer(item.node)
		if err != nil || peerErr != nil || id != item.id || peer != peerNode {
			return nil, errors.New("alignment_cutover_identity_mismatch")
		}
		job, err := ReadSnapshotJob(ctx, db, proof.Plan, item.node)
		local, _ := proof.Local(item.node)
		if err != nil || job.Result == nil || *job.Result != proof.Result || job.CaptureBoundary == nil || *job.CaptureBoundary != local {
			return nil, errors.Join(errors.New("alignment_cutover_local_state_changed"), err)
		}
		proofs = append(proofs, proof)
	}
	return loadActiveTopologyProofs(ctx, db, proofs, scopes...)
}

func isMissingTable(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1146
}

// Retire only the uniqueness reservation after ACTIVE, never the job evidence.
// This permits another member's snapshot while keeping one unresolved copy per table.
func finishSnapshotScope(ctx context.Context, db *sql.DB, proof CutoverProof, node string) error {
	id, _, _, err := jobIdentity(proof.Plan, node)
	if err != nil {
		return err
	}
	stored, receipt, _, err := readCutover(ctx, db, id)
	if err != nil || receipt.Phase != CutoverActive || stored.ID != proof.ID {
		return errors.Join(errors.New("alignment_scope_release_requires_active"), err)
	}
	_, err = db.ExecContext(ctx, "UPDATE sync_alignment_job SET scope_hash=? WHERE job_id=? AND phase IN (?,?)", hash([]string{"active", id}), id, JobTargetConfirmed, JobTargetCommitted)
	return err
}

// Existing versions or tombstones cannot be silently discarded by first-copy.
func CheckInitialConflictState(ctx context.Context, db *sql.DB, database, table string) error {
	var found int
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sync_row_version WHERE JSON_UNQUOTE(JSON_EXTRACT(CONVERT(row_identity USING utf8mb4),'$.database'))=? AND JSON_UNQUOTE(JSON_EXTRACT(CONVERT(row_identity USING utf8mb4),'$.table'))=? LIMIT 1)", database, table).Scan(&found)
	if err != nil {
		return err
	}
	if found != 0 {
		return errors.New("alignment_existing_conflict_history: first-copy cannot replace existing versions or tombstones")
	}
	return nil
}
