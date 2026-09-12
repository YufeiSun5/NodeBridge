package queueaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

const MaxScan = 100
const MaxScanBytes = 2 << 20

type Plan struct {
	ID             string    `json:"plan_id"`
	NodeID         string    `json:"node_id"`
	SourceQueue    string    `json:"source_queue"`
	TargetQueue    string    `json:"target_queue"`
	RuleID         string    `json:"rule_id"`
	RuleRevision   string    `json:"rule_revision"`
	ConfigRevision string    `json:"config_revision"`
	EventID        string    `json:"event_id"`
	Fingerprint    string    `json:"fingerprint"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type Receipt struct {
	Plan      Plan      `json:"plan"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Getter interface {
	Get(string, bool) (amqp091.Delivery, bool, error)
}

type Journal interface {
	Load(string) (Receipt, error)
	Save(Receipt) error
}

type PublishConfirmed func(context.Context, string, amqp091.Delivery, string) error

func Fingerprint(delivery amqp091.Delivery) (string, error) {
	encoded, err := json.Marshal(struct {
		Body                                                                []byte
		ContentType, ContentEncoding, MessageID, CorrelationID, Type, AppID string
		Headers                                                             amqp091.Table
	}{delivery.Body, delivery.ContentType, delivery.ContentEncoding, delivery.MessageId, delivery.CorrelationId, delivery.Type, delivery.AppId, delivery.Headers})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func Seal(plan Plan) (Plan, error) {
	plan.ID = ""
	if plan.NodeID == "" || plan.SourceQueue == "" || plan.TargetQueue == "" || plan.SourceQueue == plan.TargetQueue || plan.RuleID == "" || plan.RuleRevision == "" || plan.EventID == "" || len(plan.Fingerprint) != 64 || plan.ExpiresAt.IsZero() {
		return Plan{}, errors.New("invalid queue transfer plan")
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		return Plan{}, err
	}
	sum := sha256.Sum256(encoded)
	plan.ID = hex.EncodeToString(sum[:])
	return plan, nil
}

// WithEvent holds a bounded batch to avoid repeatedly inspecting one requeued head.
// Callers must use a dedicated channel and close it even when requeue fails.
func WithEvent(ctx context.Context, getter Getter, queue, eventID string, visit func(amqp091.Delivery) (bool, error)) (err error) {
	held := []amqp091.Delivery{}
	acked := map[uint64]bool{}
	defer func() {
		for _, delivery := range held {
			if !acked[delivery.DeliveryTag] {
				if nackErr := delivery.Nack(false, true); nackErr != nil {
					err = errors.Join(err, fmt.Errorf("return inspected message: %w", nackErr))
				}
			}
		}
	}()
	bytes := 0
	for i := 0; i < MaxScan; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		delivery, ok, err := getter.Get(queue, false)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		held = append(held, delivery)
		bytes += len(delivery.Body)
		if bytes > MaxScanBytes {
			return errors.New("queue_scan_byte_limit: no message removed")
		}
		var identity struct {
			EventID string `json:"event_id"`
		}
		if json.Unmarshal(delivery.Body, &identity) != nil || identity.EventID != eventID {
			continue
		}
		removed, err := visit(delivery)
		acked[delivery.DeliveryTag] = removed
		return err
	}
	return errors.New("event_not_in_bounded_queue_scan: no message removed")
}

// Transfer preserves the original until a confirmed copy and its audit are durable.
// A lost publisher confirmation may create duplicate copies with the same plan ID.
func Transfer(ctx context.Context, getter Getter, journal Journal, publish PublishConfirmed, plan Plan, revision string, now time.Time) (Receipt, error) {
	sealed, err := Seal(plan)
	if err != nil || sealed.ID != plan.ID || plan.RuleRevision != revision {
		return Receipt{}, errors.New("queue_plan_stale")
	}
	receipt, err := journal.Load(plan.ID)
	if err != nil {
		return Receipt{}, err
	}
	if receipt.Plan != plan {
		return Receipt{}, errors.New("queue_plan_mismatch")
	}
	if receipt.State == "acked" {
		return receipt, nil
	}
	if !now.Before(plan.ExpiresAt) {
		return receipt, errors.New("queue_plan_expired")
	}
	if receipt.State != "planned" && receipt.State != "publishing" && receipt.State != "confirmed" {
		return receipt, errors.New("invalid_queue_audit_state")
	}
	err = WithEvent(ctx, getter, plan.SourceQueue, plan.EventID, func(delivery amqp091.Delivery) (bool, error) {
		fingerprint, err := Fingerprint(delivery)
		if err != nil || fingerprint != plan.Fingerprint {
			return false, errors.New("queue_message_changed")
		}
		if receipt.State != "confirmed" {
			receipt.State, receipt.UpdatedAt = "publishing", now
			if err := journal.Save(receipt); err != nil {
				return false, err
			}
			if err := publish(ctx, plan.TargetQueue, delivery, plan.ID); err != nil {
				return false, err
			}
			receipt.State = "confirmed"
			if err := journal.Save(receipt); err != nil {
				return false, err
			}
		}
		if err := delivery.Ack(false); err != nil {
			return false, err
		}
		receipt.State = "acked"
		return true, journal.Save(receipt)
	})
	return receipt, err
}
