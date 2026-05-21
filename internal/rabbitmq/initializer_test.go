package rabbitmq_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

func TestInitializeTopologyOrder(t *testing.T) {
	declarer := &recordingDeclarer{}
	topology := rabbitmq.Topology{
		Exchanges: []rabbitmq.Exchange{{Name: "events.x", Kind: rabbitmq.ExchangeDirect, Durable: true}},
		Queues:    []rabbitmq.Queue{{Name: "events.q", Durable: true}},
		Bindings:  []rabbitmq.Binding{{Queue: "events.q", RoutingKey: "events", Exchange: "events.x"}},
	}

	if err := rabbitmq.InitializeTopology(declarer, topology); err != nil {
		t.Fatalf("InitializeTopology returned error: %v", err)
	}

	want := []string{"exchange:events.x", "queue:events.q", "bind:events.q:events:events.x"}
	if !reflect.DeepEqual(declarer.calls, want) {
		t.Fatalf("unexpected calls\nwant=%v\ngot=%v", want, declarer.calls)
	}
}

func TestInitializeTopologyExchangeError(t *testing.T) {
	declarer := &recordingDeclarer{exchangeErr: errors.New("boom")}

	err := rabbitmq.InitializeTopology(declarer, rabbitmq.Topology{
		Exchanges: []rabbitmq.Exchange{{Name: "events.x", Kind: rabbitmq.ExchangeDirect, Durable: true}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

type recordingDeclarer struct {
	calls       []string
	exchangeErr error
}

func (d *recordingDeclarer) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp091.Table) error {
	d.calls = append(d.calls, "exchange:"+name)
	return d.exchangeErr
}

func (d *recordingDeclarer) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) (amqp091.Queue, error) {
	d.calls = append(d.calls, "queue:"+name)
	return amqp091.Queue{Name: name}, nil
}

func (d *recordingDeclarer) QueueBind(name, key, exchange string, noWait bool, args amqp091.Table) error {
	d.calls = append(d.calls, "bind:"+name+":"+key+":"+exchange)
	return nil
}
