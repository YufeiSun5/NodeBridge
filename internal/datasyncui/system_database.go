package datasyncui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

// InitializeSystemDatabase uses the saved configuration and the installer upgrade path.
func (a *App) InitializeSystemDatabase() uiapi.OperationResult {
	if err := a.requireAdmin(); err != nil {
		return uiapi.OperationResult{OK: false, Status: uiapi.StateLocked, Message: err.Error()}
	}
	executable, err := newExternalAgentController().resolveExecutable()
	if err != nil {
		return uiapi.OperationResult{OK: false, Status: "error", Message: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "upgrade-system", "-config", a.effectiveConfigPath(), "-create-database")
	configureBackgroundProcess(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return uiapi.OperationResult{OK: false, Status: "error", Message: agentlog.ForConfig(a.config)(fmt.Sprintf("system database initialization failed: %v: %s", err, strings.TrimSpace(string(output))))}
	}
	return uiapi.OperationResult{OK: true, Status: "ready", Message: strings.TrimSpace(string(output))}
}
