package datasyncui

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/status"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

func TestSaveConfigPersistsAndRedactsSecrets(t *testing.T) {
	app, configPath, _ := newTempApp(t)
	unlockTestAdmin(t, app)
	_, err := app.SaveConfig(uiapi.SaveConfigRequest{Config: uiapi.ConfigFromApp(validConfig())})
	if err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	cfg := app.GetConfig()
	if cfg.MySQL.Password != appconfig.RedactedSecret || cfg.RabbitMQ.Password != appconfig.RedactedSecret ||
		cfg.CDC.Password != appconfig.RedactedSecret || cfg.LogWeb.Token != appconfig.RedactedSecret ||
		cfg.Security.AdminPassword != appconfig.RedactedSecret || cfg.Security.ExitPassword != appconfig.RedactedSecret {
		t.Fatalf("expected redacted config, got %+v", cfg)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "secret") || strings.Contains(string(data), "admin-pass") ||
		strings.Contains(string(data), "exit-pass") {
		t.Fatalf("config file contains plaintext secret:\n%s", string(data))
	}
}

func TestSaveConfigKeepsExistingSecretWhenRedacted(t *testing.T) {
	app, _, _ := newTempApp(t)
	unlockTestAdmin(t, app)
	if _, err := app.SaveConfig(uiapi.SaveConfigRequest{Config: uiapi.ConfigFromApp(validConfig())}); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}
	redacted := app.GetConfig()
	redacted.Node.Name = "changed"
	if _, err := app.SaveConfig(uiapi.SaveConfigRequest{Config: redacted}); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}
	if app.config.MySQL.Password != "secret" || app.config.Security.AdminPassword != "admin-pass" ||
		app.config.Security.ExitPassword != "exit-pass" {
		t.Fatalf("expected secrets to be preserved, got %+v", app.config)
	}
}

