package cdc

import (
	"context"
	"errors"
	"testing"
)

func TestStubSourceReturnsChangesInOrder(t *testing.T) {
	source := NewStubSource([]ChangeEvent{
		{DatabaseName: "db1", TableName: "t1"},
		{DatabaseName: "db2", TableName: "t2"},
	})

	first, ok, err := source.GetChange(context.Background())
	if err != nil || !ok {
		t.Fatalf("expected first change, ok=%t err=%v", ok, err)
	}
	second, ok, err := source.GetChange(context.Background())
	if err != nil || !ok {
		t.Fatalf("expected second change, ok=%t err=%v", ok, err)
	}
	_, ok, err = source.GetChange(context.Background())
	if err != nil || ok {
		t.Fatalf("expected empty source, ok=%t err=%v", ok, err)
	}
	if first.DatabaseName != "db1" || second.DatabaseName != "db2" {
		t.Fatalf("unexpected order first=%+v second=%+v", first, second)
	}
}

func TestStubSourceHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := NewStubSource([]ChangeEvent{{DatabaseName: "db"}}).GetChange(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
