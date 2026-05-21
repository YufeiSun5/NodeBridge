package status

import (
	"fmt"
	"sync"
	"time"
)

const (
	WorkerRunning = "running"
	WorkerIdle    = "idle"
	WorkerError   = "error"
	WorkerStopped = "stopped"
)

type WorkerStatus struct {
	Name             string    `json:"name"`
	State            string    `json:"state"`
	LastEventID      string    `json:"last_event_id,omitempty"`
	LastAction       string    `json:"last_action,omitempty"`
	LastError        string    `json:"last_error,omitempty"`
	ProcessedCount   int64     `json:"processed_count"`
	ErrorCount       int64     `json:"error_count"`
	DispatchCount    int64     `json:"dispatch_count"`
	LastProcessedAt  time.Time `json:"last_processed_at,omitempty"`
	LastTransitionAt time.Time `json:"last_transition_at"`
}

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Worker  string    `json:"worker"`
	EventID string    `json:"event_id,omitempty"`
	Action  string    `json:"action,omitempty"`
	Message string    `json:"message"`
}

type RuntimeSnapshot struct {
	Workers []WorkerStatus `json:"workers"`
	Logs    []LogEntry     `json:"logs"`
}

type RuntimeStore struct {
	clock    func() time.Time
	mu       sync.RWMutex
	workers  map[string]WorkerStatus
	logs     []LogEntry
	logLimit int
}

func NewRuntimeStore() *RuntimeStore {
	return &RuntimeStore{
		clock:    time.Now,
		workers:  make(map[string]WorkerStatus),
		logLimit: 200,
	}
}

func (s *RuntimeStore) RecordProcessed(name, eventID, action string, dispatchCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	current := s.workers[name]
	current.Name = name
	current.State = WorkerRunning
	current.LastEventID = eventID
	current.LastAction = action
	current.LastError = ""
	current.ProcessedCount++
	current.DispatchCount += int64(dispatchCount)
	current.LastProcessedAt = now
	current.LastTransitionAt = now
	s.workers[name] = current
	s.appendLogLocked(LogEntry{
		Time:    now,
		Level:   "INFO",
		Worker:  name,
		EventID: eventID,
		Action:  action,
		Message: fmt.Sprintf("worker processed action=%s dispatch=%d", action, dispatchCount),
	})
}

func (s *RuntimeStore) RecordIdle(name string) {
	s.setState(name, WorkerIdle, "", "DEBUG", "worker idle")
}

func (s *RuntimeStore) RecordError(name string, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	current := s.workers[name]
	current.Name = name
	current.State = WorkerError
	current.LastError = message
	current.ErrorCount++
	current.LastTransitionAt = now
	s.workers[name] = current
	s.appendLogLocked(LogEntry{
		Time:    now,
		Level:   "ERROR",
		Worker:  name,
		Message: message,
	})
}

func (s *RuntimeStore) RecordStopped(name string) {
	s.setState(name, WorkerStopped, "", "INFO", "worker stopped")
}

func (s *RuntimeStore) Snapshot() RuntimeSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make([]WorkerStatus, 0, len(s.workers))
	for _, worker := range s.workers {
		workers = append(workers, worker)
	}
	logs := append([]LogEntry(nil), s.logs...)
	return RuntimeSnapshot{Workers: workers, Logs: logs}
}

func (s *RuntimeStore) setState(name, state, errMessage, level, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	current := s.workers[name]
	current.Name = name
	current.State = state
	current.LastError = errMessage
	current.LastTransitionAt = now
	s.workers[name] = current
	s.appendLogLocked(LogEntry{
		Time:    now,
		Level:   level,
		Worker:  name,
		Message: message,
	})
}

func (s *RuntimeStore) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

func (s *RuntimeStore) appendLogLocked(entry LogEntry) {
	// Redacted by design. / 默认脱敏。 / 既定でマスク。
	if s.logLimit <= 0 {
		return
	}
	s.logs = append(s.logs, entry)
	if len(s.logs) > s.logLimit {
		s.logs = append([]LogEntry(nil), s.logs[len(s.logs)-s.logLimit:]...)
	}
}
