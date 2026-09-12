package datasyncui

import (
	"github.com/YufeiSun5/NodeBridge/internal/queueaudit"
	"time"
)

type QueueAuditPlanDTO struct {
	ID             string `json:"plan_id"`
	NodeID         string `json:"node_id"`
	SourceQueue    string `json:"source_queue"`
	TargetQueue    string `json:"target_queue"`
	RuleID         string `json:"rule_id"`
	RuleRevision   string `json:"rule_revision"`
	ConfigRevision string `json:"config_revision"`
	EventID        string `json:"event_id"`
	Fingerprint    string `json:"fingerprint"`
	ExpiresAt      string `json:"expires_at"`
}

type QueueAuditReceiptDTO struct {
	Plan      QueueAuditPlanDTO `json:"plan"`
	State     string            `json:"state"`
	UpdatedAt string            `json:"updated_at"`
}

func queueReceiptDTO(r queueaudit.Receipt, err error) (QueueAuditReceiptDTO, error) {
	p := r.Plan
	return QueueAuditReceiptDTO{Plan: QueueAuditPlanDTO{ID: p.ID, NodeID: p.NodeID, SourceQueue: p.SourceQueue, TargetQueue: p.TargetQueue, RuleID: p.RuleID, RuleRevision: p.RuleRevision, ConfigRevision: p.ConfigRevision, EventID: p.EventID, Fingerprint: p.Fingerprint, ExpiresAt: p.ExpiresAt.Format(time.RFC3339Nano)}, State: r.State, UpdatedAt: r.UpdatedAt.Format(time.RFC3339Nano)}, err
}
