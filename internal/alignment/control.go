package alignment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/rabbitmq/amqp091-go"
)

const maxControlBytes = 4 << 20

type controlMessage struct {
	Version   int             `json:"version"`
	Stage     string          `json:"stage"`
	From      string          `json:"from"`
	To        string          `json:"to"`
	RuleHash  string          `json:"rule_hash"`
	Nonce     string          `json:"nonce"`
	PeerNonce string          `json:"peer_nonce,omitempty"`
	Body      json.RawMessage `json:"body,omitempty"`
}

// PairControl lives only for one explicitly armed pair operation. Session queues
// are exclusive; no permanent RPC service or business queue is created or read.
type PairControl struct {
	input     *amqp091.Channel
	output    *amqp091.Channel
	publisher *rabbitmq.Publisher
	node      string
	peer      string
	ruleHash  string
	nonce     string
	peerNonce string
	pending   map[string]json.RawMessage
	completed map[string]bool
}

func sessionQueue(nonce string) string { return "nb.alignment.session." + nonce }

func validNonce(nonce string) bool {
	if len(nonce) != 32 {
		return false
	}
	_, err := hex.DecodeString(nonce)
	return err == nil
}

func OpenPairControl(ctx context.Context, conn *amqp091.Connection, node, peer string, rule rules.SyncRule, initiator bool) (*PairControl, error) {
	if conn == nil || node == "" || peer == "" || node == peer || rule.Enable {
		return nil, errors.New("alignment_control_identity_invalid")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	c := &PairControl{node: node, peer: peer, nonce: hex.EncodeToString(random[:]), ruleHash: hash(canonicalCutoverRule(rule)), pending: make(map[string]json.RawMessage), completed: make(map[string]bool)}
	var err error
	c.input, err = conn.Channel()
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = c.Close()
		}
	}()
	c.output, err = conn.Channel()
	if err != nil {
		return nil, err
	}
	c.publisher, err = rabbitmq.NewPublisher(c.output)
	if err != nil {
		return nil, err
	}
	if _, err := c.input.QueueDeclare(sessionQueue(c.nonce), false, true, true, false, nil); err != nil {
		return nil, err
	}
	edge, server := node, peer
	if !initiator {
		edge, server = peer, node
	}
	offers := "nb.alignment.offer." + hash([]string{edge, server, rule.ID})
	if _, err := c.input.QueueDeclare(offers, false, false, false, false, amqp091.Table{"x-message-ttl": int32(120000), "x-expires": int32(600000)}); err != nil {
		return nil, err
	}
	if initiator {
		if err := c.send(ctx, offers, "offer", nil); err != nil {
			return nil, err
		}
		for {
			message, ok, err := c.poll(sessionQueue(c.nonce))
			if err != nil {
				return nil, err
			}
			if ok && message.Stage == "challenge" && message.PeerNonce == c.nonce {
				c.peerNonce = message.Nonce
				if err := c.send(ctx, sessionQueue(c.peerNonce), "confirm", nil); err != nil {
					return nil, err
				}
				break
			}
			if err := controlPause(ctx); err != nil {
				return nil, err
			}
		}
	} else {
		candidates := make(map[string]bool)
		for {
			message, found, err := c.poll(offers)
			if err != nil {
				return nil, err
			}
			if found && message.Stage == "offer" {
				candidates[message.Nonce] = true
				if len(candidates) > 64 {
					return nil, errors.New("alignment_control_offer_limit")
				}
				c.peerNonce = message.Nonce
				if err := c.send(ctx, sessionQueue(c.peerNonce), "challenge", nil); err != nil {
					return nil, err
				}
			}
			message, found, err = c.poll(sessionQueue(c.nonce))
			if err != nil {
				return nil, err
			}
			if found && message.Stage == "confirm" && message.PeerNonce == c.nonce && candidates[message.Nonce] {
				c.peerNonce = message.Nonce
				break
			}
			if err := controlPause(ctx); err != nil {
				return nil, err
			}
		}
	}
	ok = true
	return c, nil
}

func controlPause(ctx context.Context) error {
	timer := time.NewTimer(20 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *PairControl) send(ctx context.Context, queue, stage string, body any) error {
	var encoded json.RawMessage
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	message := controlMessage{Version: 1, Stage: stage, From: c.node, To: c.peer, RuleHash: c.ruleHash, Nonce: c.nonce, PeerNonce: c.peerNonce, Body: encoded}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(data) > maxControlBytes {
		return errors.New("alignment_control_message_too_large")
	}
	return c.publisher.Publish(ctx, rabbitmq.PublishRequest{RoutingKey: queue, Body: data})
}

func (c *PairControl) poll(queue string) (controlMessage, bool, error) {
	var message controlMessage
	delivery, ok, err := c.input.Get(queue, false)
	if err != nil || !ok {
		return message, ok, err
	}
	if len(delivery.Body) > maxControlBytes {
		_ = delivery.Nack(false, true)
		return message, false, errors.New("alignment_control_message_too_large")
	}
	if err := json.Unmarshal(delivery.Body, &message); err != nil {
		_ = delivery.Nack(false, true)
		return message, false, err
	}
	if message.Version != 1 || message.From != c.peer || message.To != c.node || message.RuleHash != c.ruleHash || !validNonce(message.Nonce) {
		_ = delivery.Nack(false, true)
		return message, false, errors.New("alignment_control_peer_or_rule_changed")
	}
	// Only protocol messages on this pair's dedicated control queues are ACKed.
	if err := delivery.Ack(false); err != nil {
		return message, false, err
	}
	return message, true, nil
}

func (c *PairControl) Exchange(ctx context.Context, stage string, local, peer any) error {
	if c.completed[stage] {
		return errors.New("alignment_control_stage_repeated")
	}
	if err := c.send(ctx, sessionQueue(c.peerNonce), stage, local); err != nil {
		return err
	}
	for {
		if body, ok := c.pending[stage]; ok {
			delete(c.pending, stage)
			c.completed[stage] = true
			return json.Unmarshal(body, peer)
		}
		message, ok, err := c.poll(sessionQueue(c.nonce))
		if err != nil {
			return err
		}
		if ok {
			if message.Nonce != c.peerNonce || message.PeerNonce != c.nonce {
				return errors.New("alignment_control_session_changed")
			}
			if message.Stage == "error" {
				return fmt.Errorf("alignment_peer_failed: %s", string(message.Body))
			}
			if message.Stage != "confirm" && message.Stage != "challenge" && !c.completed[message.Stage] {
				if prior, exists := c.pending[message.Stage]; exists && string(prior) != string(message.Body) {
					return errors.New("alignment_control_conflicting_stage")
				}
				c.pending[message.Stage] = message.Body
				if len(c.pending) > 16 {
					return errors.New("alignment_control_stage_limit")
				}
			}
		}
		if err := controlPause(ctx); err != nil {
			return err
		}
	}
}

func (c *PairControl) Close() error {
	var err error
	if c.input != nil {
		err = c.input.Close()
	}
	if c.output != nil {
		err = errors.Join(err, c.output.Close())
	}
	return err
}
