package canal

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
)

type countingCheckpointStore struct {
	cdc.OffsetStore
	saves int
}

func (s *countingCheckpointStore) Save(ctx context.Context, offset cdc.Offset) error {
	s.saves++
	return s.OffsetStore.Save(ctx, offset)
}

func TestAcknowledgmentOnlyBatchDoesNotCreateCheckpointFeedback(t *testing.T) {
	ctx := context.Background()
	store := &countingCheckpointStore{OffsetStore: cdc.NewMemoryOffsetStore()}
	client := &fakeCanalClient{}
	adapter, err := NewAdapter(Config{ReaderName: "server-001", Address: "localhost:11111", Destination: "server-001"}, client, store)
	if err != nil {
		t.Fatal(err)
	}
	business := cdc.Offset{BatchID: 1, BinlogFile: "mysql-bin.000001", BinlogPos: 100}
	if err := adapter.Commit(ctx, business); err != nil {
		t.Fatal(err)
	}
	for i := 2; i < 20; i++ {
		client.acked = false
		if err := adapter.Commit(ctx, cdc.Offset{BatchID: int64(i), BinlogFile: business.BinlogFile, BinlogPos: uint32(i * 100), SkipCheckpoint: true}); err != nil || !client.acked {
			t.Fatalf("ACK-only commit failed: acked=%t err=%v", client.acked, err)
		}
	}
	saved, ok, err := store.Load(ctx, "server-001")
	if err != nil || !ok || saved.BinlogPos != 100 || store.saves != 1 {
		t.Fatalf("checkpoint feedback: saved=%+v saves=%d err=%v", saved, store.saves, err)
	}
	business.BatchID, business.BinlogPos = 20, 2000
	if err := adapter.Commit(ctx, business); err != nil || store.saves != 2 {
		t.Fatalf("next business checkpoint not persisted: saves=%d err=%v", store.saves, err)
	}
	client.ackErr = errors.New("ACK failed")
	if err := adapter.Commit(ctx, cdc.Offset{BatchID: 21, SkipCheckpoint: true}); !errors.Is(err, client.ackErr) {
		t.Fatalf("ACK-only failure was hidden: %v", err)
	}
}
