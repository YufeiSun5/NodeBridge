package alignment

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
)

const (
	JobPrepared        = "PREPARED"
	JobSourceReady     = "SOURCE_READY"
	JobTargetCommitted = "TARGET_COMMITTED"
	JobTargetConfirmed = "TARGET_CONFIRMED"
)

// SnapshotJob is a durable fence, not authorization to enable CDC. No phase here
// is COMPLETE: the later cutover must install and verify both capture watermarks.
type SnapshotJob struct {
	ID              string            `json:"job_id"`
	NodeID          string            `json:"node_id"`
	Role            string            `json:"endpoint_role"`
	Phase           string            `json:"phase"`
	Plan            Plan              `json:"plan"`
	Result          *CopyResult       `json:"result,omitempty"`
	CaptureMarker   string            `json:"capture_marker,omitempty"`
	CaptureBoundary *SnapshotBoundary `json:"capture_boundary,omitempty"`
}

func jobIdentity(plan Plan, nodeID string) (id, scope, role string, err error) {
	var local Observation
	switch nodeID {
	case plan.Source.NodeID:
		local, role = plan.Source, "SOURCE"
	case plan.Target.NodeID:
		local, role = plan.Target, "TARGET"
	default:
		return "", "", "", errors.New("alignment_job_node_mismatch")
	}
	return hash([]string{plan.ID, nodeID}), hash([]string{local.Schema.Database, local.Schema.Table}), role, nil
}

// PrepareSnapshotJob must be called while owning the local Agent maintenance
// lease. A second plan cannot displace the same table's unresolved job.
func PrepareSnapshotJob(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, nodeID string, confirm bool) (SnapshotJob, error) {
	if err := validateStreamPlan(plan, rule, confirm); err != nil {
		return SnapshotJob{}, err
	}
	if db == nil {
		return SnapshotJob{}, errors.New("alignment_endpoint_required")
	}
	id, _, role, err := jobIdentity(plan, nodeID)
	if err != nil {
		return SnapshotJob{}, err
	}
	scope, err := databaseJobScope(ctx, db, plan, nodeID)
	if err != nil {
		return SnapshotJob{}, err
	}
	b, err := json.Marshal(plan)
	if err != nil {
		return SnapshotJob{}, err
	}
	_, err = db.ExecContext(ctx, "INSERT INTO sync_alignment_job (job_id,scope_hash,plan_id,node_id,endpoint_role,phase,plan_json,updated_at) VALUES (?,?,?,?,?,?,?,UTC_TIMESTAMP(6))", id, scope, plan.ID, nodeID, role, JobPrepared, string(b))
	if err != nil {
		return SnapshotJob{}, fmt.Errorf("alignment_job_prepare_failed: inspect existing job before retry: %w", err)
	}
	return SnapshotJob{ID: id, NodeID: nodeID, Role: role, Phase: JobPrepared, Plan: plan}, nil
}

type jobQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func databaseJobScope(ctx context.Context, query jobQuery, plan Plan, nodeID string) (string, error) {
	local := plan.Source
	if nodeID == plan.Target.NodeID {
		local = plan.Target
	}
	var folding int
	if err := query.QueryRowContext(ctx, "SELECT @@lower_case_table_names").Scan(&folding); err != nil {
		return "", err
	}
	if folding < 0 || folding > 2 {
		return "", errors.New("alignment_table_case_mode_invalid")
	}
	return canonicalJobScope(local.Schema.Database, local.Schema.Table, folding), nil
}

func canonicalJobScope(database, table string, folding int) string {
	if folding != 0 {
		database, table = strings.ToLower(database), strings.ToLower(table)
	}
	return hash([]string{database, table})
}

func ReadSnapshotJob(ctx context.Context, db *sql.DB, plan Plan, nodeID string) (SnapshotJob, error) {
	if db == nil {
		return SnapshotJob{}, errors.New("alignment_endpoint_required")
	}
	return readSnapshotJob(ctx, db, plan, nodeID, false)
}

