package syncruntime

import (
	"context"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/status"
)

type Stepper interface {
	RunOnce(ctx context.Context) (StepResult, error)
}

type WorkerConfig struct {
	Name          string
	IdleInterval  time.Duration
	ErrorInterval time.Duration
	MaxSteps      int
}

type Worker struct {
	Config  WorkerConfig
	Stepper Stepper
	Status  *status.RuntimeStore
	Sleep   func(context.Context, time.Duration) error
}

func (w Worker) Run(ctx context.Context) error {
	if w.Stepper == nil {
		return fmt.Errorf("stepper is required")
	}
	name := w.Config.Name
	if name == "" {
		name = "sync-worker"
	}
	idleInterval := defaultDuration(w.Config.IdleInterval, time.Second)
	errorInterval := defaultDuration(w.Config.ErrorInterval, 5*time.Second)
	sleep := w.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	steps := 0
	for {
		if err := ctx.Err(); err != nil {
			w.recordStopped(name)
			return err
		}
		if w.Config.MaxSteps > 0 && steps >= w.Config.MaxSteps {
			w.recordStopped(name)
			return nil
		}
		steps++

		result, err := w.Stepper.RunOnce(ctx)
		if err != nil {
			w.recordError(name, err)
			if sleepErr := sleep(ctx, errorInterval); sleepErr != nil {
				w.recordStopped(name)
				return sleepErr
			}
			continue
		}
		if result.Processed {
			w.recordProcessed(name, result)
			continue
		}

		w.recordIdle(name)
		if sleepErr := sleep(ctx, idleInterval); sleepErr != nil {
			w.recordStopped(name)
			return sleepErr
		}
	}
}

func (w Worker) recordProcessed(name string, result StepResult) {
	if w.Status != nil {
		w.Status.RecordProcessed(name, result.EventID, result.Action, result.DispatchCount)
	}
}

func (w Worker) recordIdle(name string) {
	if w.Status != nil {
		w.Status.RecordIdle(name)
	}
}

func (w Worker) recordError(name string, err error) {
	if w.Status != nil {
		w.Status.RecordError(name, err)
	}
}

func (w Worker) recordStopped(name string) {
	if w.Status != nil {
		w.Status.RecordStopped(name)
	}
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
