package canal

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{ReaderName: "edge-001", Address: "127.0.0.1:11111", Destination: "edge-001", BatchSize: 100}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	for name, cfg := range map[string]Config{
		"reader":      {Address: "127.0.0.1:11111", Destination: "edge-001"},
		"address":     {ReaderName: "edge-001", Destination: "edge-001"},
		"destination": {ReaderName: "edge-001", Address: "127.0.0.1:11111"},
		"batch":       {ReaderName: "edge-001", Address: "127.0.0.1:11111", Destination: "edge-001", BatchSize: -1},
	} {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected %s validation error", name)
		}
	}
}

func TestConvertRowChange(t *testing.T) {
	change, err := ConvertRowChange(RowChange{
		DatabaseName: "scada_edge",
		TableName:    "device_config",
		Operation:    cdc.OperationUpdate,
		PrimaryKey:   map[string]any{"id": 1},
		After:        map[string]any{"id": 1},
		BinlogFile:   "mysql-bin.000001",
		BinlogPos:    42,
	})
	if err != nil {
		t.Fatalf("ConvertRowChange returned error: %v", err)
	}
	if change.DatabaseName != "scada_edge" || change.TableName != "device_config" || change.BinlogPos != 42 {
		t.Fatalf("unexpected change %+v", change)
	}
	change.After["id"] = 2
	if change.After["id"] == 1 {
		t.Fatal("expected cloned map to be mutable without changing source assertion")
	}

	if _, err := ConvertRowChange(RowChange{Operation: "DDL"}); err == nil {
		t.Fatal("expected invalid row error")
	}
}

func TestAdapterFetchOncePublishesEventsAndSavesOffset(t *testing.T) {
	store := cdc.NewMemoryOffsetStore()
	client := &fakeCanalClient{
		rows: []RowChange{
			{
				DatabaseName: "scada_edge",
				TableName:    "device_config",
				Operation:    cdc.OperationUpdate,
				PrimaryKey:   map[string]any{"id": 1},
				After:        map[string]any{"id": 1},
				BinlogFile:   "mysql-bin.000001",
				BinlogPos:    42,
			},
		},
		offset: cdc.Offset{BinlogFile: "mysql-bin.000001", BinlogPos: 43},
	}
	adapter, err := NewAdapter(Config{ReaderName: "edge-001", Address: "127.0.0.1:11111", Destination: "edge-001", BatchSize: 10}, client, store)
	if err != nil {
		t.Fatalf("NewAdapter returned error: %v", err)
	}

	count, err := adapter.FetchOnce(context.Background())
	if err != nil {
		t.Fatalf("FetchOnce returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one event, got %d", count)
	}
	select {
	case evt := <-adapter.Events():
		if evt.TableName != "device_config" {
			t.Fatalf("unexpected event %+v", evt)
		}
	default:
		t.Fatal("expected event")
	}
	offset, ok, err := store.Load(context.Background(), "edge-001")
	if err != nil || !ok {
		t.Fatalf("expected saved offset ok=%t err=%v", ok, err)
	}
	if offset.BinlogPos != 43 || !client.acked {
		t.Fatalf("unexpected offset=%+v acked=%t", offset, client.acked)
	}
}

func TestAdapterLifecycleDelegatesClient(t *testing.T) {
	client := &fakeCanalClient{}
	adapter, err := NewAdapter(Config{ReaderName: "edge-001", Address: "127.0.0.1:11111", Destination: "edge-001"}, client, nil)
	if err != nil {
		t.Fatalf("NewAdapter returned error: %v", err)
	}

	if err := adapter.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !client.connected || client.destination != "edge-001" {
		t.Fatalf("unexpected client state %+v", client)
	}
	if err := adapter.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if !client.closed {
		t.Fatal("expected client close")
	}
}

func TestAdapterPropagatesFetchError(t *testing.T) {
	adapter, err := NewAdapter(Config{ReaderName: "edge-001", Address: "127.0.0.1:11111", Destination: "edge-001"}, &fakeCanalClient{err: errors.New("canal down")}, nil)
	if err != nil {
		t.Fatalf("NewAdapter returned error: %v", err)
	}
	if _, err := adapter.FetchOnce(context.Background()); err == nil {
		t.Fatal("expected fetch error")
	}
}

type fakeCanalClient struct {
	rows        []RowChange
	offset      cdc.Offset
	err         error
	connected   bool
	destination string
	acked       bool
	closed      bool
}

func (c *fakeCanalClient) Connect(ctx context.Context) error {
	c.connected = true
	return nil
}

func (c *fakeCanalClient) Subscribe(ctx context.Context, destination string) error {
	c.destination = destination
	return nil
}

func (c *fakeCanalClient) Fetch(ctx context.Context, batchSize int) ([]RowChange, cdc.Offset, error) {
	if c.err != nil {
		return nil, cdc.Offset{}, c.err
	}
	return c.rows, c.offset, nil
}

func (c *fakeCanalClient) Ack(ctx context.Context, offset cdc.Offset) error {
	c.acked = true
	return nil
}

func (c *fakeCanalClient) Close(ctx context.Context) error {
	c.closed = true
	return nil
}
