package agentstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var ErrRunning = errors.New("agent already running for this configuration")

type State struct {
	PID        int    `json:"pid"`
	Executable string `json:"executable"`
	StartedAt  string `json:"started_at"`
	StopFile   string `json:"stop_file"`
}

func paths(config string) (string, string) {
	return config + ".agent.lock", config + ".agent.json"
}

// Lock is released by the OS even if the process crashes.
func Lock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func Acquire(config, stopFile string) (func(), error) {
	lockPath, statePath := paths(config)
	f, err := Lock(lockPath)
	if err != nil {
		return nil, err
	}
	exe, _ := os.Executable()
	data, _ := json.Marshal(State{PID: os.Getpid(), Executable: exe, StartedAt: time.Now().Format(time.RFC3339Nano), StopFile: stopFile})
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() { _ = os.Remove(statePath); _ = f.Close() }, nil
}

func Read(config string) (*State, error) {
	lockPath, statePath := paths(config)
	if _, err := os.Stat(lockPath); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	f, err := Lock(lockPath)
	if err == nil {
		_ = f.Close()
		return nil, nil
	}
	if !errors.Is(err, ErrRunning) {
		return nil, err
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("agent is starting; retry status: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
