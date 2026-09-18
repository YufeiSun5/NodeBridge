package alignment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc/canal"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/rabbitmq/amqp091-go"
)

type PairSessionOptions struct {
	NodeID      string
	PeerID      string
	IsEdge      bool
	Rule        rules.SyncRule
	DB          *sql.DB
	Broker      *amqp091.Connection
	Canal       canal.Config
	Confirm     bool
	Ready       func(context.Context, CutoverProof) error
	Progress    func(string)
	BeforePlan  func(context.Context, *PairControl, Observation, Observation) error
	AfterActive func(context.Context, *PairControl, CutoverProof) error
	Timeout     time.Duration
}

type pairState struct {
	Observation Observation  `json:"observation"`
	Job         *SnapshotJob `json:"job,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

// RunPairSession must own the existing local Agent configuration lock for its
// entire lifetime. Both endpoints are explicitly armed; it never stops an Agent.
func RunPairSession(ctx context.Context, options PairSessionOptions) (proof CutoverProof, runErr error) {
	if !options.Confirm {
		return proof, errors.New("alignment_confirmation_required")
	}
	if options.DB == nil || options.Broker == nil || options.Ready == nil {
		return proof, errors.New("alignment_session_dependencies_required")
	}
	rule := canonicalCutoverRule(options.Rule)
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = SnapshotTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// Broker RPCs have no per-call context. This operation owns its connection.
	stopped := make(chan struct{})
	stopIO := context.AfterFunc(ctx, func() { _ = options.Broker.CloseDeadline(time.Now().Add(time.Second)); close(stopped) })
	defer func() {
		if !stopIO() {
			<-stopped
		}
	}()
	report := func(stage string) {
		if options.Progress != nil {
			options.Progress(stage)
		}
	}
	report("waiting_peer")
	control, err := OpenPairControl(ctx, options.Broker, options.NodeID, options.PeerID, rule, options.IsEdge)
	if err != nil {
		return proof, err
	}
	defer control.Close()
	defer func() {
		if runErr != nil {
			failureCtx, done := context.WithTimeout(context.Background(), time.Second)
			defer done()
			_ = control.send(failureCtx, sessionQueue(control.peerNonce), "error", "local operation failed; inspect the peer's local status")
		}
	}()
	report("checking")
	database, table := rule.DatabaseName, rule.TableName
	if !options.IsEdge {
		if rule.TargetDatabaseName != "" {
			database = rule.TargetDatabaseName
		}
		if rule.TargetTableName != "" {
			table = rule.TargetTableName
		}
	}
	observed, err := Observe(ctx, options.DB, options.NodeID, database, table)
	if err != nil {
		return proof, err
	}
	existing, err := FindSnapshotJob(ctx, options.DB, options.NodeID, rule.ID, options.PeerID)
	if err != nil {
		return proof, err
	}
	localState := pairState{Observation: observed, Job: existing, CreatedAt: time.Now().UTC()}
	var peerState pairState
	if err := control.Exchange(ctx, "state", localState, &peerState); err != nil {
		return proof, err
	}
	if peerState.Observation.NodeID != options.PeerID {
		return proof, errors.New("alignment_peer_observation_mismatch")
	}
	// Reject stale history before BeforePlan persists a new topology intent.
	if err := exchangeGeneration(ctx, options.DB, control, rule, options.NodeID, options.PeerID); err != nil {
		return proof, err
	}
	generation, err := readGeneration(ctx, options.DB)
	if err != nil {
		return proof, err
	}
	if err := checkExistingPairPlans(rule, localState.Job, peerState.Job); err != nil {
		return proof, err
	}
	if options.BeforePlan != nil {
		if err := options.BeforePlan(ctx, control, localState.Observation, peerState.Observation); err != nil {
			return proof, err
		}
		localState.Observation, err = Observe(ctx, options.DB, options.NodeID, database, table)
		if err != nil {
			return proof, err
		}
		localState.CreatedAt = time.Now().UTC()
		if err := control.Exchange(ctx, "ordered_state", localState, &peerState); err != nil {
			return proof, err
		}
		if peerState.Observation.NodeID != options.PeerID {
			return proof, errors.New("alignment_peer_observation_mismatch")
		}
	}
	// Retain a cancelled job only while its peer still owns that same attempt.
	localState.Job, peerState.Job = pendingPairJobs(localState.Job, peerState.Job)
	existing = localState.Job
	if existing == nil {
		if err := CheckInitialConflictState(ctx, options.DB, database, table); err != nil {
			return proof, err
		}
	}
	edgeState, serverState := localState, peerState
	if !options.IsEdge {
		edgeState, serverState = peerState, localState
	}
	var plan Plan
	if generation != nil && localState.Job == nil && peerState.Job == nil && serverState.Observation.HasRows {
		return proof, errors.New("rebaseline_target_not_empty: stop target writers before alignment")
	}
	if existing != nil {
		plan = existing.Plan
	} else if peerState.Job != nil {
		plan = peerState.Job.Plan
	} else {
		plan, err = BuildPlan(rule, edgeState.Observation, serverState.Observation, edgeState.CreatedAt)
		if err != nil {
			return proof, err
		}
	}
	if err := plan.Validate(rule, plan.CreatedAt, true); err != nil {
		return proof, err
	}
	if _, _, _, err := jobIdentity(plan, options.NodeID); err != nil {
		return proof, err
	}
	if _, _, _, err := jobIdentity(plan, options.PeerID); err != nil {
		return proof, err
	}
	for _, state := range []pairState{localState, peerState} {
		local := plan.Source
		if state.Observation.NodeID == plan.Target.NodeID {
			local = plan.Target
		}
		if local.NodeID != state.Observation.NodeID || hash(local.Schema) != hash(state.Observation.Schema) || (state.Job != nil && hash(state.Job.Plan) != hash(plan)) {
			return proof, errors.New("alignment_session_plan_or_schema_changed")
		}
	}
	if existing == nil {
		job, err := prepareSnapshotJob(ctx, options.DB, plan, rule, options.NodeID, true, peerState.Job != nil)
		if err != nil {
			return proof, err
		}
		existing = &job
	}
	var peerJob SnapshotJob
	if err := control.Exchange(ctx, "prepared", existing, &peerJob); err != nil {
		return proof, err
	}
	if hash(peerJob.Plan) != hash(plan) || peerJob.NodeID != options.PeerID {
		return proof, errors.New("alignment_peer_job_mismatch")
	}
	sourceJob, targetJob := *existing, peerJob
	if options.NodeID == plan.Target.NodeID {
		sourceJob, targetJob = peerJob, *existing
	}
	if targetJob.Phase == JobCancelled || sourceJob.Phase == JobCancelled ||
		(targetJob.Phase == JobPrepared && (sourceJob.Phase != JobPrepared || sourceJob.CaptureMarker != "" || !time.Now().Before(plan.ExpiresAt))) {
		report("cancelling_uncommitted")
		if existing.Role == "TARGET" {
			job, err := CancelUncommittedJob(ctx, options.DB, plan, options.NodeID, peerJob)
			if err != nil {
				return proof, err
			}
			existing = &job
		}
		if err := control.Exchange(ctx, "target_cancelled", existing, &peerJob); err != nil {
			return proof, err
		}
		if existing.Role == "SOURCE" {
			job, err := CancelUncommittedJob(ctx, options.DB, plan, options.NodeID, peerJob)
			if err != nil {
				return proof, err
			}
			existing = &job
		}
		if err := control.Exchange(ctx, "cancelled", existing, &peerJob); err != nil {
			return proof, err
		}
		return proof, errors.New("alignment_uncommitted_attempt_cancelled: retry on both endpoints to create a fresh plan; original evidence retained")
	}
	if targetJob.Phase == JobPrepared {
		if sourceJob.Phase != JobPrepared || sourceJob.CaptureMarker != "" {
			return proof, errors.New("alignment_retry_requires_cancel: prior attempt did not commit a target receipt")
		}
		report("copying")
		client, err := canal.NewWithlinClient(options.Canal)
		if err != nil {
			return proof, err
		}
		probe, err := StartSnapshotCapture(ctx, options.DB, client, options.Canal.Destination, plan, rule, options.NodeID, true)
		if err != nil {
			return proof, err
		}
		if options.NodeID == plan.Source.NodeID {
			_, err = SendCapturedSnapshotAMQP(ctx, options.DB, options.Broker, plan, rule, options.NodeID, true, probe)
		} else {
			_, err = ReceiveCapturedSnapshotAMQP(ctx, options.DB, options.Broker, plan, rule, options.NodeID, true, probe)
		}
		copyErr := err
		job, readErr := ReadSnapshotJob(ctx, options.DB, plan, options.NodeID)
		if readErr != nil {
			_ = probe.Close()
			return proof, errors.Join(copyErr, readErr)
		}
		if copyErr != nil && job.Phase != JobSourceReady && job.Phase != JobTargetCommitted {
			_ = probe.Close()
			return proof, copyErr
		}
		if job.Phase == JobTargetCommitted && job.CaptureBoundary == nil {
			job, err = RecoverTargetCapture(ctx, options.DB, plan, rule, options.NodeID, true, probe)
		}
		closeErr := probe.Close()
		if err != nil && job.CaptureBoundary == nil {
			return proof, errors.Join(err, closeErr)
		}
		if closeErr != nil {
			return proof, closeErr
		}
		existing = &job
	} else if targetJob.Phase != JobTargetCommitted {
		return proof, errors.New("alignment_target_outcome_invalid")
	}
	if existing.Role == "TARGET" && existing.Phase == JobTargetCommitted && existing.CaptureBoundary == nil {
		report("recovering")
		client, err := canal.NewWithlinClient(options.Canal)
		if err != nil {
			return proof, err
		}
		probe, err := StartSnapshotRecoveryCapture(ctx, options.DB, client, options.Canal.Destination, plan, rule, options.NodeID, true)
		if err != nil {
			return proof, err
		}
		job, recoverErr := RecoverTargetCapture(ctx, options.DB, plan, rule, options.NodeID, true, probe)
		closeErr := probe.Close()
		if recoverErr != nil || closeErr != nil {
			return proof, errors.Join(recoverErr, closeErr)
		}
		existing = &job
	}
	if err := control.Exchange(ctx, "copied", existing, &peerJob); err != nil {
		return proof, err
	}
	if existing.Role == "SOURCE" {
		job, err := ReconcileSnapshotReceipt(ctx, options.DB, plan, rule, options.NodeID, peerJob, true)
		if err != nil {
			return proof, err
		}
		existing = &job
	}
	if err := control.Exchange(ctx, "confirmed", existing, &peerJob); err != nil {
		return proof, err
	}
	sourceJob, targetJob = *existing, peerJob
	if options.NodeID == plan.Target.NodeID {
		sourceJob, targetJob = peerJob, *existing
	}
	proof, err = BuildCutoverProof(rule, sourceJob, targetJob)
	if err != nil {
		return proof, err
	}
	report("preparing_incremental")
	if err := options.Ready(ctx, proof); err != nil {
		return proof, err
	}
	receipt, err := InstallCutover(ctx, options.DB, proof, options.NodeID)
	if err != nil {
		return proof, err
	}
	var peerReceipt CutoverReceipt
	if err := control.Exchange(ctx, "ready", receipt, &peerReceipt); err != nil {
		return proof, err
	}
	if err := ActivateCutover(ctx, options.DB, proof, options.NodeID, peerReceipt); err != nil {
		return proof, err
	}
	receipt.Phase = CutoverActive
	if err := control.Exchange(ctx, "active", receipt, &peerReceipt); err != nil {
		return proof, err
	}
	if peerReceipt.Phase != CutoverActive || peerReceipt.ProofID != proof.ID || peerReceipt.NodeID != options.PeerID {
		return proof, errors.New("alignment_peer_activation_invalid")
	}
	if err := finishSnapshotScope(ctx, options.DB, proof, options.NodeID); err != nil {
		return proof, err
	}
	if options.AfterActive != nil {
		if err := options.AfterActive(ctx, control, proof); err != nil {
			return proof, err
		}
	}
	report("completed")
	return proof, nil
}

func FindSnapshotJob(ctx context.Context, db *sql.DB, node, ruleID string, peer ...string) (*SnapshotJob, error) {
	rows, err := db.QueryContext(ctx, "SELECT plan_json,phase FROM sync_alignment_job WHERE node_id=? ORDER BY updated_at DESC,job_id DESC", node)
	if err != nil {
		return nil, err
	}
	var plans []Plan
	var cancelled *Plan
	for rows.Next() {
		var data []byte
		var phase string
		if err := rows.Scan(&data, &phase); err != nil {
			rows.Close()
			return nil, err
		}
		var plan Plan
		if err := json.Unmarshal(data, &plan); err != nil {
			rows.Close()
			return nil, err
		}
		if plan.RuleID == ruleID {
			if len(peer) > 0 && plan.Source.NodeID != peer[0] && plan.Target.NodeID != peer[0] {
				continue
			}
			if phase == JobCancelled {
				if cancelled == nil {
					cancelled = &plan
				}
				continue
			}
			plans = append(plans, plan)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(plans) == 0 {
		if cancelled == nil {
			return nil, nil
		}
		plans = append(plans, *cancelled)
	}
	if len(plans) != 1 {
		return nil, fmt.Errorf("alignment_multiple_jobs_for_rule: %s", ruleID)
	}
	job, err := ReadSnapshotJob(ctx, db, plans[0], node)
	return &job, err
}

func pendingPairJobs(local, peer *SnapshotJob) (*SnapshotJob, *SnapshotJob) {
	originalLocal, originalPeer := local, peer
	if local != nil && local.Phase == JobCancelled && (originalPeer == nil || originalPeer.Phase == JobCancelled || originalPeer.Plan.ID != local.Plan.ID) {
		local = nil
	}
	if peer != nil && peer.Phase == JobCancelled && (originalLocal == nil || originalLocal.Phase == JobCancelled || originalLocal.Plan.ID != peer.Plan.ID) {
		peer = nil
	}
	return local, peer
}
