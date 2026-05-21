package rabbitmq_test

import (
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
)

func TestEdgeTopology(t *testing.T) {
	topology := rabbitmq.EdgeTopology()

	if topology.VHost != rabbitmq.EdgeVHost {
		t.Fatalf("unexpected vhost %q", topology.VHost)
	}
	requireExchange(t, topology, "edge.upload.x")
	requireQueue(t, topology, "edge.upload.cdc.q")
	requireQueue(t, topology, "edge.downlink.q")
	requireBinding(t, topology, "edge.upload.cdc.q", "edge.upload.cdc", "edge.upload.x")
	requireDeadLetter(t, topology, "edge.upload.cdc.q", "edge.dead.x", "edge.dead")
}

func TestServerTopology(t *testing.T) {
	topology := rabbitmq.ServerTopology([]string{"edge-001", "edge-002"})

	if topology.VHost != rabbitmq.ServerVHost {
		t.Fatalf("unexpected vhost %q", topology.VHost)
	}
	requireExchange(t, topology, "server.ingress.x")
	requireQueue(t, topology, "server.cdc.ingress.q")
	requireQueue(t, topology, "edge-001.downlink.q")
	requireQueue(t, topology, "edge-002.downlink.q")
	requireBinding(t, topology, "edge-001.downlink.q", "edge-001.downlink", "server.dispatch.x")
	requireDeadLetter(t, topology, "edge-001.downlink.q", "server.dead.x", "server.dead")
}

func requireExchange(t *testing.T, topology rabbitmq.Topology, name string) {
	t.Helper()
	for _, exchange := range topology.Exchanges {
		if exchange.Name == name {
			if !exchange.Durable {
				t.Fatalf("exchange %s must be durable", name)
			}
			return
		}
	}
	t.Fatalf("missing exchange %s", name)
}

func requireQueue(t *testing.T, topology rabbitmq.Topology, name string) {
	t.Helper()
	for _, queue := range topology.Queues {
		if queue.Name == name {
			if !queue.Durable {
				t.Fatalf("queue %s must be durable", name)
			}
			return
		}
	}
	t.Fatalf("missing queue %s", name)
}

func requireBinding(t *testing.T, topology rabbitmq.Topology, queue, key, exchange string) {
	t.Helper()
	for _, binding := range topology.Bindings {
		if binding.Queue == queue && binding.RoutingKey == key && binding.Exchange == exchange {
			return
		}
	}
	t.Fatalf("missing binding queue=%s key=%s exchange=%s", queue, key, exchange)
}

func requireDeadLetter(t *testing.T, topology rabbitmq.Topology, queueName, exchange, key string) {
	t.Helper()
	for _, queue := range topology.Queues {
		if queue.Name == queueName {
			if queue.Args["x-dead-letter-exchange"] != exchange {
				t.Fatalf("unexpected dlx for %s: %+v", queueName, queue.Args)
			}
			if queue.Args["x-dead-letter-routing-key"] != key {
				t.Fatalf("unexpected dl routing key for %s: %+v", queueName, queue.Args)
			}
			return
		}
	}
	t.Fatalf("missing queue %s", queueName)
}
