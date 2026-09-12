package alignment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	hashpkg "hash"
	"io"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

const MaxSnapshotFrameBytes = 1024 * 1024

// SnapshotFrame carries bytes, not JSON numbers, so transfer is lossless.
// End is a commit request; broker acceptance alone is not a commit receipt.
type SnapshotFrame struct {
	PlanID   string      `json:"plan_id"`
	Sequence int64       `json:"sequence"`
	Values   [][]byte    `json:"values,omitempty"`
	End      *CopyResult `json:"end,omitempty"`
}

func EncodeSnapshotFrame(frame SnapshotFrame) ([]byte, error) {
	if err := validateFrame(frame); err != nil {
		return nil, err
	}
	size := len(frame.Values)
	for _, value := range frame.Values {
		if len(value) > MaxSnapshotFrameBytes-size {
			return nil, errors.New("alignment_frame_too_large")
		}
		size += len(value)
	}
	if size > MaxSnapshotFrameBytes {
		return nil, errors.New("alignment_frame_too_large")
	}
	b, err := json.Marshal(frame)
	if len(b) > MaxSnapshotFrameBytes {
		return nil, errors.New("alignment_frame_too_large")
	}
	return b, err
}

func DecodeSnapshotFrame(b []byte) (SnapshotFrame, error) {
	var frame SnapshotFrame
	if len(b) > MaxSnapshotFrameBytes {
		return frame, errors.New("alignment_frame_too_large")
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err := d.Decode(&frame); err != nil {
		return frame, err
	}
	// A frame is exactly one JSON document.
	if d.Decode(new(any)) != io.EOF {
		return SnapshotFrame{}, errors.New("alignment_frame_trailing_data")
	}
	return frame, validateFrame(frame)
}

func validateFrame(frame SnapshotFrame) error {
	if len(frame.PlanID) != 64 || frame.Sequence < 0 {
		return errors.New("alignment_frame_identity_invalid")
	}
	if _, err := hex.DecodeString(frame.PlanID); err != nil {
		return errors.New("alignment_frame_identity_invalid")
	}
	if frame.End != nil {
		if len(frame.Values) != 0 || frame.End.PlanID != frame.PlanID || frame.End.Rows != frame.Sequence || len(frame.End.Digest) != 64 {
			return errors.New("alignment_frame_end_invalid")
		}
		if _, err := hex.DecodeString(frame.End.Digest); err != nil {
			return errors.New("alignment_frame_end_invalid")
		}
	} else if len(frame.Values) == 0 {
		return errors.New("alignment_frame_row_required")
	}
	return nil
}

func validateStreamPlan(plan Plan, rule rules.SyncRule, confirm bool) error {
	if err := plan.Validate(rule, time.Now(), confirm); err != nil {
		return err
	}
	if rule.Enable {
		return errors.New("alignment_requires_disabled_rule")
	}
	return nil
}

func lockObserved(ctx context.Context, tx *sql.Tx, observation Observation) (int64, error) {
	n, err := lockSnapshotTable(ctx, tx, observation.Schema)
	if err != nil {
		return 0, err
	}
	if (n != 0) != observation.HasRows {
		return 0, errors.New("alignment_endpoint_emptiness_changed")
	}
	actual, err := rulecheck.ReadSchema(ctx, tx, observation.Schema.Database, observation.Schema.Table)
	if err != nil {
		return 0, err
	}
	if hash(actual) != hash(observation.Schema) {
		return 0, errors.New("alignment_schema_changed")
	}
	return n, nil
}

// ExportSnapshot holds the local source locks through the final commit receipt.
// It needs only this endpoint's DB. A transport must return the target's receipt,
// not a publisher confirm. Durable recovery and CDC cutover remain caller duties.
func ExportSnapshot(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, confirm bool, send func(context.Context, SnapshotFrame) (CopyResult, error)) (CopyResult, error) {
	return exportSnapshot(ctx, db, plan, rule, confirm, send, nil)
}

func exportSnapshot(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, confirm bool, send func(context.Context, SnapshotFrame) (CopyResult, error), beforeCopy func(context.Context) error) (CopyResult, error) {
	result := CopyResult{PlanID: plan.ID}
	if err := validateStreamPlan(plan, rule, confirm); err != nil {
		return result, err
	}
	if db == nil || send == nil {
		return result, errors.New("alignment_endpoint_required")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	tx, release, err := beginSnapshotTx(ctx, db)
	if err != nil {
		return result, err
	}
	defer release()
	n, err := lockObserved(ctx, tx, plan.Source)
	if err != nil {
		return result, err
	}
	if beforeCopy != nil {
		if err := beforeCopy(ctx); err != nil {
			return result, err
		}
	}
	columns, _, _ := copyColumns(plan)
	h := sha256.New()
	var sequence int64
	count, err := scanSnapshot(ctx, tx, plan.Source.Schema, columns, plan.Source.Schema.PrimaryKeys, func(values [][]byte) error {
		frame := SnapshotFrame{PlanID: plan.ID, Sequence: sequence, Values: values}
		if _, err := EncodeSnapshotFrame(frame); err != nil {
			return err
		}
		writeRowDigest(h, values)
		if _, err := send(ctx, frame); err != nil {
			return err
		}
		sequence++
		return nil
	})
	if err != nil {
		return result, err
	}
	if count != n {
		return result, errors.New("alignment_source_count_changed")
	}
	result.Rows, result.Digest = count, hex.EncodeToString(h.Sum(nil))
	end := result
	receipt, err := send(ctx, SnapshotFrame{PlanID: plan.ID, Sequence: count, End: &end})
	if err != nil {
		return result, fmt.Errorf("alignment_commit_unknown: %w", err)
	}
	if receipt != result {
		return result, errors.New("alignment_commit_unknown: invalid target receipt")
	}
	return receipt, nil
}

// SnapshotReceiver is a single-goroutine, local transaction, not a resumable job.
// Close rolls back incomplete transfers. Never expose it without durable job
// fencing: a lost final receipt must not authorize a second copy or Agent start.
type SnapshotReceiver struct {
	db      *sql.DB
	tx      *sql.Tx
	stmt    *sql.Stmt
	plan    Plan
	release func()
	cancel  context.CancelFunc
	hash    hashpkg.Hash
	rows    int64
	closed  bool
	jobID   string
	capture *SnapshotCapture
	marker  string
}

func NewSnapshotReceiver(ctx context.Context, db *sql.DB, plan Plan, rule rules.SyncRule, confirm bool) (*SnapshotReceiver, error) {
	if err := validateStreamPlan(plan, rule, confirm); err != nil {
		return nil, err
	}
	if db == nil || plan.Target.HasRows {
		return nil, errors.New("alignment_empty_target_required")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	tx, release, err := beginSnapshotTx(ctx, db)
	if err != nil {
		cancel()
		return nil, err
	}
	r := &SnapshotReceiver{db: db, tx: tx, release: release, cancel: cancel, plan: plan, hash: sha256.New()}
	ok := false
	defer func() {
		if !ok {
			r.Close()
		}
	}()
	if _, err := lockObserved(ctx, tx, plan.Target); err != nil {
		return nil, err
	}
	if err := rejectTargetSideEffects(ctx, tx, plan.Target.Schema); err != nil {
		return nil, err
	}
	_, columns, _ := copyColumns(plan)
	query := "INSERT INTO " + qualified(plan.Target.Schema) + " (" + quoteColumns(columns) + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + ")"
	r.stmt, err = tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	ok = true
	return r, nil
}

func (r *SnapshotReceiver) Close() {
	if r.closed {
		return
	}
	r.closed = true
	if r.stmt != nil {
		_ = r.stmt.Close()
	}
	r.release()
	r.cancel()
}

func (r *SnapshotReceiver) Accept(ctx context.Context, frame SnapshotFrame) (result CopyResult, err error) {
	if r.closed {
		return result, errors.New("alignment_receiver_closed")
	}
	defer func() {
		if err != nil {
			r.Close()
		}
	}()
	if _, err := EncodeSnapshotFrame(frame); err != nil {
		return result, err
	}
	if frame.PlanID != r.plan.ID || frame.Sequence != r.rows {
		return result, errors.New("alignment_frame_out_of_order")
	}
	if frame.End == nil {
		if len(frame.Values) != len(r.plan.Columns) {
			return result, errors.New("alignment_frame_columns_invalid")
		}
		args := make([]any, len(frame.Values))
		for i, value := range frame.Values {
			if value != nil {
				args[i] = value
			}
		}
		if _, err := r.stmt.ExecContext(ctx, args...); err != nil {
			return result, err
		}
		writeRowDigest(r.hash, frame.Values)
		r.rows++
		return result, nil
	}
	result = CopyResult{PlanID: r.plan.ID, Rows: r.rows, Digest: hex.EncodeToString(r.hash.Sum(nil))}
	if *frame.End != result {
		return CopyResult{}, errors.New("alignment_stream_digest_mismatch")
	}
	_, columns, order := copyColumns(r.plan)
	actual := sha256.New()
	n, err := scanSnapshot(ctx, r.tx, r.plan.Target.Schema, columns, order, func(values [][]byte) error {
		writeRowDigest(actual, values)
		return nil
	})
	if err != nil {
		return CopyResult{}, err
	}
	if n != r.rows || hex.EncodeToString(actual.Sum(nil)) != result.Digest {
		return CopyResult{}, errors.New("alignment_copy_verification_failed")
	}
	if err := r.recordCommit(ctx, result); err != nil {
		return CopyResult{}, err
	}
	if err := r.tx.Commit(); err != nil {
		return CopyResult{}, fmt.Errorf("alignment_commit_unknown: %w", err)
	}
	r.Close()
	if r.capture != nil {
		boundary, err := r.capture.WaitMarker(ctx, r.marker)
		if err != nil {
			return result, fmt.Errorf("alignment_capture_after_commit_failed: %w", err)
		}
		if err := persistCaptureBoundary(ctx, r.db, r.jobID, JobTargetCommitted, boundary); err != nil {
			return result, fmt.Errorf("alignment_capture_after_commit_failed: %w", err)
		}
	}
	return result, nil
}