func TestSaveConfigValidates(t *testing.T) {
	app, _, _ := newTempApp(t)
	unlockTestAdmin(t, app)
	_, err := app.SaveConfig(uiapi.SaveConfigRequest{Config: uiapi.ConfigDTO{}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSaveConfigRequiresAdminPassword(t *testing.T) {
	app, _, _ := newTempApp(t)
	unlockTestAdmin(t, app)
	cfg := validConfig()
	cfg.Security.AdminPassword = ""
	_, err := app.SaveConfig(uiapi.SaveConfigRequest{Config: uiapi.ConfigFromApp(cfg)})
	if err == nil || !strings.Contains(err.Error(), "security.admin_password") {
		t.Fatalf("expected admin password validation error, got %v", err)
	}
}

func TestSyncRulesRoundTripPersists(t *testing.T) {
	app, _, rulesPath := newTempApp(t)
	unlockTestAdmin(t, app)
	saved, err := app.SaveSyncRules(uiapi.SaveSyncRulesRequest{Rules: []rules.SyncRule{
		{
			ID:             "device",
			DatabaseName:   "scada_edge",
			TableName:      "device_config",
			Direction:      rules.DirectionBidirectional,
			ConflictPolicy: rules.ConflictLastWriteWin,
			Enable:         true,
			PrimaryKeys:    []string{"id"},
		},
	}})
	if err != nil {
		t.Fatalf("SaveSyncRules returned error: %v", err)
	}
	if len(saved.Rules) != 1 {
		t.Fatalf("unexpected saved rules %+v", saved)
	}

	if _, err := os.Stat(rulesPath); err != nil {
		t.Fatalf("expected rules file: %v", err)
	}
	app.ruleSet = nil
	loaded, err := app.GetSyncRules()
	if err != nil {
		t.Fatalf("GetSyncRules returned error: %v", err)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].ID != "device" {
		t.Fatalf("unexpected loaded rules %+v", loaded)
	}
}

func TestGetSyncRulesFallsBackToExample(t *testing.T) {
	app, _, _ := newTempApp(t)
	app.rulesPath = filepath.Join(t.TempDir(), "missing-sync-rules.yaml")
	loaded, err := app.GetSyncRules()
	if err != nil {
		t.Fatalf("GetSyncRules returned error: %v", err)
	}
	if len(loaded.Rules) == 0 {
		t.Fatal("expected example rules fallback")
	}
	if loaded.Rules[0].ID == "" {
		t.Fatalf("expected usable example rule IDs, got %+v", loaded.Rules)
	}
	foundFieldRule := false
	for _, rule := range loaded.Rules {
		if rule.ID == "data-all-edge-010" {
			foundFieldRule = true
		}
	}
	if !foundFieldRule {
		t.Fatalf("expected field fallback rules, got %+v", loaded.Rules)
	}
	if _, err := os.Stat(app.rulesPath); err != nil {
		t.Fatalf("expected fallback rules to be persisted: %v", err)
	}
}

func TestSaveSyncRulesRejectsInvalidMapping(t *testing.T) {
	app, _, _ := newTempApp(t)
	unlockTestAdmin(t, app)
	_, err := app.SaveSyncRules(uiapi.SaveSyncRulesRequest{Rules: []rules.SyncRule{
		{DatabaseName: "scada_edge", TableName: "device_config", PrimaryKeys: []string{"id"}, TargetPrimaryKeys: []string{"id", "extra"}},
	}})
	if err == nil {
		t.Fatal("expected invalid rule error")
	}
}

func TestEmptyStates(t *testing.T) {
	app, _, _ := newTempApp(t)
	if queues := app.GetQueueStatus(); len(queues.Queues) == 0 {
		t.Fatal("expected stable queue placeholders")
	}
	failures, err := app.GetFailedEvents(uiapi.FailedEventsRequest{})
	if err != nil {
		t.Fatalf("GetFailedEvents returned error: %v", err)
	}
	if len(failures.Items) != 0 {
		t.Fatalf("expected empty failed events, got %+v", failures)
	}
	if logs := app.GetLogs(uiapi.LogQuery{}); len(logs.Items) != 0 {
		t.Fatalf("expected empty logs, got %+v", logs)
	}
	if result := app.StartAgent(); result.OK || result.Status != uiapi.StateLocked {
		t.Fatalf("expected locked start result, got %+v", result)
	}
}

func TestVerifyExitPassword(t *testing.T) {
	app, _, _ := newTempApp(t)
	cfg := validConfig()
	app.config = &cfg
	if result := app.VerifyExitPassword(uiapi.VerifyExitPasswordRequest{Password: "bad"}); result.OK {
		t.Fatalf("expected bad password to fail, got %+v", result)
	}
	if result := app.VerifyExitPassword(uiapi.VerifyExitPasswordRequest{Password: "exit-pass"}); !result.OK {
		t.Fatalf("expected good password to pass, got %+v", result)
	}
}

func TestVerifyExitPasswordAllowsExitWhenNotConfigured(t *testing.T) {
	app, _, _ := newTempApp(t)
	cfg := validConfig()
	cfg.Security.ExitPassword = ""
	app.config = &cfg
	if result := app.VerifyExitPassword(uiapi.VerifyExitPasswordRequest{}); !result.OK {
		t.Fatalf("expected no password config to allow exit, got %+v", result)
	}
}

func TestRequestExitArmsSingleClose(t *testing.T) {
	app, _, _ := newTempApp(t)
	cfg := validConfig()
	app.config = &cfg
	if result := app.RequestExit(uiapi.VerifyExitPasswordRequest{Password: "bad"}); result.OK {
		t.Fatalf("expected bad password to fail, got %+v", result)
	}
	if app.consumeExitRequest() {
		t.Fatal("bad password must not arm exit")
	}
	if result := app.RequestExit(uiapi.VerifyExitPasswordRequest{Password: "exit-pass"}); !result.OK {
		t.Fatalf("expected good password to arm exit, got %+v", result)
	}
	if !app.consumeExitRequest() {
		t.Fatal("expected exit request to be armed")
	}
	if app.consumeExitRequest() {
		t.Fatal("exit request must be single-use")
	}
}

func TestGetQueueStatusMarksErrorsWhenRabbitMQUnavailable(t *testing.T) {
	app, _, _ := newTempApp(t)
	cfg := validConfig()
	cfg.RabbitMQ.LocalURL = "amqp://guest:guest@127.0.0.1:1/edge-sync"
	app.config = &cfg
	queues := app.GetQueueStatus()
	if len(queues.Queues) == 0 {
		t.Fatal("expected queues")
	}
	for _, queue := range queues.Queues {
		if queue.Status != status.AgentError {
			t.Fatalf("expected queue error state, got %+v", queues)
		}
	}
}

func TestAutoStart(t *testing.T) {
	app, _, _ := newTempApp(t)
	if current := app.GetAutoStart(); current.Enabled {
		t.Fatalf("expected disabled autostart, got %+v", current)
	}
	unlockTestAdmin(t, app)
	enabled := app.SetAutoStart(uiapi.SetAutoStartRequest{Enabled: true})
	if !enabled.Enabled || enabled.Status != status.AgentRunning {
		t.Fatalf("expected enabled autostart, got %+v", enabled)
	}
	disabled := app.SetAutoStart(uiapi.SetAutoStartRequest{Enabled: false})
	if disabled.Enabled {
		t.Fatalf("expected disabled autostart, got %+v", disabled)
	}
}

func TestLogsFilterAndDiagnosticPackage(t *testing.T) {
	app, _, _ := newTempApp(t)
	cfg := validConfig()
	app.config = &cfg
	unlockTestAdmin(t, app)
	app.runtime.RecordProcessed("server-ingress", "evt-001", "apply", 1)
	logs := app.GetLogs(uiapi.LogQuery{Module: "server-ingress", Limit: 10})
	if len(logs.Items) != 1 || logs.Items[0].Module != "server-ingress" {
		t.Fatalf("unexpected logs %+v", logs)
	}

	result, err := app.ExportDiagnosticPackage()
	if err != nil {
		t.Fatalf("ExportDiagnosticPackage returned error: %v", err)
	}
	reader, err := zip.OpenReader(result.Path)
	if err != nil {
		t.Fatalf("open diagnostic zip: %v", err)
	}
	defer reader.Close()
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, name := range []string{"config.redacted.json", "sync-rules.json", "overview.json", "logs.json", "agent-process.json", "mcp-status.json", "managed-install-plan.json"} {
		if !names[name] {
			t.Fatalf("missing diagnostic file %s in %+v", name, names)
		}
	}
}

func TestOverviewReturnsExplicitConfigState(t *testing.T) {
	app, configPath, rulesPath := newTempApp(t)
	cfg := validConfig()
	app.config = &cfg
	overview := app.GetOverview()
	if !overview.ConfigLoaded || overview.ConfigPath != configPath || overview.RulesPath != rulesPath {
		t.Fatalf("expected explicit config state, got %+v", overview)
	}
	if overview.NodeID != "edge-001" || overview.NodeName != "Edge 001" {
		t.Fatalf("expected node identity in overview, got %+v", overview)
	}
	if overview.CDCStatus != uiapi.StateConfigured || overview.CDCMessage != "stub" {
		t.Fatalf("expected configured CDC status, got %+v", overview)
	}
}

func TestGetLogsIncludesExternalAgentLog(t *testing.T) {
	app, configPath, _ := newTempApp(t)
	if err := os.MkdirAll(filepath.Dir(agentLogPath(configPath)), 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	if err := os.WriteFile(agentLogPath(configPath), []byte("agent started\nagent processed event\n"), 0o600); err != nil {
		t.Fatalf("write agent log: %v", err)
	}
	logs := app.GetLogs(uiapi.LogQuery{Module: "sync-agent", Limit: 5})
	if len(logs.Items) == 0 || logs.Items[0].Module != "sync-agent" || !strings.Contains(logs.Items[0].Message, "agent processed") {
		t.Fatalf("expected external agent logs, got %+v", logs)
	}
}

func TestAdminUnlockRequiredForSensitiveMethods(t *testing.T) {
	app, _, _ := newTempApp(t)
	cfg := validConfig()
	app.config = &cfg
	if state := app.GetAuthState(); state.Unlocked {
		t.Fatalf("expected locked auth state, got %+v", state)
	}
	if _, err := app.SaveConfig(uiapi.SaveConfigRequest{Config: uiapi.ConfigFromApp(cfg)}); err == nil {
		t.Fatal("expected SaveConfig to require admin unlock")
	}
	if result := app.SetAutoStart(uiapi.SetAutoStartRequest{Enabled: true}); result.Enabled || result.Status != uiapi.StateLocked {
		t.Fatalf("expected SetAutoStart locked result, got %+v", result)
	}
	if result := app.SetMCPServerEnabled(uiapi.SetMCPServerEnabledRequest{Enabled: true}); result.Enabled || result.Status != uiapi.StateLocked {
		t.Fatalf("expected SetMCPServerEnabled locked result, got %+v", result)
	}
	if _, err := app.RetryFailedEvents(uiapi.RetryFailedEventsRequest{Limit: 10}); err == nil {
		t.Fatal("expected RetryFailedEvents to require admin unlock")
	}
	if _, err := app.GetDeadLetters(uiapi.DeadLetterRequest{Limit: 1}); err == nil {
		t.Fatal("expected GetDeadLetters to require admin unlock")
	}
	if _, err := app.ApplyManagedInstall(uiapi.ManagedInstallRequest{}); err == nil {
		t.Fatal("expected ApplyManagedInstall to require admin unlock")
	}
	if result := app.UnlockAdmin(uiapi.UnlockAdminRequest{Password: "bad"}); result.OK {
		t.Fatalf("expected bad admin password to fail, got %+v", result)
	}
	if result := app.UnlockAdmin(uiapi.UnlockAdminRequest{Password: "admin-pass"}); !result.OK {
		t.Fatalf("expected admin unlock, got %+v", result)
	}
	if state := app.GetAuthState(); !state.Unlocked {
		t.Fatalf("expected unlocked auth state, got %+v", state)
	}
	if result := app.LockAdmin(); !result.OK {
		t.Fatalf("expected lock success, got %+v", result)
	}
	if state := app.GetAuthState(); state.Unlocked {
		t.Fatalf("expected locked auth state after LockAdmin, got %+v", state)
	}
}

func TestManagedInstallPlanAndApply(t *testing.T) {
	app, configPath, _ := newTempApp(t)
	cfg := validConfig()
	cfg.RabbitMQ.LocalURL = ""
	cfg.RabbitMQ.ServerURL = ""
	cfg.CDC.Type = "canal"
	cfg.CDC.Mode = "managed"
	cfg.CDC.Install = true
	cfg.CDC.ConfigDir = filepath.Join(t.TempDir(), "canal")
	app.config = &cfg

	plan, err := app.GetManagedInstallPlan(uiapi.ManagedInstallRequest{})
	if err != nil {
		t.Fatalf("GetManagedInstallPlan returned error: %v", err)
	}
	if plan.Mode != "plan" || len(plan.Operations) == 0 || !strings.HasSuffix(plan.ManifestPath, "install-manifest.json") {
		t.Fatalf("unexpected install plan %+v", plan)
	}

	unlockTestAdmin(t, app)
	applied, err := app.ApplyManagedInstall(uiapi.ManagedInstallRequest{})
	if err != nil {
		t.Fatalf("ApplyManagedInstall returned error: %v", err)
	}
	if applied.Mode != "apply" {
		t.Fatalf("unexpected apply result %+v", applied)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(configPath), "install-manifest.json")); err != nil {
		t.Fatalf("expected manifest next to config: %v", err)
	}
}

func TestMCPServerSwitchDefaultsOffAndPersists(t *testing.T) {
	app, configPath, _ := newTempApp(t)
	cfg := validConfig()
	app.config = &cfg
	if current := app.GetMCPServerStatus(); current.Enabled || current.Status != status.AgentStopped {
		t.Fatalf("expected mcp server disabled by default, got %+v", current)
	}
	unlockTestAdmin(t, app)
	enabled := app.SetMCPServerEnabled(uiapi.SetMCPServerEnabledRequest{Enabled: true})
	if !enabled.Enabled || enabled.Status != uiapi.StateConfigured {
		t.Fatalf("expected mcp server configured status, got %+v", enabled)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "mcp_server:") || !strings.Contains(string(data), "enable: true") {
		t.Fatalf("expected mcp server switch to persist, got:\n%s", string(data))
	}
	disabled := app.SetMCPServerEnabled(uiapi.SetMCPServerEnabledRequest{Enabled: false})
	if disabled.Enabled || disabled.Status != status.AgentStopped {
		t.Fatalf("expected mcp server disabled status, got %+v", disabled)
	}
}

func TestAgentControlStartsStopsAndRestarts(t *testing.T) {
	app, _, _ := newTempApp(t)
	cfg := validConfig()
	app.config = &cfg
	unlockTestAdmin(t, app)

	started := app.StartAgent()
	if !started.OK || started.Status != status.AgentRunning {
		t.Fatalf("expected start success, got %+v", started)
	}
	if overview := app.GetOverview(); overview.AgentStatus != status.AgentRunning {
		t.Fatalf("expected running overview, got %+v", overview)
	}
	startedAgain := app.StartAgent()
	if !startedAgain.OK || startedAgain.Status != status.AgentRunning {
		t.Fatalf("expected idempotent start, got %+v", startedAgain)
	}
	stopped := app.StopAgent()
	if !stopped.OK || stopped.Status != status.AgentStopped {
		t.Fatalf("expected stop success, got %+v", stopped)
	}
	restarted := app.RestartAgent()
	if !restarted.OK || restarted.Status != status.AgentRunning {
		t.Fatalf("expected restart success, got %+v", restarted)
	}
	process := app.GetAgentProcessStatus()
	if process.Status != status.AgentRunning || process.PID == 0 || process.ExecutablePath == "" {
		t.Fatalf("expected running process status, got %+v", process)
	}
	finalStop := app.StopAgent()
	if !finalStop.OK || finalStop.Status != status.AgentStopped {
		t.Fatalf("expected final stop success, got %+v", finalStop)
	}
	process = app.GetAgentProcessStatus()
	if process.Status != status.AgentStopped || process.PID != 0 {
		t.Fatalf("expected stopped process status, got %+v", process)
	}
}

func TestInitialConfigPathFallsBackToExeDirectory(t *testing.T) {
	primary := filepath.Join(t.TempDir(), "missing", "config.yaml")
	exeDir := t.TempDir()
	exeConfig := filepath.Join(exeDir, "config.yaml")
	if err := os.WriteFile(exeConfig, []byte("mode: edge\nnode:\n  id: edge-001\nmysql:\n  database: scada_edge\nsync:\n  retry_interval_seconds: 5\n"), 0o600); err != nil {
		t.Fatalf("write exe config: %v", err)
	}
	oldExecutablePath := executablePath
	t.Setenv("NODEBRIDGE_CONFIG_PATH", "")
	executablePath = func() (string, error) {
		return filepath.Join(exeDir, "DataSync.exe"), nil
	}
	t.Cleanup(func() { executablePath = oldExecutablePath })

	if got := initialConfigPath(primary); got != exeConfig {
		t.Fatalf("expected exe config %q, got %q", exeConfig, got)
	}
}

func TestInitialConfigPathHonorsExplicitOverride(t *testing.T) {
	primary := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("NODEBRIDGE_CONFIG_PATH", primary)
	oldExecutablePath := executablePath
	executablePath = func() (string, error) {
		t.Fatal("executable path must not be called when override is set")
		return "", nil
	}
	t.Cleanup(func() { executablePath = oldExecutablePath })

	if got := initialConfigPath(primary); got != primary {
		t.Fatalf("expected override path %q, got %q", primary, got)
	}
}

func TestAgentExecutableResolverHonorsEnv(t *testing.T) {
	expected := filepath.Join(t.TempDir(), "SyncAgent.exe")
	t.Setenv("NODEBRIDGE_SYNC_AGENT_PATH", expected)
	resolved, err := newExternalAgentController().resolveExecutable()
	if err != nil {
		t.Fatalf("resolve executable: %v", err)
	}
	if resolved != expected {
		t.Fatalf("expected %q, got %q", expected, resolved)
	}
}

func TestAgentExecutableResolverChecksExeDirectory(t *testing.T) {
	dir := t.TempDir()
	expected := filepath.Join(dir, "SyncAgent.exe")
	if err := os.WriteFile(expected, []byte("fake"), 0o600); err != nil {
		t.Fatalf("write fake executable: %v", err)
	}
	oldExecutablePath := executablePath
	t.Setenv("NODEBRIDGE_SYNC_AGENT_PATH", "")
	executablePath = func() (string, error) {
		return filepath.Join(dir, "DataSync.exe"), nil
	}
	t.Cleanup(func() { executablePath = oldExecutablePath })

	resolved, err := newExternalAgentController().resolveExecutable()
	if err != nil {
		t.Fatalf("resolve executable: %v", err)
	}
	if resolved != expected {
		t.Fatalf("expected %q, got %q", expected, resolved)
	}
}

func TestStartAgentRequiresConfig(t *testing.T) {
	app, _, _ := newTempApp(t)
	cfg := validConfig()
	app.config = &cfg
	unlockTestAdmin(t, app)
	app.config = nil
	result := app.StartAgent()
	if result.OK || result.Status != status.AgentError {
		t.Fatalf("expected config error, got %+v", result)
	}
}

func newTempApp(t *testing.T) (*App, string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	rulesPath := filepath.Join(dir, "sync-rules.yaml")
	app := newAppForTest(configPath, rulesPath, fakeProtector{}, newFakeAutoStart())
	app.agent = newFakeAgentController()
	return app, configPath, rulesPath
}

func unlockTestAdmin(t *testing.T, app *App) {
	t.Helper()
	if app.config == nil {
		cfg := validConfig()
		app.config = &cfg
	}
	password := app.config.Security.AdminPassword
	if password == "" {
		password = app.config.Security.ExitPassword
	}
	if result := app.UnlockAdmin(uiapi.UnlockAdminRequest{Password: password}); !result.OK {
		t.Fatalf("unlock admin: %+v", result)
	}
}

func validConfig() appconfig.Config {
	return appconfig.Config{
		Mode: appconfig.ModeEdge,
		Node: appconfig.NodeConfig{ID: "edge-001", Name: "Edge 001"},
		MySQL: appconfig.MySQLConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			Username: "sync",
			Password: "secret",
			Database: "scada_edge",
		},
		RabbitMQ: appconfig.RabbitMQConfig{
			LocalURL:  "amqp://sync:secret@127.0.0.1:5672/edge-sync",
			ServerURL: "amqp://sync:secret@127.0.0.1:5672/server-sync",
			Password:  "secret",
		},
		CDC:      appconfig.CDCConfig{Type: "stub", Password: "secret"},
		Sync:     appconfig.SyncConfig{RetryIntervalSeconds: 5},
		LogWeb:   appconfig.LogWebConfig{Token: "secret"},
		Security: appconfig.SecurityConfig{AdminPassword: "admin-pass", ExitPassword: "exit-pass"},
	}
}

type fakeProtector struct{}

func (fakeProtector) Protect(plain string) (string, error) {
	return "enc-" + base64.StdEncoding.EncodeToString([]byte(plain)), nil
}

func (fakeProtector) Unprotect(cipher string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(cipher, "enc-"))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type fakeAutoStart struct {
	enabled bool
}

func newFakeAutoStart() *fakeAutoStart {
	return &fakeAutoStart{}
}

func (f *fakeAutoStart) Enabled() (bool, error) {
	return f.enabled, nil
}

func (f *fakeAutoStart) SetEnabled(enabled bool) error {
	f.enabled = enabled
	return nil
}

type fakeAgentController struct {
	running bool
	starts  int
	stops   int
	status  uiapi.AgentProcessStatus
}

func newFakeAgentController() *fakeAgentController {
	return &fakeAgentController{}
}

func (f *fakeAgentController) Start(_ context.Context, _, _, _ string) error {
	if f.running {
		return errAgentAlreadyRunning
	}
	f.running = true
	f.starts++
	f.status = uiapi.AgentProcessStatus{ExecutablePath: "fake-sync-agent", PID: 1234, Status: status.AgentRunning, LogPath: "fake.log"}
	return nil
}

func (f *fakeAgentController) Stop(_ context.Context, _ string, _ time.Duration) (string, error) {
	if !f.running {
		return status.AgentStopped, errAgentNotRunning
	}
	f.running = false
	f.stops++
	f.status.PID = 0
	f.status.Status = status.AgentStopped
	return status.AgentStopped, nil
}

func (f *fakeAgentController) Running() bool {
	return f.running
}

func (f *fakeAgentController) Status() uiapi.AgentProcessStatus {
	if f.status.Status == "" {
		return uiapi.AgentProcessStatus{Status: status.AgentStopped}
	}
	return f.status
}