func readSnapshotJob(ctx context.Context, query jobQuery, plan Plan, nodeID string, lock bool) (SnapshotJob, error) {
	id, _, role, err := jobIdentity(plan, nodeID)
	if err != nil {
		return SnapshotJob{}, err
	}
	scope, err := databaseJobScope(ctx, query, plan, nodeID)
	if err != nil {
		return SnapshotJob{}, err
	}
	statement := "SELECT scope_hash,plan_id,node_id,endpoint_role,phase,plan_json,row_count,row_digest,capture_marker,capture_boundary FROM sync_alignment_job WHERE job_id=?"
	if lock {
		statement += " FOR UPDATE"
	}
	var actualScope, planID, actualNode, actualRole, phase string
	var b []byte
	var count sql.NullInt64
	var digest sql.NullString
	var marker sql.NullString
	var encodedBoundary []byte
	err = query.QueryRowContext(ctx, statement, id).Scan(&actualScope, &planID, &actualNode, &actualRole, &phase, &b, &count, &digest, &marker, &encodedBoundary)
	if err != nil {
		return SnapshotJob{}, err
	}
	var stored Plan
	if err := json.Unmarshal(b, &stored); err != nil {
		return SnapshotJob{}, err
	}
	if actualScope != scope || planID != plan.ID || actualNode != nodeID || actualRole != role || hash(stored) != hash(plan) {
		return SnapshotJob{}, errors.New("alignment_job_identity_changed")
	}
	job := SnapshotJob{ID: id, NodeID: nodeID, Role: role, Phase: phase, Plan: plan}
	if marker.Valid {
		if len(marker.String) != 64 {
			return SnapshotJob{}, errors.New("alignment_job_marker_invalid")
		}
		if _, err := hex.DecodeString(marker.String); err != nil {
			return SnapshotJob{}, errors.New("alignment_job_marker_invalid")
		}
		job.CaptureMarker = marker.String
	}
	if len(encodedBoundary) != 0 {
		var boundary SnapshotBoundary
		if err := json.Unmarshal(encodedBoundary, &boundary); err != nil {
			return SnapshotJob{}, err
		}
		local := plan.Source
		if role == "TARGET" {
			local = plan.Target
		}
		if boundary.Validate() != nil || !marker.Valid || boundary.Token != marker.String || boundary.OriginNodeID != nodeID || boundary.DatabaseName != local.Schema.Database || boundary.TableName != local.Schema.Table {
			return SnapshotJob{}, errors.New("alignment_job_boundary_invalid")
		}
		var serverUUID string
		if err := query.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&serverUUID); err != nil {
			return SnapshotJob{}, err
		}
		if boundary.MySQLServerUUID != serverUUID {
			return SnapshotJob{}, errors.New("alignment_job_server_changed")
		}
		job.CaptureBoundary = &boundary
	}
	switch phase {
	case JobPrepared:
		if count.Valid || digest.Valid {
			return SnapshotJob{}, errors.New("alignment_job_result_invalid")
		}
		if role == "TARGET" && marker.Valid {
			return SnapshotJob{}, errors.New("alignment_job_marker_without_commit")
		}
	case JobSourceReady, JobTargetCommitted, JobTargetConfirmed:
		if !count.Valid || !digest.Valid || count.Int64 < 0 || (phase == JobTargetCommitted && role != "TARGET") || (phase != JobTargetCommitted && role != "SOURCE") {
			return SnapshotJob{}, errors.New("alignment_job_result_invalid")
		}
		if role == "SOURCE" && marker.Valid && job.CaptureBoundary == nil {
			return SnapshotJob{}, errors.New("alignment_job_source_boundary_missing")
		}
		result := CopyResult{PlanID: plan.ID, Rows: count.Int64, Digest: digest.String}
		if err := validateFrame(SnapshotFrame{PlanID: plan.ID, Sequence: count.Int64, End: &result}); err != nil {
			return SnapshotJob{}, err
		}
		job.Result = &result
	default:
		return SnapshotJob{}, errors.New("alignment_job_phase_invalid")
	}
	return job, nil
}

// CheckPendingJobs is fail-closed for every existing job until cutover is built.
// A pre-feature database without this table has no jobs; other SQL errors fail.
func CheckPendingJobs(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("alignment_endpoint_required")
	}
	var id string
	err := db.QueryRowContext(ctx, "SELECT job_id FROM sync_alignment_job LIMIT 1").Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1146 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("alignment_job_check_failed: %w", err)
	}
	return fmt.Errorf("alignment_cutover_pending: job_id=%s", id)
}

