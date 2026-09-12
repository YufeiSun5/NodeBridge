package syncruntime

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type phaseTraceKey struct{}
type phaseTrace struct {
	mu        sync.Mutex
	durations map[string]time.Duration
	active    map[uint64]activePhase
	next      uint64
}

type activePhase struct {
	name    string
	started time.Time
}

func withPhaseTrace(ctx context.Context, enabled bool) (context.Context, *phaseTrace) {
	if !enabled {
		return ctx, nil
	}
	trace := &phaseTrace{durations: map[string]time.Duration{}, active: map[uint64]activePhase{}}
	return context.WithValue(ctx, phaseTraceKey{}, trace), trace
}

func measurePhase(ctx context.Context, phase string) func() {
	trace, _ := ctx.Value(phaseTraceKey{}).(*phaseTrace)
	if trace == nil {
		return func() {}
	}
	started := time.Now()
	trace.mu.Lock()
	trace.next++
	id := trace.next
	trace.active[id] = activePhase{name: phase, started: started}
	trace.mu.Unlock()
	return func() {
		elapsed := time.Since(started)
		trace.mu.Lock()
		trace.durations[phase] += elapsed
		delete(trace.active, id)
		trace.mu.Unlock()
	}
}

func (t *phaseTrace) milliseconds() map[string]float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make(map[string]float64, len(t.durations))
	for name, elapsed := range t.durations {
		result[name] = float64(elapsed.Microseconds()) / 1000
	}
	for _, phase := range t.active {
		result[phase.name] += float64(time.Since(phase.started).Microseconds()) / 1000
	}
	return result
}

func watchStep(ctx context.Context, logger *slog.Logger, name string, trace *phaseTrace, threshold time.Duration) func() {
	if logger == nil {
		return func() {}
	}
	done, finished := make(chan struct{}), make(chan struct{})
	started := time.Now()
	go func() {
		defer close(finished)
		timer := time.NewTimer(threshold)
		defer timer.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-timer.C:
				logger.Warn("worker step still running", "worker", name, "elapsed_ms", time.Since(started).Milliseconds(), "phase_ms", trace.milliseconds())
				timer.Reset(30 * time.Second)
			}
		}
	}()
	return func() { close(done); <-finished }
}
