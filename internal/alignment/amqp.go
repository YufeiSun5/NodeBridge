package alignment

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/rabbitmq/amqp091-go"
)

// The offline transport retains unacked input until the target transaction
// commits. Larger copies need durable staging, not an unbounded broker backlog.
const MaxSnapshotTransferBytes = 64 * 1024 * 1024

const attemptHeader = "nb_alignment_attempt"

type snapshotReply struct {
	PlanID   string      `json:"plan_id"`
	Sequence int64       `json:"sequence"`
	Result   *CopyResult `json:"result,omitempty"`
	Error    string      `json:"error,omitempty"`
}

func decodeSnapshotReply(b []byte) (snapshotReply, error) {
	var reply snapshotReply
	if len(b) > 1024 {
		return reply, errors.New("alignment_reply_too_large")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&reply); err != nil {
		return reply, err
	}
	if d.Decode(new(any)) != io.EOF {
		return reply, errors.New("alignment_reply_trailing_data")
	}
	if reply.Sequence < 0 || len(reply.PlanID) != 64 || reply.Error != "" && (reply.Error != "alignment_receive_failed" || reply.Result != nil) {
		return reply, errors.New("alignment_reply_invalid")
	}
	if _, err := hex.DecodeString(reply.PlanID); err != nil {
		return reply, errors.New("alignment_reply_invalid")
	}
	if reply.Result != nil {
		if err := validateFrame(SnapshotFrame{PlanID: reply.PlanID, Sequence: reply.Sequence, End: reply.Result}); err != nil {
			return reply, err
		}
	}
	return reply, nil
}

func snapshotQueues(plan Plan) (frames, replies string) {
	base := "nb.alignment." + plan.ID
	return base + ".frames", base + ".replies"
}

func declareSnapshotQueues(channel *amqp091.Channel, plan Plan) error {
	frames, replies := snapshotQueues(plan)
	for _, name := range []string{frames, replies} {
		if _, err := channel.QueueDeclare(name, true, false, false, false, nil); err != nil {
			return err
		}
	}
	return nil
}

// SendSnapshotAMQP uses only the source's DB and a dedicated pair of channels.
// Both endpoints must already own their maintenance leases and prepared jobs.
// Success means copy committed, never CDC cutover complete. No queue is purged.
func SendSnapshotAMQP(ctx context.Context, db *sql.DB, conn *amqp091.Connection, plan Plan, rule rules.SyncRule, nodeID string, confirm bool) (CopyResult, error) {
	return sendSnapshotAMQP(ctx, db, conn, plan, rule, nodeID, confirm, nil)
}

func SendCapturedSnapshotAMQP(ctx context.Context, db *sql.DB, conn *amqp091.Connection, plan Plan, rule rules.SyncRule, nodeID string, confirm bool, probe *SnapshotCapture) (CopyResult, error) {
	if err := probe.requireEndpoint(plan, nodeID); err != nil {
		return CopyResult{}, err
	}
	return sendSnapshotAMQP(ctx, db, conn, plan, rule, nodeID, confirm, probe)
}

func sendSnapshotAMQP(ctx context.Context, db *sql.DB, conn *amqp091.Connection, plan Plan, rule rules.SyncRule, nodeID string, confirm bool, probe *SnapshotCapture) (CopyResult, error) {
	if err := validateStreamPlan(plan, rule, confirm); err != nil {
		return CopyResult{}, err
	}
	if conn == nil || nodeID != plan.Source.NodeID {
		return CopyResult{}, errors.New("alignment_transport_endpoint_invalid")
	}
	job, err := ReadSnapshotJob(ctx, db, plan, nodeID)
	if err != nil {
		return CopyResult{}, err
	}
	if job.Phase != JobPrepared {
		return CopyResult{}, errors.New("alignment_job_not_prepared")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	publication, err := conn.Channel()
	if err != nil {
		return CopyResult{}, err
	}
	defer publication.Close()
	if err := declareSnapshotQueues(publication, plan); err != nil {
		return CopyResult{}, err
	}
	publisher, err := rabbitmq.NewPublisher(publication)
	if err != nil {
		return CopyResult{}, err
	}
	receipts, err := conn.Channel()
	if err != nil {
		return CopyResult{}, err
	}
	defer receipts.Close()
	if err := receipts.Qos(1, 0, false); err != nil {
		return CopyResult{}, err
	}
	framesQueue, repliesQueue := snapshotQueues(plan)
	deliveries, err := receipts.ConsumeWithContext(ctx, repliesQueue, "", false, true, false, false, nil)
	if err != nil {
		return CopyResult{}, err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return CopyResult{}, err
	}
	attempt := hex.EncodeToString(random[:])
	var transferred int
	return exportPreparedSnapshot(ctx, db, plan, rule, nodeID, confirm, func(ctx context.Context, frame SnapshotFrame) (CopyResult, error) {
		body, err := EncodeSnapshotFrame(frame)
		if err != nil {
			return CopyResult{}, err
		}
		if len(body) > MaxSnapshotTransferBytes-transferred {
			return CopyResult{}, errors.New("alignment_transfer_limit_exceeded")
		}
		transferred += len(body)
		if err := publisher.Publish(ctx, rabbitmq.PublishRequest{RoutingKey: framesQueue, Body: body, Headers: amqp091.Table{attemptHeader: attempt}}); err != nil {
			return CopyResult{}, err
		}
		select {
		case <-ctx.Done():
			return CopyResult{}, ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return CopyResult{}, errors.New("alignment_reply_channel_closed")
			}
			reply, err := decodeSnapshotReply(delivery.Body)
			if err != nil {
				return CopyResult{}, err
			}
			if delivery.Headers[attemptHeader] != attempt || reply.PlanID != plan.ID || reply.Sequence != frame.Sequence {
				return CopyResult{}, errors.New("alignment_reply_mismatch")
			}
			if reply.Error != "" {
				if err := delivery.Ack(false); err != nil {
					return CopyResult{}, err
				}
				return CopyResult{}, errors.New(reply.Error)
			}
			if (reply.Result != nil) != (frame.End != nil) {
				return CopyResult{}, errors.New("alignment_reply_phase_mismatch")
			}
			if err := delivery.Ack(false); err != nil {
				return CopyResult{}, err
			}
			if reply.Result != nil {
				return *reply.Result, nil
			}
			return CopyResult{}, nil
		}
	}, probe)
}