func (r *SnapshotReceiver) recordCommit(ctx context.Context, result CopyResult) error {
	if r.jobID == "" {
		return nil
	}
	if r.capture != nil {
		token, err := r.capture.WriteMarker(ctx, r.tx)
		if err != nil {
			return err
		}
		if err := persistCaptureMarker(ctx, r.tx, r.jobID, token); err != nil {
			return err
		}
		r.marker = token
	}
	updated, err := r.tx.ExecContext(ctx, "UPDATE sync_alignment_job SET phase=?,row_count=?,row_digest=?,updated_at=UTC_TIMESTAMP(6) WHERE job_id=? AND phase=?", JobTargetCommitted, result.Rows, result.Digest, r.jobID, JobPrepared)
	if err != nil {
		return err
	}
	n, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("alignment_job_transition_failed")
	}
	return nil
}

// NewPreparedSnapshotReceiver couples the target receipt with the copied rows.
func NewPreparedSnapshotReceiver(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, nodeID string, confirm bool) (*SnapshotReceiver, error) {
	if nodeID != plan.Target.NodeID {
		return nil, errors.New("alignment_job_node_mismatch")
	}
	r, err := NewSnapshotReceiver(ctx, db, plan, rule, confirm)
	if err != nil {
		return nil, err
	}
	job, err := readSnapshotJob(ctx, r.tx, plan, nodeID, true)
	if err == nil && job.Phase != JobPrepared {
		err = errors.New("alignment_job_not_prepared")
	}
	if err != nil {
		r.Close()
		return nil, err
	}
	r.jobID = job.ID
	return r, nil
}

// ExportPreparedSnapshot keeps its durable fence even after a valid receipt.
// TARGET_CONFIRMED describes the copy, not readiness for incremental replay.
func ExportPreparedSnapshot(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, nodeID string, confirm bool, send func(context.Context, SnapshotFrame) (CopyResult, error)) (CopyResult, error) {
	return exportPreparedSnapshot(ctx, db, plan, rule, nodeID, confirm, send, nil)
}

func exportPreparedSnapshot(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, nodeID string, confirm bool, send func(context.Context, SnapshotFrame) (CopyResult, error), probe *SnapshotCapture) (CopyResult, error) {
	if err := validateStreamPlan(plan, rule, confirm); err != nil {
		return CopyResult{}, err
	}
	if nodeID != plan.Source.NodeID {
		return CopyResult{}, errors.New("alignment_job_node_mismatch")
	}
	if send == nil {
		return CopyResult{}, errors.New("alignment_endpoint_required")
	}
	job, err := ReadSnapshotJob(ctx, db, plan, nodeID)
	if err != nil {
		return CopyResult{}, err
	}
	if job.Phase != JobPrepared {
		return CopyResult{}, errors.New("alignment_job_not_prepared")
	}
	if job.CaptureMarker != "" {
		return CopyResult{}, errors.New("alignment_source_capture_already_started")
	}
	var beforeCopy func(context.Context) error
	if probe != nil {
		if err := probe.requireEndpoint(plan, nodeID); err != nil {
			return CopyResult{}, err
		}
		beforeCopy = func(ctx context.Context) error {
			return prepareSourceCapture(ctx, db, job.ID, probe)
		}
	}
	result, err := exportSnapshot(ctx, db, plan, rule, confirm, func(ctx context.Context, frame SnapshotFrame) (CopyResult, error) {
		if frame.End != nil {
			updated, err := db.ExecContext(ctx, "UPDATE sync_alignment_job SET phase=?,row_count=?,row_digest=?,updated_at=UTC_TIMESTAMP(6) WHERE job_id=? AND phase=?", JobSourceReady, frame.End.Rows, frame.End.Digest, job.ID, JobPrepared)
			if err := requireJobUpdate(updated, err); err != nil {
				return CopyResult{}, fmt.Errorf("alignment_source_manifest_record_failed: %w", err)
			}
		}
		return send(ctx, frame)
	}, beforeCopy)
	if err != nil {
		return result, err
	}
	updated, err := db.ExecContext(ctx, "UPDATE sync_alignment_job SET phase=?,updated_at=UTC_TIMESTAMP(6) WHERE job_id=? AND phase=? AND row_count=? AND row_digest=?", JobTargetConfirmed, job.ID, JobSourceReady, result.Rows, result.Digest)
	if err != nil {
		return result, fmt.Errorf("alignment_source_receipt_record_failed: %w", err)
	}
	n, err := updated.RowsAffected()
	if err != nil {
		return result, err
	}
	if n != 1 {
		return result, errors.New("alignment_job_transition_failed")
	}
	return result, nil
}
