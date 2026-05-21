package cdc

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryOffsetStoreSaveLoad(t *testing.T) {
	store := NewMemoryOffsetStore()
	offset := Offset{
		ReaderName: "edge-001",
		BinlogFile: "mysql-bin.000001",
		BinlogPos:  42,
	}

	if err := store.Save(context.Background(), offset); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	loaded, ok, err := store.Load(context.Background(), "edge-001")
	if err != nil || !ok {
		t.Fatalf("expected loaded offset, ok=%t err=%v", ok, err)
	}
	if loaded.BinlogFile != "mysql-bin.000001" || loaded.BinlogPos != 42 || loaded.UpdatedAt.IsZero() {
		t.Fatalf("unexpected offset %+v", loaded)
	}
}

func TestMemoryOffsetStoreRejectsInvalidOffset(t *testing.T) {
	store := NewMemoryOffsetStore()
	if err := store.Save(context.Background(), Offset{}); err == nil {
		t.Fatal("expected invalid offset error")
	}
	if err := store.Save(context.Background(), Offset{ReaderName: "edge-001"}); err == nil {
		t.Fatal("expected missing position error")
	}
}

func TestMemoryOffsetStoreHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := NewMemoryOffsetStore()

	if err := store.Save(ctx, Offset{ReaderName: "edge-001", GTID: "gtid"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled save, got %v", err)
	}
	if _, _, err := store.Load(ctx, "edge-001"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled load, got %v", err)
	}
}
