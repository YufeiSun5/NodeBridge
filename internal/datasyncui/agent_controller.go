package datasyncui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/status"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

var (
	errAgentAlreadyRunning = errors.New("agent already running")
	errAgentNotRunning     = errors.New("agent is not running")
)

type agentController interface {
	Start(ctx context.Context, configPath, rulesPath, stopFile string) error
	Stop(ctx context.Context, stopFile string, gracefulTimeout time.Duration) (string, error)
	Running() bool
	Status() uiapi.AgentProcessStatus
}

type externalAgentController struct {
	mu         sync.Mutex
	executable string
	cmd        *exec.Cmd
	done       chan error
	logFile    *os.File
	status     string
	pid        int
	startedAt  time.Time
	exitedAt   time.Time
	lastError  string
	logPath    string
}

func newExternalAgentController() *externalAgentController {
	return &externalAgentController{executable: stringsFromEnvOrDefault("NODEBRIDGE_SYNC_AGENT_PATH", "")}
}

func (c *externalAgentController) Start(ctx context.Context, configPath, rulesPath, stopFile string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.runningLocked() {
		return errAgentAlreadyRunning
	}
	executable, err := c.resolveExecutable()
	if err != nil {
		c.status = status.AgentError
		c.lastError = err.Error()
		return err
	}
	args := []string{"run", "-config", configPath, "-rules", rulesPath}
	if stopFile != "" {
		args = append(args, "-stop-file", stopFile)
		_ = os.Remove(stopFile)
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	logFile, err := openAgentLog(configPath)
	if err != nil {
		c.status = status.AgentError
		c.lastError = err.Error()
		return err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		c.status = status.AgentError
		c.lastError = err.Error()
		return fmt.Errorf("start sync-agent: %w", err)
	}
	done := make(chan error, 1)
	c.cmd = cmd
	c.done = done
	c.logFile = logFile
	c.executable = executable
	c.pid = cmd.Process.Pid
	c.startedAt = time.Now()
	c.exitedAt = time.Time{}
	c.status = status.AgentRunning
	c.lastError = ""
	c.logPath = agentLogPath(configPath)
	go func() {
		err := cmd.Wait()
		done <- err
		c.mu.Lock()
		if c.cmd == cmd {
			c.cmd = nil
			c.done = nil
			c.pid = 0
			c.exitedAt = time.Now()
			if c.status == status.AgentRunning {
				if err != nil {
					c.status = status.AgentError
					c.lastError = err.Error()
				} else {
					c.status = "exited"
				}
			}
			if c.logFile != nil {
				_ = c.logFile.Close()
				c.logFile = nil
			}
		}
		c.mu.Unlock()
	}()
	return nil
}

func (c *externalAgentController) Stop(ctx context.Context, stopFile string, gracefulTimeout time.Duration) (string, error) {
	c.mu.Lock()
	cmd := c.cmd
	done := c.done
	if cmd == nil || done == nil {
		c.mu.Unlock()
		return status.AgentStopped, errAgentNotRunning
	}
	c.mu.Unlock()

	if stopFile != "" {
		if err := os.MkdirAll(filepath.Dir(stopFile), 0o755); err != nil {
			c.markStopped(status.AgentError, err.Error())
			return status.AgentError, fmt.Errorf("create stop file directory: %w", err)
		}
		if err := os.WriteFile(stopFile, []byte(time.Now().Format(time.RFC3339Nano)), 0o600); err != nil {
			c.markStopped(status.AgentError, err.Error())
			return status.AgentError, fmt.Errorf("write stop file: %w", err)
		}
	}
	if gracefulTimeout <= 0 {
		gracefulTimeout = 10 * time.Second
	}
	gracefulTimer := time.NewTimer(gracefulTimeout)
	defer gracefulTimer.Stop()

	select {
	case <-done:
		c.markStopped(status.AgentStopped, "")
		return status.AgentStopped, nil
	case <-gracefulTimer.C:
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				c.markStopped(status.AgentError, err.Error())
				return status.AgentError, fmt.Errorf("stop sync-agent: %w", err)
			}
		}
		select {
		case <-done:
			c.markStopped("forced_stopped", "")
			return "forced_stopped", nil
		case <-ctx.Done():
			c.markStopped(status.AgentError, ctx.Err().Error())
			return status.AgentError, ctx.Err()
		}
	case <-ctx.Done():
		c.markStopped(status.AgentError, ctx.Err().Error())
		return status.AgentError, ctx.Err()
	}
}

func (c *externalAgentController) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runningLocked()
}

func (c *externalAgentController) Status() uiapi.AgentProcessStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.status
	if state == "" {
		state = status.AgentStopped
	}
	if c.runningLocked() {
		state = status.AgentRunning
	}
	return uiapi.AgentProcessStatus{
		ExecutablePath: c.executable,
		PID:            c.pid,
		Status:         state,
		StartedAt:      uiapi.TimeString(c.startedAt),
		ExitedAt:       uiapi.TimeString(c.exitedAt),
		LastError:      c.lastError,
		LogPath:        c.logPath,
	}
}

func (c *externalAgentController) runningLocked() bool {
	return c.cmd != nil && c.done != nil
}

func (c *externalAgentController) markStopped(state, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = state
	c.pid = 0
	c.exitedAt = time.Now()
	c.lastError = message
}

func (c *externalAgentController) resolveExecutable() (string, error) {
	if c.executable != "" {
		return c.executable, nil
	}
	if env := stringsFromEnvOrDefault("NODEBRIDGE_SYNC_AGENT_PATH", ""); env != "" {
		return env, nil
	}
	if exe, err := executablePath(); err == nil {
		dir := filepath.Dir(exe)
		for _, name := range []string{"SyncAgent.exe", "sync-agent.exe"} {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	if path, err := exec.LookPath("sync-agent.exe"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("sync-agent"); err == nil {
		return path, nil
	}
	return "", errors.New("sync-agent executable not found; set NODEBRIDGE_SYNC_AGENT_PATH")
}

func stringsFromEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func agentLogPath(configPath string) string {
	baseDir := filepath.Dir(configPath)
	if baseDir == "." || baseDir == "" {
		baseDir = os.TempDir()
	}
	return filepath.Join(baseDir, "logs", "sync-agent.log")
}

func agentStopFilePath(configPath string) string {
	baseDir := filepath.Dir(configPath)
	if baseDir == "." || baseDir == "" {
		baseDir = os.TempDir()
	}
	return filepath.Join(baseDir, "run", "sync-agent.stop")
}

func openAgentLog(configPath string) (*os.File, error) {
	path := agentLogPath(configPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create agent log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open agent log: %w", err)
	}
	return file, nil
}
