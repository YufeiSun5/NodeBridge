package alignment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func (p *SnapshotCapture) requireEndpoint(plan Plan, nodeID string) error {
	if p == nil || p.closed || p.planID != plan.ID || p.base.OriginNodeID != nodeID {
		return errors.New("alignment_capture_endpoint_mismatch")
	}
	local := plan.Source
	if nodeID == plan.Target.NodeID {
		local = plan.Target
	}
	if local.NodeID != nodeID || local.Schema.Database != p.base.DatabaseName || local.Schema.Table != p.base.TableName {
		return errors.New("alignment_capture_endpoint_mismatch")
	}
	return nil
}

func persistCaptureMarker(ctx context.Context, tx *sql.Tx, jobID, token string) error {
	result, err := tx.ExecContext(ctx, "UPDATE sync_alignment_job SET capture_marker=?,updated_at=UTC_TIMESTAMP(6) WHERE job_id=? AND phase=? AND capture_marker IS NULL AND capture_boundary IS NULL", token, jobID, JobPrepared)
	return requireJobUpdate(result, err)
}

func requireJobUpdate(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("alignment_job_transition_failed")
	}
	return nil
}

func persistCaptureBoundary(ctx context.Context, db *sql.DB, jobID, phase string, boundary SnapshotBoundary) error {
	if err := boundary.Validate(); err != nil {
		return err
	}
	var serverUUID string
	if err := db.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&serverUUID); err != nil {
		return err
	}
	if serverUUID != boundary.MySQLServerUUID {
		return errors.New("alignment_capture_writer_mismatch")
	}
	b, err := json.Marshal(boundary)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, "UPDATE sync_alignment_job SET capture_boundary=?,updated_at=UTC_TIMESTAMP(6) WHERE job_id=? AND phase=? AND node_id=? AND capture_marker=? AND capture_boundary IS NULL", string(b), jobID, phase, boundary.OriginNodeID, boundary.Token)
	return requireJobUpdate(result, err)
}

// The caller holds the source business table locks throughout marker commit
// and observation. A lost marker receipt leaves the prepared job fenced.
func prepareSourceCapture(ctx context.Context, db *sql.DB, jobID string, probe *SnapshotCapture) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	token, err := probe.WriteMarker(ctx, tx)
	if err != nil {
		return err
	}
	if err := persistCaptureMarker(ctx, tx, jobID, token); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("alignment_source_marker_commit_unknown: %w", err)
	}
	boundary, err := probe.WaitMarker(ctx, token)
	if err != nil {
		return err
	}
	return persistCaptureBoundary(ctx, db, jobID, JobPrepared, boundary)
}

// NewCapturedSnapshotReceiver requires a live exclusive Canal probe started
// before copying. It does not complete cutover or release the durable job fence.
func NewCapturedSnapshotReceiver(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, nodeID string, confirm bool, probe *SnapshotCapture) (*SnapshotReceiver, error) {
	if err := probe.requireEndpoint(plan, nodeID); err != nil {
		return nil, err
	}
	r, err := NewPreparedSnapshotReceiver(ctx, db, plan, rule, nodeID, confirm)
	if err != nil {
		return nil, err
	}
	r.capture = probe
	return r, nil
}

func ExportCapturedSnapshot(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, nodeID string, confirm bool, send func(context.Context, SnapshotFrame) (CopyResult, error), probe *SnapshotCapture) (CopyResult, error) {
	if err := probe.requireEndpoint(plan, nodeID); err != nil {
		return CopyResult{}, err
	}
	return exportPreparedSnapshot(ctx, db, plan, rule, nodeID, confirm, send, probe)
}

// RecoverTargetCapture re-observes the marker already committed with the copy.
// It must never write a replacement pulse and call its newer position a boundary.
func RecoverTargetCapture(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, nodeID string, confirm bool, probe *SnapshotCapture) (SnapshotJob, error) {
	if err := plan.Validate(rule, plan.CreatedAt, confirm); err != nil {
		return SnapshotJob{}, err
	}
	if rule.Enable || nodeID != plan.Target.NodeID {
		return SnapshotJob{}, errors.New("alignment_recovery_endpoint_invalid")
	}
	if err := probe.requireEndpoint(plan, nodeID); err != nil {
		return SnapshotJob{}, err
	}
	job, err := ReadSnapshotJob(ctx, db, plan, nodeID)
	if err != nil {
		return SnapshotJob{}, err
	}
	if job.Phase != JobTargetCommitted || job.CaptureMarker == "" {
		return SnapshotJob{}, errors.New("alignment_recovery_capture_not_committed")
	}
	boundary, err := probe.WaitMarker(ctx, job.CaptureMarker)
	if err != nil {
		return SnapshotJob{}, err
	}
	if job.CaptureBoundary != nil {
		if *job.CaptureBoundary != boundary {
			return SnapshotJob{}, errors.New("alignment_recovery_capture_mismatch")
		}
		return job, nil
	}
	if err := persistCaptureBoundary(ctx, db, job.ID, JobTargetCommitted, boundary); err != nil {
		return SnapshotJob{}, err
	}
	job.CaptureBoundary = &boundary
	return job, nil
}
