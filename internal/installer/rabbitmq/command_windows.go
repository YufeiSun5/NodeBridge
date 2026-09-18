package rabbitmq

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func adminCommand(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	ext := filepath.Ext(name)
	if !strings.EqualFold(ext, ".bat") && !strings.EqualFold(ext, ".cmd") {
		return exec.CommandContext(ctx, name, args...), nil
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	values := append([]string{path}, args...)
	for i, value := range values {
		// Batch files may reparse arguments; reject expansion and quote breakers.
		if strings.ContainsAny(value, "\"%!\r\n\x00") {
			return nil, fmt.Errorf("unsupported character in RabbitMQ batch argument %d", i)
		}
		values[i] = `"` + value + `"`
	}
	shell := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	cmd := exec.CommandContext(ctx, shell)
	// Go's default Windows quoting targets executables, not cmd batch syntax.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CmdLine: `"` + shell + `" /d /v:off /s /c "` + strings.Join(values, " ") + `"`}
	return cmd, nil
}
