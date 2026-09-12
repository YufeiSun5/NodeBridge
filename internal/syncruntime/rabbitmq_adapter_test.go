package syncruntime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/event"
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

func TestRoutingDownlinkDispatcherBatchKeepsRoutingAndPayload(t *testing.T) {
	for _, useBatch := range []bool{false, true} {
		publisher := &fakePublisher{}
		batchPublisher := &fakeBatchPublisher{}
		dispatcher := RoutingDownlinkDispatcher{Publisher: publisher, Exchange: "server.dispatch.x"}
		if useBatch {
			dispatcher.Publisher = batchPublisher
		}
		first, second := sampleServerEvent(), sampleServerEvent()
		second.EventID = "evt-second"
		requests := []DownlinkRequest{{Event: first, TargetNodeID: "edge-a"}, {Event: first, TargetNodeID: "edge-b"}, {Event: second, TargetNodeID: "edge-a"}}
		if err := dispatcher.DispatchBatch(context.Background(), requests); err != nil {
			t.Fatal(err)
		}
		published := publisher.requests
		if useBatch {
			if len(batchPublisher.batches) != 1 || len(batchPublisher.requests) != 0 {
				t.Fatalf("expected only one batch: %+v", batchPublisher)
			}
			published = batchPublisher.batches[0]
		}
		if len(published) != len(requests) {
			t.Fatalf("got %d messages", len(published))
		}
		for i, req := range published {
			var evt event.SyncEvent
			if err := json.Unmarshal(req.Body, &evt); err != nil || evt.EventID != requests[i].Event.EventID || evt.DatabaseName != "scada_center" {
				t.Fatalf("invalid source event: %+v %v", evt, err)
			}
			if req.Exchange != dispatcher.Exchange || req.RoutingKey != requests[i].TargetNodeID+".downlink" {
				t.Fatalf("wrong routing: %+v", req)
			}
		}
	}
}

func TestRoutingDownlinkDispatcherEncodesWholeBatchBeforePublishing(t *testing.T) {
	publisher := &fakeBatchPublisher{}
	dispatcher := RoutingDownlinkDispatcher{Publisher: publisher}
	bad := sampleServerEvent()
	bad.After["bad"] = func() {}
	if err := dispatcher.DispatchBatch(context.Background(), []DownlinkRequest{{Event: sampleServerEvent()}, {Event: bad}}); err == nil {
		t.Fatal("expected encoding failure")
	}
	if len(publisher.batches) != 0 || len(publisher.requests) != 0 {
		t.Fatal("encoding failure partially published batch")
	}
	if err := dispatcher.DispatchBatch(context.Background(), nil); err != nil || len(publisher.batches) != 0 {
		t.Fatalf("empty batch published: %v", err)
	}
}