// ReceiveSnapshotAMQP accepts one prepared job, without acknowledging its input
// until copied rows and the durable receipt commit. Failure requeues outstanding
// input on channel close; a caller must inspect the job before any new attempt.
func ReceiveSnapshotAMQP(ctx context.Context, db *sql.DB, conn *amqp091.Connection, plan Plan, rule rules.SyncRule, nodeID string, confirm bool) (CopyResult, error) {
	return receiveSnapshotAMQP(ctx, db, conn, plan, rule, nodeID, confirm, nil)
}

func ReceiveCapturedSnapshotAMQP(ctx context.Context, db *sql.DB, conn *amqp091.Connection, plan Plan, rule rules.SyncRule, nodeID string, confirm bool, probe *SnapshotCapture) (CopyResult, error) {
	if err := probe.requireEndpoint(plan, nodeID); err != nil {
		return CopyResult{}, err
	}
	return receiveSnapshotAMQP(ctx, db, conn, plan, rule, nodeID, confirm, probe)
}

func receiveSnapshotAMQP(ctx context.Context, db *sql.DB, conn *amqp091.Connection, plan Plan, rule rules.SyncRule, nodeID string, confirm bool, probe *SnapshotCapture) (CopyResult, error) {
	if err := validateStreamPlan(plan, rule, confirm); err != nil {
		return CopyResult{}, err
	}
	if conn == nil || nodeID != plan.Target.NodeID {
		return CopyResult{}, errors.New("alignment_transport_endpoint_invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	r, err := NewPreparedSnapshotReceiver(ctx, db, plan, rule, nodeID, confirm)
	if err != nil {
		return CopyResult{}, err
	}
	defer r.Close()
	r.capture = probe
	publication, err := conn.Channel()
	if err != nil {
		return CopyResult{}, err
	}
	defer publication.Close()
	if err := declareSnapshotQueues(publication, plan); err != nil {
		return CopyResult{}, err
	}
	publisher, err := rabbitmq.NewPublisher(publication)
	if err != nil {
		return CopyResult{}, err
	}
	input, err := conn.Channel()
	if err != nil {
		return CopyResult{}, err
	}
	defer input.Close()
	// Per-row replies provide flow control without prematurely ACKing the input.
	if err := input.Qos(0, 0, false); err != nil {
		return CopyResult{}, err
	}
	framesQueue, repliesQueue := snapshotQueues(plan)
	deliveries, err := input.ConsumeWithContext(ctx, framesQueue, "", false, true, false, false, nil)
	if err != nil {
		return CopyResult{}, err
	}
	attempt, transferred := "", 0
	for {
		select {
		case <-ctx.Done():
			return CopyResult{}, ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return CopyResult{}, errors.New("alignment_input_channel_closed")
			}
			incoming, ok := delivery.Headers[attemptHeader].(string)
			if !ok || len(incoming) != 32 {
				return CopyResult{}, errors.New("alignment_attempt_invalid")
			}
			if _, err := hex.DecodeString(incoming); err != nil {
				return CopyResult{}, errors.New("alignment_attempt_invalid")
			}
			if attempt == "" {
				attempt = incoming
			}
			if attempt != incoming {
				return CopyResult{}, errors.New("alignment_attempt_changed")
			}
			if len(delivery.Body) > MaxSnapshotTransferBytes-transferred {
				return CopyResult{}, errors.New("alignment_transfer_limit_exceeded")
			}
			transferred += len(delivery.Body)
			frame, err := DecodeSnapshotFrame(delivery.Body)
			if err != nil {
				return CopyResult{}, err
			}
			result, receiveErr := r.Accept(ctx, frame)
			reply := snapshotReply{PlanID: plan.ID, Sequence: frame.Sequence}
			if receiveErr != nil {
				reply.Error = "alignment_receive_failed"
			} else if frame.End != nil {
				reply.Result = &result
			}
			body, err := json.Marshal(reply)
			if err != nil {
				return result, err
			}
			publishErr := publisher.Publish(ctx, rabbitmq.PublishRequest{RoutingKey: repliesQueue, Body: body, Headers: amqp091.Table{attemptHeader: attempt}})
			if receiveErr != nil {
				return CopyResult{}, errors.Join(receiveErr, publishErr)
			}
			if publishErr != nil {
				return result, fmt.Errorf("alignment_receipt_publish_failed: %w", publishErr)
			}
			if frame.End != nil {
				if err := delivery.Ack(true); err != nil {
					return result, fmt.Errorf("alignment_committed_input_ack_failed: %w", err)
				}
				return result, nil
			}
		}
	}
}
