package alignment

import (
	"context"
	"database/sql"
	"errors"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// ReconcileSnapshotReceipt only records an already committed copy. The peer
// receipt must come from its authenticated control channel, never a claim from
// an untrusted data frame. This does not release either endpoint's CDC fence.
func ReconcileSnapshotReceipt(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, nodeID string, peer SnapshotJob, confirm bool) (SnapshotJob, error) {
	// Expiry prevents new work, not inspection of an existing durable outcome.
	if err := plan.Validate(rule, plan.CreatedAt, confirm); err != nil {
		return SnapshotJob{}, err
	}
	if rule.Enable || nodeID != plan.Source.NodeID || db == nil {
		return SnapshotJob{}, errors.New("alignment_recovery_endpoint_invalid")
	}
	peerID, _, _, err := jobIdentity(plan, plan.Target.NodeID)
	if err != nil {
		return SnapshotJob{}, err
	}
	if peer.ID != peerID || peer.NodeID != plan.Target.NodeID || peer.Role != "TARGET" || peer.Phase != JobTargetCommitted || hash(peer.Plan) != hash(plan) || peer.Result == nil {
		return SnapshotJob{}, errors.New("alignment_recovery_receipt_invalid")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SnapshotJob{}, err
	}
	defer tx.Rollback()
	job, err := readSnapshotJob(ctx, tx, plan, nodeID, true)
	if err != nil {
		return SnapshotJob{}, err
	}
	if (job.Phase != JobSourceReady && job.Phase != JobTargetConfirmed) || job.Result == nil || *job.Result != *peer.Result {
		return SnapshotJob{}, errors.New("alignment_recovery_manifest_mismatch")
	}
	if (job.CaptureMarker != "") != (peer.CaptureMarker != "") {
		return SnapshotJob{}, errors.New("alignment_recovery_capture_mismatch")
	}
	if job.CaptureMarker != "" {
		b := peer.CaptureBoundary
		if job.CaptureBoundary == nil || b == nil || b.Validate() != nil || b.Token != peer.CaptureMarker || b.OriginNodeID != peer.NodeID || b.DatabaseName != plan.Target.Schema.Database || b.TableName != plan.Target.Schema.Table {
			return SnapshotJob{}, errors.New("alignment_recovery_capture_incomplete")
		}
	}
	if job.Phase == JobTargetConfirmed {
		return job, nil
	}
	updated, err := tx.ExecContext(ctx, "UPDATE sync_alignment_job SET phase=?,updated_at=UTC_TIMESTAMP(6) WHERE job_id=? AND phase=?", JobTargetConfirmed, job.ID, JobSourceReady)
	if err := requireJobUpdate(updated, err); err != nil {
		return SnapshotJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return SnapshotJob{}, err
	}
	job.Phase = JobTargetConfirmed
	return job, nil
}
