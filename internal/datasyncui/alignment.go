package datasyncui

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/atomicfile"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

type alignmentTask struct {
	mu     sync.Mutex
	status uiapi.InitialAlignmentStatus
	cancel context.CancelFunc
	run    func(context.Context, string, string, alignment.ConfiguredRequest, func(string)) (alignment.CutoverProof, error)
}

func (a *App) GetInitialAlignmentStatus() uiapi.InitialAlignmentStatus {
	a.alignment.mu.Lock()
	defer a.alignment.mu.Unlock()
	if a.alignment.status.Stage != "" {
		return a.alignment.status
	}
	var previous uiapi.InitialAlignmentStatus
	if data, err := os.ReadFile(a.effectiveConfigPath() + ".alignment-status.json"); err == nil && json.Unmarshal(data, &previous) == nil {
		if previous.Running {
			previous.Running, previous.Stage = false, "unknown"
			previous.Message = "The previous operation has no final status in this window; its persisted database outcome must be checked by retry."
		}
		return previous
	}
	return uiapi.InitialAlignmentStatus{Stage: "idle"}
}

func (a *App) StartInitialAlignment(req uiapi.InitialAlignmentRequest) (uiapi.InitialAlignmentStatus, error) {
	if err := a.requireAdmin(); err != nil {
		return uiapi.InitialAlignmentStatus{}, err
	}
	if !req.Confirm || req.RuleID == "" || req.PeerNodeID == "" {
		return uiapi.InitialAlignmentStatus{}, errors.New("alignment_explicit_rule_peer_and_confirmation_required")
	}
	if a.config == nil {
		return uiapi.InitialAlignmentStatus{}, errConfigMissing
	}
	if a.agent != nil && a.agent.Running() {
		return uiapi.InitialAlignmentStatus{}, errors.New("alignment_requires_stopped_agent")
	}
	a.alignment.mu.Lock()
	defer a.alignment.mu.Unlock()
	if a.alignment.cancel != nil {
		return a.alignment.status, errors.New("alignment_operation_running")
	}
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	a.alignment.cancel = cancel
	now := time.Now().UTC().Format(time.RFC3339Nano)
	a.alignment.status = uiapi.InitialAlignmentStatus{RuleID: req.RuleID, PeerNodeID: req.PeerNodeID, Running: true, Stage: "waiting_peer", StartedAt: now, UpdatedAt: now}
	configPath, rulesPath := a.effectiveConfigPath(), a.effectiveRulesPath()
	redact := agentlog.ForConfig(a.config)
	run := a.alignment.run
	if run == nil {
		run = alignment.RunConfigured
	}
	if err := a.persistAlignmentStatusLocked(configPath); err != nil {
		cancel()
		a.alignment.cancel = nil
		a.alignment.status.Running = false
		return a.alignment.status, err
	}
	go func() {
		defer cancel()
		proof, err := run(ctx, configPath, rulesPath, alignment.ConfiguredRequest{RuleID: req.RuleID, PeerID: req.PeerNodeID, Confirm: true}, func(stage string) {
			a.alignment.mu.Lock()
			defer a.alignment.mu.Unlock()
			a.alignment.status.Stage = stage
			a.alignment.status.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			_ = a.persistAlignmentStatusLocked(configPath)
		})
		a.alignment.mu.Lock()
		defer a.alignment.mu.Unlock()
		a.alignment.cancel = nil
		a.alignment.status.Running = false
		a.alignment.status.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err != nil {
			a.alignment.status.Stage, a.alignment.status.Message = "failed", redact(err.Error())
		} else {
			a.alignment.status.Stage, a.alignment.status.PlanID, a.alignment.status.Rows = "completed", proof.Plan.ID, proof.Result.Rows
		}
		_ = a.persistAlignmentStatusLocked(configPath)
	}()
	return a.alignment.status, nil
}

func (a *App) InterruptInitialAlignment() (uiapi.InitialAlignmentStatus, error) {
	if err := a.requireAdmin(); err != nil {
		return uiapi.InitialAlignmentStatus{}, err
	}
	a.alignment.mu.Lock()
	if a.alignment.cancel == nil {
		a.alignment.mu.Unlock()
		status := a.GetInitialAlignmentStatus()
		if status.Stage == "unknown" {
			return status, errors.New("alignment_operation_not_owned_by_this_session: reconcile the durable outcome before retry")
		}
		return status, nil
	}
	defer a.alignment.mu.Unlock()
	a.alignment.status.Stage = "stopping"
	a.alignment.cancel()
	return a.alignment.status, nil
}

func (a *App) persistAlignmentStatusLocked(configPath string) error {
	data, err := json.Marshal(a.alignment.status)
	if err != nil {
		return err
	}
	return atomicfile.Write(configPath+".alignment-status.json", data, 0o600)
}
