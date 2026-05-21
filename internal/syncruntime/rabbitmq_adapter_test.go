package syncruntime

import (
	"context"
	"testing"
)

func TestRoutingDownlinkDispatcherUsesServerTopologyRoutingKey(t *testing.T) {
	publisher := &fakePublisher{}
	dispatcher := RoutingDownlinkDispatcher{
		Publisher: publisher,
		Exchange:  "server.dispatch.x",
	}

	if err := dispatcher.Dispatch(context.Background(), sampleEvent(), "edge-b"); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}

	if len(publisher.requests) != 1 {
		t.Fatalf("expected one publish, got %d", len(publisher.requests))
	}
	req := publisher.requests[0]
	if req.Exchange != "server.dispatch.x" {
		t.Fatalf("unexpected exchange %q", req.Exchange)
	}
	if req.RoutingKey != "edge-b.downlink" {
		t.Fatalf("unexpected routing key %q", req.RoutingKey)
	}
	if len(req.Body) == 0 || !rabbitmqLooksLikeJSON(req.Body) {
		t.Fatalf("expected JSON body, got %q", string(req.Body))
	}
}

func rabbitmqLooksLikeJSON(body []byte) bool {
	return len(body) > 1 && body[0] == '{' && body[len(body)-1] == '}'
}
