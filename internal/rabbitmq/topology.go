package rabbitmq

import "github.com/rabbitmq/amqp091-go"

const (
	EdgeVHost   = "/edge-sync"
	ServerVHost = "/server-sync"

	ExchangeDirect = "direct"
	ExchangeTopic  = "topic"
)

type Exchange struct {
	Name       string
	Kind       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp091.Table
}

type Queue struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       amqp091.Table
}

type Binding struct {
	Queue      string
	RoutingKey string
	Exchange   string
	NoWait     bool
	Args       amqp091.Table
}

type Topology struct {
	VHost     string
	Exchanges []Exchange
	Queues    []Queue
	Bindings  []Binding
}

func EdgeTopology() Topology {
	return Topology{
		VHost: EdgeVHost,
		Exchanges: []Exchange{
			durableExchange("edge.upload.x", ExchangeDirect),
			durableExchange("edge.downlink.x", ExchangeDirect),
			durableExchange("edge.dead.x", ExchangeDirect),
		},
		Queues: []Queue{
			durableQueue("edge.upload.cdc.q", "edge.dead.x", "edge.dead"),
			durableQueue("edge.upload.retry.q", "edge.dead.x", "edge.dead"),
			durableQueue("edge.downlink.q", "edge.dead.x", "edge.dead"),
			durableQueue("edge.dead.q", "", ""),
		},
		Bindings: []Binding{
			bind("edge.upload.cdc.q", "edge.upload.cdc", "edge.upload.x"),
			bind("edge.upload.retry.q", "edge.upload.retry", "edge.upload.x"),
			bind("edge.downlink.q", "edge.downlink", "edge.downlink.x"),
			bind("edge.dead.q", "edge.dead", "edge.dead.x"),
		},
	}
}

func ServerTopology(edgeNodeIDs []string) Topology {
	topology := Topology{
		VHost: ServerVHost,
		Exchanges: []Exchange{
			durableExchange("server.ingress.x", ExchangeDirect),
			durableExchange("server.dispatch.x", ExchangeDirect),
			durableExchange("server.dead.x", ExchangeDirect),
		},
		Queues: []Queue{
			durableQueue("server.cdc.ingress.q", "server.dead.x", "server.dead"),
			durableQueue("server.dead.q", "", ""),
		},
		Bindings: []Binding{
			bind("server.cdc.ingress.q", "server.ingress", "server.ingress.x"),
			bind("server.dead.q", "server.dead", "server.dead.x"),
		},
	}

	for _, nodeID := range edgeNodeIDs {
		queueName := nodeID + ".downlink.q"
		topology.Queues = append(topology.Queues, durableQueue(queueName, "server.dead.x", "server.dead"))
		topology.Bindings = append(topology.Bindings, bind(queueName, nodeID+".downlink", "server.dispatch.x"))
	}

	return topology
}

func durableExchange(name, kind string) Exchange {
	return Exchange{Name: name, Kind: kind, Durable: true}
}

func durableQueue(name, deadExchange, deadRoutingKey string) Queue {
	args := amqp091.Table{}
	if deadExchange != "" {
		args["x-dead-letter-exchange"] = deadExchange
		args["x-dead-letter-routing-key"] = deadRoutingKey
	}
	return Queue{Name: name, Durable: true, Args: args}
}

func bind(queue, routingKey, exchange string) Binding {
	return Binding{Queue: queue, RoutingKey: routingKey, Exchange: exchange}
}
