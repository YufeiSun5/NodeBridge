package syncruntime

import (
	"context"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

func TestDisabledRuleRealBrokerRetainsEvent(t *testing.T) {
	raw := os.Getenv("NODEBRIDGE_REMEDIATION_RABBITMQ_URL")
	if raw == "" {
		t.Skip("NODEBRIDGE_REMEDIATION_RABBITMQ_URL required")
	}
	u, err := url.Parse(raw)
	if err != nil || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
		t.Fatal("isolated loopback broker required")
	}
	conn, err := amqp091.Dial(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()
	queue, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	publisher, err := rabbitmq.NewPublisher(ch)
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.Publish(ctx, rabbitmq.PublishRequest{RoutingKey: queue.Name, Body: mustJSON(t, sampleEvent())}); err != nil {
		t.Fatal(err)
	}
	ruleSet := sampleRules()
	ruleSet.Rules[0].Enable = false
	worker := &fakeWorker{}
	runtime := EdgeDownlinkRuntime{Source: ownedQueueSource{ch, queue.Name}, Consumer: rabbitmq.Consumer{RequeueOnError: true}, Rules: ruleSet, Worker: worker}
	for i := 0; i < 2; i++ {
		result, err := runtime.RunOnce(ctx)
		if err == nil || !strings.Contains(err.Error(), "rule_not_active") || result.EventID != "evt-001" || len(worker.events) != 0 {
			t.Fatalf("attempt=%d result=%+v err=%v writes=%d", i, result, err, len(worker.events))
		}
	}
	ruleSet.Rules[0].Enable = true
	result, err := runtime.RunOnce(ctx)
	if err != nil || result.Action != "applied" || len(worker.events) != 1 {
		t.Fatalf("resume result=%+v err=%v writes=%d", result, err, len(worker.events))
	}
	if _, ok, err := ch.Get(queue.Name, false); err != nil || ok {
		t.Fatalf("ACK did not drain owned event: %t %v", ok, err)
	}
}

type ownedQueueSource struct {
	channel *amqp091.Channel
	queue   string
}

func (s ownedQueueSource) Get(ctx context.Context) (rabbitmq.IncomingMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	delivery, ok, err := s.channel.Get(s.queue, false)
	return rabbitmq.DeliveryMessage{Delivery: delivery}, ok, err
}
