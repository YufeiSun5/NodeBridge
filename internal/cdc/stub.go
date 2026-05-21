package cdc

import (
	"context"
	"sync"
)

type StubSource struct {
	mu      sync.Mutex
	changes []ChangeEvent
}

func NewStubSource(changes []ChangeEvent) *StubSource {
	copied := append([]ChangeEvent(nil), changes...)
	return &StubSource{changes: copied}
}

func (s *StubSource) GetChange(ctx context.Context) (ChangeEvent, bool, error) {
	if err := ctx.Err(); err != nil {
		return ChangeEvent{}, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.changes) == 0 {
		return ChangeEvent{}, false, nil
	}
	change := s.changes[0]
	s.changes = s.changes[1:]
	return change, true, nil
}
