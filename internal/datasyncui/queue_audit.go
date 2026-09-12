package datasyncui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/queueaudit"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/rabbitmq/amqp091-go"
)

type QueueEventPlanRequest struct {
	Queue   string `json:"queue"`
	RuleID  string `json:"rule_id"`
	EventID string `json:"event_id"`
}

type QueueEventApplyRequest struct {
	PlanID  string `json:"plan_id"`
	Confirm bool   `json:"confirm"`
}

type QueueEventAuditRequest struct {
	PlanID string `json:"plan_id"`
}

func (a *App) queueJournal() queueaudit.FileJournal {
	return queueaudit.FileJournal{Directory: filepath.Join(filepath.Dir(a.effectiveConfigPath()), "queue-audit")}
}

func (a *App) GetQueueEventAudit(req QueueEventAuditRequest) (QueueAuditReceiptDTO, error) {
	return queueReceiptDTO(a.queueJournal().Load(req.PlanID))
}

func (a *App) PlanQueueEventQuarantine(req QueueEventPlanRequest) (QueueAuditReceiptDTO, error) {
	if err := a.requireAdmin(); err != nil {
		return QueueAuditReceiptDTO{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return queueReceiptDTO(a.planQueueEventQuarantine(ctx, req))
}

func (a *App) ApplyQueueEventQuarantine(req QueueEventApplyRequest) (QueueAuditReceiptDTO, error) {
	if err := a.requireAdmin(); err != nil {
		return QueueAuditReceiptDTO{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return queueReceiptDTO(a.applyQueueEventQuarantine(ctx, req))
}

func (a *App) queueOperation() (func(), error) {
	if a.config == nil {
		return nil, errConfigMissing
	}
	// Hold the agent lease for the entire bounded operation; no local start races.
	lock, err := agentstate.Lock(a.effectiveConfigPath() + ".agent.lock")
	if err != nil {
		return nil, fmt.Errorf("queue operation requires a stopped agent: %w", err)
	}
	ruleLock, err := agentstate.Lock(a.effectiveRulesPath() + ".write.lock")
	if err != nil {
		lock.Close()
		return nil, err
	}
	return func() { ruleLock.Close(); lock.Close() }, nil
}

func (a *App) queueEndpoint(queue string) (string, error) {
	if a.config == nil {
		return "", errConfigMissing
	}
	switch a.config.Mode {
	case appconfig.ModeServer:
		if queue == "server.cdc.ingress.q" || queue == "server.dead.q" {
			return a.config.RabbitMQ.ServerURL, nil
		}
	case appconfig.ModeEdge:
		if queue == "edge.upload.cdc.q" || queue == "edge.downlink.q" || queue == "edge.dead.q" {
			return a.config.RabbitMQ.LocalURL, nil
		}
	}
	return "", errors.New("queue must be a local NodeBridge ingress, downlink, upload, or dead-letter queue")
}

func (a *App) queueConfigRevision() (string, error) {
	data, err := os.ReadFile(a.effectiveConfigPath())
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (a *App) planQueueEventQuarantine(ctx context.Context, req QueueEventPlanRequest) (queueaudit.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if req.EventID == "" || len(req.EventID) > 128 {
		return queueaudit.Receipt{}, errors.New("event_id must contain 1..128 bytes")
	}
	unlock, err := a.queueOperation()
	if err != nil {
		return queueaudit.Receipt{}, err
	}
	defer unlock()
	endpoint, err := a.queueEndpoint(req.Queue)
	if err != nil {
		return queueaudit.Receipt{}, err
	}
	set, revision, err := rules.LoadFileWithRevision(a.effectiveRulesPath())
	if err != nil {
		return queueaudit.Receipt{}, err
	}
	if _, err := findRuleByID(set, req.RuleID); err != nil {
		return queueaudit.Receipt{}, err
	}
	configRevision, err := a.queueConfigRevision()
	if err != nil {
		return queueaudit.Receipt{}, err
	}
	conn, err := rabbitmq.Dial(endpoint)
	if err != nil {
		return queueaudit.Receipt{}, err
	}
	defer conn.Close()
	if err := queueHasNoConsumers(conn.Channel, req.Queue); err != nil {
		return queueaudit.Receipt{}, err
	}
	var receipt queueaudit.Receipt
	err = queueaudit.WithEvent(ctx, conn.Channel, req.Queue, req.EventID, func(delivery amqp091.Delivery) (bool, error) {
		var evt event.SyncEvent
		if err := json.Unmarshal(delivery.Body, &evt); err != nil {
			return false, err
		}
		matched := set.FindForNode(evt.DatabaseName, evt.TableName, evt.OriginNodeID, evt.SourceNodeID)
		if matched == nil || matched.ID != req.RuleID {
			return false, errors.New("event does not match the selected rule")
		}
		fingerprint, err := queueaudit.Fingerprint(delivery)
		if err != nil {
			return false, err
		}
		now := time.Now().UTC()
		plan, err := queueaudit.Seal(queueaudit.Plan{NodeID: a.config.Node.ID, SourceQueue: req.Queue, TargetQueue: req.Queue + ".quarantine", RuleID: req.RuleID, RuleRevision: revision, ConfigRevision: configRevision, EventID: req.EventID, Fingerprint: fingerprint, ExpiresAt: now.Add(10 * time.Minute)})
		if err != nil {
			return false, err
		}
		receipt = queueaudit.Receipt{Plan: plan, State: "planned", UpdatedAt: now}
		return false, a.queueJournal().Save(receipt)
	})
	return receipt, err
}

func (a *App) applyQueueEventQuarantine(ctx context.Context, req QueueEventApplyRequest) (queueaudit.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if !req.Confirm {
		return queueaudit.Receipt{}, errors.New("explicit confirm is required for this quarantine plan")
	}
	unlock, err := a.queueOperation()
	if err != nil {
		return queueaudit.Receipt{}, err
	}
	defer unlock()
	receipt, err := a.queueJournal().Load(req.PlanID)
	if err != nil {
		return receipt, err
	}
	plan := receipt.Plan
	configRevision, err := a.queueConfigRevision()
	if err != nil {
		return receipt, err
	}
	if plan.NodeID != a.config.Node.ID || plan.TargetQueue != plan.SourceQueue+".quarantine" || plan.ConfigRevision != configRevision {
		return receipt, errors.New("queue_plan_stale")
	}
	endpoint, err := a.queueEndpoint(plan.SourceQueue)
	if err != nil {
		return receipt, err
	}
	_, revision, err := rules.LoadFileWithRevision(a.effectiveRulesPath())
	if err != nil {
		return receipt, err
	}
	conn, err := rabbitmq.Dial(endpoint)
	if err != nil {
		return receipt, err
	}
	defer conn.Close()
	if err := queueHasNoConsumers(conn.Channel, plan.SourceQueue); err != nil {
		return receipt, err
	}
	return queueaudit.Transfer(ctx, conn.Channel, a.queueJournal(), func(ctx context.Context, queue string, delivery amqp091.Delivery, id string) error {
		return queueaudit.PublishCopy(ctx, conn.Conn, queue, delivery, id)
	}, plan, revision, time.Now().UTC())
}

func queueHasNoConsumers(channel *amqp091.Channel, queue string) error {
	status, err := channel.QueueDeclarePassive(queue, true, false, false, false, nil)
	if err != nil {
		return err
	}
	if status.Consumers != 0 {
		return errors.New("queue has active consumers; stop consumers before planning or applying quarantine")
	}
	return nil
}
