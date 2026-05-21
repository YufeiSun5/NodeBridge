package cdc

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Offset struct {
	ReaderName string    `json:"reader_name"`
	BinlogFile string    `json:"binlog_file,omitempty"`
	BinlogPos  uint32    `json:"binlog_pos,omitempty"`
	GTID       string    `json:"gtid,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (o Offset) Validate() error {
	if o.ReaderName == "" {
		return fmt.Errorf("reader name is required")
	}
	if o.BinlogFile == "" && o.GTID == "" {
		return fmt.Errorf("binlog file or gtid is required")
	}
	return nil
}

type OffsetStore interface {
	Load(ctx context.Context, readerName string) (Offset, bool, error)
	Save(ctx context.Context, offset Offset) error
}

type MemoryOffsetStore struct {
	mu      sync.RWMutex
	offsets map[string]Offset
}

func NewMemoryOffsetStore() *MemoryOffsetStore {
	return &MemoryOffsetStore{offsets: make(map[string]Offset)}
}

func (s *MemoryOffsetStore) Load(ctx context.Context, readerName string) (Offset, bool, error) {
	if err := ctx.Err(); err != nil {
		return Offset{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	offset, ok := s.offsets[readerName]
	return offset, ok, nil
}

func (s *MemoryOffsetStore) Save(ctx context.Context, offset Offset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := offset.Validate(); err != nil {
		return err
	}
	if offset.UpdatedAt.IsZero() {
		offset.UpdatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offsets[offset.ReaderName] = offset
	return nil
}
