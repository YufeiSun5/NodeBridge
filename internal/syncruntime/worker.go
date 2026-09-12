package syncruntime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/status"
	"github.com/go-sql-driver/mysql"
)

type Stepper interface {
	RunOnce(ctx context.Context) (StepResult, error)
}

type StoppableStepper interface {
	Stop(ctx context.Context) error
}

type WorkerConfig struct {
	Name              string
	IdleInterval      time.Duration
	ErrorInterval     time.Duration
	MaxSteps          int
	SlowStepThreshold time.Duration
	SummaryInterval   time.Duration
}

type Worker struct {
	Config  WorkerConfig
	Stepper Stepper
	Status  *status.RuntimeStore
	Sleep   func(context.Context, time.Duration) error
	Logger  *slog.Logger
}

func (w Worker) Run(ctx context.Context) (runErr error) {
	if w.Stepper == nil {
		return fmt.Errorf("stepper is required")
	}
	name := w.Config.Name
	if name == "" {
		name = "sync-worker"
	}
	if stoppable, ok := w.Stepper.(StoppableStepper); ok {
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := stoppable.Stop(stopCtx); err != nil {
				w.logStep(slog.LevelError, "worker cleanup failed", name, StepResult{Action: "stop"}, 0, err, nil, 0, 0)
				if runErr == nil {
					runErr = fmt.Errorf("stop %s: %w", name, err)
				}
			}
		}()
	}
	idleInterval := defaultDuration(w.Config.IdleInterval, time.Second)
	errorInterval := defaultDuration(w.Config.ErrorInterval, 5*time.Second)
	slowThreshold := defaultDuration(w.Config.SlowStepThreshold, 2*time.Second)
	summaryInterval := defaultDuration(w.Config.SummaryInterval, time.Minute)
	sleep := w.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	steps := 0
	consecutiveErrors := 0
	processedSteps, messages, failures := 0, 0, 0
	summaryStart := time.Now()
	w.logStep(slog.LevelInfo, "worker started", name, StepResult{Action: "start"}, 0, nil, nil, 0, 0)
	defer w.logStep(slog.LevelInfo, "worker loop stopped", name, StepResult{Action: "stop"}, 0, nil, nil, 0, 0)
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

		stepCtx, trace := withPhaseTrace(ctx, w.Logger != nil)
		started := time.Now()
		result, err := func() (StepResult, error) {
			stopWatch := watchStep(stepCtx, w.Logger, name, trace, slowThreshold)
			defer stopWatch()
			return w.Stepper.RunOnce(stepCtx)
		}()
		elapsed := time.Since(started)
		if ctx.Err() != nil {
			w.recordStopped(name)
			return ctx.Err()
		}
		if err != nil {
			consecutiveErrors++
			failures++
			w.recordError(name, err)
			w.logStep(slog.LevelError, "worker step failed; retry scheduled", name, result, elapsed, err, trace, consecutiveErrors, errorInterval)
			if sleepErr := sleep(ctx, errorInterval); sleepErr != nil {
				w.recordStopped(name)
				return sleepErr
			}
			continue
		}
		if consecutiveErrors > 0 {
			w.logStep(slog.LevelInfo, "worker step recovered", name, result, elapsed, nil, trace, consecutiveErrors, 0)
			consecutiveErrors = 0
		}
		if result.Processed {
			processedSteps++
			messages += result.Count
			if elapsed >= slowThreshold {
				w.logStep(slog.LevelWarn, "slow worker step", name, result, elapsed, nil, trace, 0, 0)
			}
		}
		if w.Logger != nil && time.Since(summaryStart) >= summaryInterval {
			w.Logger.Info("worker interval", "worker", name, "window_ms", time.Since(summaryStart).Milliseconds(), "processed_steps", processedSteps, "reported_messages", messages, "errors", failures)
			processedSteps, messages, failures = 0, 0, 0
			summaryStart = time.Now()
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

func (w Worker) logStep(level slog.Level, message, name string, result StepResult, elapsed time.Duration, err error, trace *phaseTrace, failures int, retry time.Duration) {
	if w.Logger == nil {
		return
	}
	var detail *EventFailure
	if errors.As(err, &detail) {
		result.EventID = detail.EventID
	}
	attrs := []any{"worker", name, "action", result.Action, "event_id", result.EventID, "batch_count", result.Count, "dispatch_count", result.DispatchCount, "elapsed_ms", float64(elapsed.Microseconds()) / 1000}
	if trace != nil {
		attrs = append(attrs, "phase_ms", trace.milliseconds())
	}
	if err != nil {
		attrs = append(attrs, "error", err, "error_type", fmt.Sprintf("%T", err), "consecutive_errors", failures, "retry_after_ms", retry.Milliseconds())
		attrs = append(attrs, "retry_at", time.Now().Add(retry).Format(time.RFC3339Nano))
		if detail != nil {
			attrs = append(attrs, "rule_id", detail.RuleID, "source_database", detail.SourceDatabase, "source_table", detail.SourceTable, "target_database", detail.TargetDatabase, "target_table", detail.TargetTable, "binlog_file", detail.BinlogFile, "binlog_pos", detail.BinlogPos, "gtid", detail.GTID)
		}
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) {
			attrs = append(attrs, "mysql_error_code", mysqlErr.Number)
		}
	}
	if err == nil && failures > 0 {
		attrs = append(attrs, "recovered_after_errors", failures)
	}
	w.Logger.Log(context.Background(), level, message, attrs...)
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
