package alignment

import (
	"context"
	"database/sql"
	"errors"
)

// CancelUncommittedJob runs under the exclusive pair session and local Agent
// lock. The target's locked receipt is cancelled before the source is released.
func CancelUncommittedJob(ctx context.Context, db *sql.DB, plan Plan, node string, peer SnapshotJob) (SnapshotJob, error) {
	if hash(peer.Plan) != hash(plan) || peer.NodeID == node {
		return SnapshotJob{}, errors.New("alignment_cancel_peer_mismatch")
	}
	peerID, _, peerRole, err := jobIdentity(plan, peer.NodeID)
	if err != nil || peer.ID != peerID || peer.Role != peerRole {
		return SnapshotJob{}, errors.New("alignment_cancel_peer_mismatch")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SnapshotJob{}, err
	}
	defer tx.Rollback()
	job, err := readSnapshotJob(ctx, tx, plan, node, true)
	if err != nil {
		return job, err
	}
	if job.Phase == JobCancelled {
		return job, nil
	}
	if job.Role == "TARGET" {
		if job.Phase != JobPrepared || (peer.Phase != JobPrepared && peer.Phase != JobSourceReady && peer.Phase != JobCancelled) {
			return job, errors.New("alignment_cancel_committed_or_unknown")
		}
	} else if (job.Phase != JobPrepared && job.Phase != JobSourceReady) || peer.Phase != JobCancelled {
		return job, errors.New("alignment_cancel_target_confirmation_required")
	}
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_alignment_cutover WHERE job_id=?", job.ID).Scan(&count); err != nil {
		return job, err
	}
	if count != 0 {
		return job, errors.New("alignment_cancel_cutover_exists")
	}
	result, err := tx.ExecContext(ctx, "UPDATE sync_alignment_job SET phase=?,scope_hash=?,updated_at=UTC_TIMESTAMP(6) WHERE job_id=? AND phase=?", JobCancelled, hash([]string{"cancelled", job.ID}), job.ID, job.Phase)
	if err := requireJobUpdate(result, err); err != nil {
		return job, err
	}
	if err := tx.Commit(); err != nil {
		return job, err
	}
	job.Phase = JobCancelled
	return job, nil
}
