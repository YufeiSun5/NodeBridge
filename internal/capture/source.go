package capture

import (
	"context"
	"errors"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
)

type BatchSource interface {
	Start(context.Context) error
	Stop(context.Context) error
	FetchChangesOnce(context.Context) ([]cdc.ChangeEvent, cdc.Offset, error)
	Commit(context.Context, cdc.Offset) error
}

// Source removes internal pulses before normalization and releases waiters only
// after the runtime commits the complete batch. It has one sequential consumer.
type Source struct {
	source  BatchSource
	fence   *Fence
	batchID int64
	pulses  []cdc.ChangeEvent
}

func NewSource(source BatchSource, fence *Fence) (*Source, error) {
	if source == nil || fence == nil {
		return nil, errors.New("capture_source_dependencies_required")
	}
	return &Source{source: source, fence: fence}, nil
}

func (s *Source) Start(ctx context.Context) error { return s.source.Start(ctx) }

func (s *Source) Stop(ctx context.Context) error {
	s.batchID, s.pulses = 0, nil
	return s.source.Stop(ctx)
}

func (s *Source) FetchChangesOnce(ctx context.Context) ([]cdc.ChangeEvent, cdc.Offset, error) {
	if s.batchID != 0 {
		return nil, cdc.Offset{}, errors.New("capture_batch_uncommitted")
	}
	changes, offset, err := s.source.FetchChangesOnce(ctx)
	if err != nil {
		return nil, cdc.Offset{}, err
	}
	if len(changes) > 0 && !offset.HasCanalBatch() {
		return nil, cdc.Offset{}, errors.New("capture_batch_id_required")
	}
	if offset.HasCanalBatch() {
		s.batchID = offset.BatchID
	}
	forward := make([]cdc.ChangeEvent, 0, len(changes))
	for _, change := range changes {
		if s.fence.isPulse(change) {
			s.pulses = append(s.pulses, change)
		} else {
			forward = append(forward, change)
		}
	}
	return forward, offset, nil
}

func (s *Source) Commit(ctx context.Context, offset cdc.Offset) error {
	if s.batchID <= 0 || offset.BatchID != s.batchID {
		return errors.New("capture_batch_mismatch")
	}
	if err := s.source.Commit(ctx, offset); err != nil {
		return err
	}
	s.fence.observe(s.pulses)
	s.batchID, s.pulses = 0, nil
	return nil
}
