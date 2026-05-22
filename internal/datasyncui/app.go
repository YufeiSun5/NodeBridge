package datasyncui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/autostart"
	"github.com/YufeiSun5/NodeBridge/internal/diagnostic"
	installerexec "github.com/YufeiSun5/NodeBridge/internal/installer/executor"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/status"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const appVersion = "0.33.0"

// App binds Wails UI. / UI 入口。 / UI入口。
type App struct {
	config     *appconfig.Config
	ruleSet    *rules.RuleSet
	configPath string
	rulesPath  string
	runtime    *status.RuntimeStore
	autoStart  autostart.Manager
	protector  appconfig.SecretProtector
	agent      agentController
	ctx        context.Context
	tray       *nativeTray
	exitArmed  atomic.Bool
	auth       adminAuth
}

func NewApp() *App {
	path := appconfig.DefaultConfigPath()
	path = initialConfigPath(path)
	app := &App{
		configPath: path,
		rulesPath:  filepath.Join(filepath.Dir(path), "sync-rules.yaml"),
		runtime:    status.NewRuntimeStore(),
		autoStart:  autostart.NewCurrentUserManager(),
		protector:  appconfig.DefaultSecretProtector(),
		agent:      newExternalAgentController(),
	}
	if cfg, err := appconfig.LoadFile(path); err == nil {
		app.config = cfg
	}
	app.auth.Timeout = 10 * time.Minute
	return app
}

var executablePath = os.Executable

func initialConfigPath(primary string) string {
	if strings.TrimSpace(os.Getenv("NODEBRIDGE_CONFIG_PATH")) != "" {
		return primary
	}
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	exe, err := executablePath()
	if err != nil {
		return primary
	}
	candidate := filepath.Join(filepath.Dir(exe), "config.yaml")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return primary
}

func newAppForTest(configPath, rulesPath string, protector appconfig.SecretProtector, autoStart autostart.Manager) *App {
	return &App{
		configPath: configPath,
		rulesPath:  rulesPath,
		runtime:    status.NewRuntimeStore(),
		autoStart:  autoStart,
		protector:  protector,
		auth:       adminAuth{Timeout: 10 * time.Minute},
	}
}

func Run() {
	RunWithAssets(os.DirFS("frontend/dist"))
}

func RunWithAssets(assets fs.FS) {
	app := NewApp()
	if err := wails.Run(&options.App{
		Title:             "NodeBridge",
		Width:             1100,
		Height:            720,
		HideWindowOnClose: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind:          []interface{}{app},
	}); err != nil {
		log.Fatal(err)
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	tray, err := startNativeTray(nativeTrayCallbacks{
		Show:        func() { a.showWindow() },
		RequestExit: func() { a.requestExitFromTray() },
	})
	if err == nil {
		a.tray = tray
		log.Print("native tray ready")
	} else {
		log.Printf("native tray unavailable: %v", err)
	}
}

func (a *App) shutdown(ctx context.Context) {
	if a.tray != nil {
		a.tray.Stop()
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.consumeExitRequest() {
		return false
	}
	wailsruntime.WindowHide(ctx)
	return true
}

func (a *App) showWindow() {
	if a.ctx == nil {
		return
	}
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
}

func (a *App) requestExitFromTray() {
	if a.ctx == nil {
		return
	}
	a.showWindow()
	wailsruntime.EventsEmit(a.ctx, "datasync:request-exit")
}

func (a *App) GetOverview() status.Overview {
	overview := status.Overview{
		ProductName:    "NodeBridge",
		Mode:           appconfig.ModeUnknown,
		ConfigPath:     a.effectiveConfigPath(),
		RulesPath:      a.effectiveRulesPath(),
		AgentStatus:    status.AgentStopped,
		AgentLogPath:   agentLogPath(a.effectiveConfigPath()),
		MySQLStatus:    uiapi.StateUnknown,
		RabbitMQStatus: uiapi.StateUnknown,
		CDCStatus:      uiapi.StateUnknown,
		Version:        appVersion,
	}
	if a.config != nil {
		overview.Mode = a.config.Mode
		overview.NodeID = a.config.Node.ID
		overview.NodeName = a.config.Node.Name
		overview.ConfigLoaded = true
		overview.MySQLStatus = a.probeMySQLStatus()
		overview.RabbitMQStatus = a.probeRabbitMQStatus()
		overview.CDCStatus, overview.CDCMessage = a.probeCDCStatus()
	}
	if a.agent != nil && a.agent.Running() {
		overview.AgentStatus = status.AgentRunning
	}
	if agentState := a.GetAgentProcessStatus(); agentState.Status != "" {
		overview.AgentStatus = agentState.Status
		overview.AgentPID = agentState.PID
		if agentState.LogPath != "" {
			overview.AgentLogPath = agentState.LogPath
		}
	}
	for _, worker := range a.runtime.Snapshot().Workers {
		if worker.State == status.WorkerRunning {
			overview.AgentStatus = status.AgentRunning
			break
		}
		if worker.State == status.WorkerError {
			overview.AgentStatus = status.AgentError
		}
	}
	for _, queue := range a.GetQueueStatus().Queues {
		switch queue.Role {
		case "local_upload", "server_ingress":
			overview.UploadQueueDepth += int64(queue.Messages)
		case "downlink":
			overview.DownlinkQueueDepth += int64(queue.Messages)
		}
	}
	if count, err := a.failedEventCount(context.Background()); err == nil {
		overview.FailedEventCount = count
	}
	return uiapi.RedactOverview(overview)
}

func (a *App) GetConfig() uiapi.ConfigDTO {
	if a.config == nil {
		return uiapi.RedactConfig(uiapi.ConfigFromApp(appconfig.Config{}))
	}
	return uiapi.RedactConfig(uiapi.ConfigFromApp(*a.config))
}

func (a *App) SaveConfig(req uiapi.SaveConfigRequest) (uiapi.ConfigDTO, error) {
	if err := a.requireAdmin(); err != nil {
		return uiapi.ConfigDTO{}, err
	}
	cfg := req.Config.ToAppConfig()
	if a.config != nil {
		cfg = appconfig.MergeRedactedSecrets(cfg, *a.config)
	}
	if err := cfg.Validate(); err != nil {
		return uiapi.ConfigDTO{}, err
	}
	if strings.TrimSpace(cfg.Security.AdminPassword) == "" {
		return uiapi.ConfigDTO{}, errors.New("security.admin_password is required")
	}
	path := a.configPath
	if path == "" {
		path = appconfig.DefaultConfigPath()
	}
	if err := appconfig.SaveFileWithProtector(path, cfg, a.protectorOrDefault()); err != nil {
		return uiapi.ConfigDTO{}, err
	}
	a.config = &cfg
	return uiapi.RedactConfig(uiapi.ConfigFromApp(cfg)), nil
}

func (a *App) TestMySQL(req appconfig.MySQLConfig) uiapi.TestResult {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	db, err := mysqlconn.Open(req)
	if err != nil {
		return uiapi.TestResult{OK: false, Status: status.AgentError, Message: err.Error()}
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return uiapi.TestResult{OK: false, Status: status.AgentError, Message: err.Error()}
	}
	return uiapi.TestResult{OK: true, Status: status.AgentRunning, Message: "mysql reachable"}
}

func (a *App) TestRabbitMQ(req appconfig.RabbitMQConfig) uiapi.TestResult {
	target := req.ServerURL
	if target == "" {
		target = req.LocalURL
	}
	if target == "" {
		return uiapi.TestResult{OK: false, Status: status.AgentError, Message: "rabbitmq url is required"}
	}
	conn, err := rabbitmq.Dial(target)
	if err != nil {
		return uiapi.TestResult{OK: false, Status: status.AgentError, Message: err.Error()}
	}
	defer conn.Close()
	return uiapi.TestResult{OK: true, Status: status.AgentRunning, Message: "rabbitmq reachable"}
}

func (a *App) GetSyncRules() (uiapi.SyncRulesDTO, error) {
	if a.ruleSet != nil {
		return uiapi.SyncRulesDTO{Rules: append([]rules.SyncRule(nil), a.ruleSet.Rules...)}, nil
	}
	path := a.rulesPath
	if path == "" {
		path = "configs/sync-rules.example.yaml"
	}
	set, err := rules.LoadFile(path)
	if err != nil {
		set, err = loadBundledRules()
		if err != nil {
			set = rules.DefaultFieldRuleSet()
		}
		_ = rules.SaveFile(a.effectiveRulesPath(), *set)
	}
	a.ruleSet = set
	return uiapi.SyncRulesDTO{Rules: append([]rules.SyncRule(nil), set.Rules...)}, nil
}

func (a *App) SaveSyncRules(req uiapi.SaveSyncRulesRequest) (uiapi.SyncRulesDTO, error) {
	if err := a.requireAdmin(); err != nil {
		return uiapi.SyncRulesDTO{}, err
	}
	set := rules.RuleSet{Rules: append([]rules.SyncRule(nil), req.Rules...)}
	if err := set.Validate(); err != nil {
		return uiapi.SyncRulesDTO{}, err
	}
	if a.rulesPath != "" {
		if err := rules.SaveFile(a.rulesPath, set); err != nil {
			return uiapi.SyncRulesDTO{}, err
		}
	}
	a.ruleSet = &set
	return uiapi.SyncRulesDTO{Rules: append([]rules.SyncRule(nil), req.Rules...)}, nil
}

func (a *App) GetQueueStatus() uiapi.QueueStatusResponse {
	queues := a.queuePlaceholders()
	if a.config == nil {
		return uiapi.QueueStatusResponse{Queues: queues}
	}
	url := a.config.RabbitMQ.ServerURL
	topology := rabbitmq.ServerTopology(nil)
	if a.config.Mode == appconfig.ModeEdge {
		url = a.config.RabbitMQ.LocalURL
		topology = rabbitmq.EdgeTopology()
	}
	if strings.TrimSpace(url) == "" {
		return uiapi.QueueStatusResponse{Queues: queues}
	}
	conn, err := rabbitmq.Dial(url)
	if err != nil {
		markQueues(queues, status.AgentError)
		return uiapi.QueueStatusResponse{Queues: queues}
	}
	defer conn.Close()
	byName := map[string]int{}
	for i := range queues {
		byName[queues[i].Name] = i
	}
	for _, queue := range topology.Queues {
		inspected, err := rabbitmq.InspectQueue(conn.Channel, queue)
		if idx, ok := byName[queue.Name]; ok {
			if err != nil {
				queues[idx].Status = status.AgentError
				continue
			}
			queues[idx].Messages = inspected.Messages
			queues[idx].Consumers = inspected.Consumers
			queues[idx].Status = status.AgentRunning
		}
	}
	return uiapi.QueueStatusResponse{Queues: queues}
}

func (a *App) GetFailedEvents(req uiapi.FailedEventsRequest) (uiapi.FailedEventsResponse, error) {
	store, closeFn, err := a.openSyncStore()
	if err != nil {
		if errors.Is(err, errConfigMissing) {
			return uiapi.FailedEventsResponse{Items: []uiapi.FailedEventDTO{}}, nil
		}
		return uiapi.FailedEventsResponse{}, err
	}
	defer closeFn()
	events, err := store.ListFailedEvents(context.Background(), req.Limit)
	if err != nil {
		return uiapi.FailedEventsResponse{}, err
	}
	items := make([]uiapi.FailedEventDTO, 0, len(events))
	for _, event := range events {
		items = append(items, uiapi.FailedEventDTO{
			EventID:      event.EventID,
			TargetNodeID: event.TargetNodeID,
			Status:       event.Status,
			ErrorMessage: event.ErrorMessage,
			CreatedAt:    uiapi.TimeString(event.CreatedAt),
		})
	}
	return uiapi.FailedEventsResponse{Items: items}, nil
}

func (a *App) RetryFailedEvent(req uiapi.RetryFailedEventRequest) (uiapi.OperationResult, error) {
	if err := a.requireAdmin(); err != nil {
		return uiapi.OperationResult{}, err
	}
	if req.EventID == "" || req.TargetNodeID == "" {
		return uiapi.OperationResult{}, errors.New("event_id and target_node_id are required")
	}
	store, closeFn, err := a.openSyncStore()
	if err != nil {
		return uiapi.OperationResult{}, err
	}
	defer closeFn()
	if err := store.MarkRetryPending(context.Background(), req.EventID, req.TargetNodeID); err != nil {
		return uiapi.OperationResult{}, err
	}
	return uiapi.OperationResult{OK: true, Status: syncstore.StatusPending, Message: "retry marked pending"}, nil
}

func (a *App) RetryFailedEvents(req uiapi.RetryFailedEventsRequest) (uiapi.OperationResult, error) {
	if err := a.requireAdmin(); err != nil {
		return uiapi.OperationResult{}, err
	}
	store, closeFn, err := a.openSyncStore()
	if err != nil {
		return uiapi.OperationResult{}, err
	}
	defer closeFn()
	count, err := store.MarkFailedEventsPending(context.Background(), req.Limit)
	if err != nil {
		return uiapi.OperationResult{}, err
	}
	return uiapi.OperationResult{OK: true, Status: syncstore.StatusPending, Message: fmt.Sprintf("retry marked pending count=%d", count)}, nil
}

func (a *App) GetDeadLetters(req uiapi.DeadLetterRequest) (uiapi.DeadLetterResponse, error) {
	if err := a.requireAdmin(); err != nil {
		return uiapi.DeadLetterResponse{}, err
	}
	if a.config == nil {
		return uiapi.DeadLetterResponse{Items: []uiapi.DeadLetterMessageDTO{}}, nil
	}
	queueName := strings.TrimSpace(req.Queue)
	if queueName == "" {
		queueName = defaultDeadLetterQueue(a.config.Mode)
	}
	url := a.config.RabbitMQ.ServerURL
	if a.config.Mode == appconfig.ModeEdge {
		url = a.config.RabbitMQ.LocalURL
	}
	if strings.TrimSpace(url) == "" {
		return uiapi.DeadLetterResponse{}, errors.New("rabbitmq url is required")
	}
	conn, err := rabbitmq.Dial(url)
	if err != nil {
		return uiapi.DeadLetterResponse{}, err
	}
	defer conn.Close()
	messages, err := rabbitmq.PeekMessages(context.Background(), conn.Channel, queueName, req.Limit)
	if err != nil {
		return uiapi.DeadLetterResponse{}, err
	}
	items := make([]uiapi.DeadLetterMessageDTO, 0, len(messages))
	for _, message := range messages {
		items = append(items, uiapi.DeadLetterMessageDTO{
			Queue:       queueName,
			ContentType: message.ContentType,
			BodyPreview: bodyPreview(message.Body, 1000),
			BodySize:    len(message.Body),
			Headers:     headerStrings(message.Headers),
		})
	}
	return uiapi.DeadLetterResponse{Items: items}, nil
}

func (a *App) GetLogs(req uiapi.LogQuery) uiapi.LogsResponse {
	snapshot := a.runtime.Snapshot()
	limit := req.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	items := make([]uiapi.LogEntry, 0, len(snapshot.Logs))
	for i := len(snapshot.Logs) - 1; i >= 0 && len(items) < limit; i-- {
		entry := snapshot.Logs[i]
		if req.Level != "" && !strings.EqualFold(entry.Level, req.Level) {
			continue
		}
		if req.Module != "" && entry.Worker != req.Module {
			continue
		}
		items = append(items, uiapi.LogEntry{
			Time:    uiapi.TimeString(entry.Time),
			Level:   entry.Level,
			Module:  entry.Worker,
			Message: entry.Message,
		})
	}
	items = append(items, a.agentLogEntries(req, limit-len(items))...)
	return uiapi.LogsResponse{Items: items}
}

func (a *App) VerifyExitPassword(req uiapi.VerifyExitPasswordRequest) uiapi.OperationResult {
	if a.config == nil || strings.TrimSpace(a.config.Security.ExitPassword) == "" {
		return uiapi.OperationResult{OK: true, Status: status.AgentRunning, Message: "exit password is not configured"}
	}
	if req.Password != a.config.Security.ExitPassword {
		return uiapi.OperationResult{OK: false, Status: status.AgentError, Message: "exit password mismatch"}
	}
	return uiapi.OperationResult{OK: true, Status: status.AgentRunning, Message: "exit password verified"}
}

func (a *App) UnlockAdmin(req uiapi.UnlockAdminRequest) uiapi.OperationResult {
	if !a.adminPasswordMatches(req.Password) {
		return uiapi.OperationResult{OK: false, Status: uiapi.StateLocked, Message: "admin password mismatch"}
	}
	a.auth.Until = time.Now().Add(a.adminTimeout())
	return uiapi.OperationResult{OK: true, Status: uiapi.StateUnlocked, Message: "admin unlocked"}
}

func (a *App) LockAdmin() uiapi.OperationResult {
	a.auth.Until = time.Time{}
	return uiapi.OperationResult{OK: true, Status: uiapi.StateLocked, Message: "admin locked"}
}

func (a *App) GetAuthState() uiapi.AuthState {
	timeout := a.adminTimeout()
	unlocked := time.Now().Before(a.auth.Until)
	state := uiapi.AuthState{
		Unlocked:       unlocked,
		TimeoutSeconds: int(timeout.Seconds()),
	}
	if unlocked {
		state.Status = uiapi.StateUnlocked
		state.ExpiresAt = uiapi.TimeString(a.auth.Until)
		return state
	}
	state.Status = uiapi.StateLocked
	return state
}

func (a *App) RequestExit(req uiapi.VerifyExitPasswordRequest) uiapi.OperationResult {
	result := a.VerifyExitPassword(req)
	if !result.OK {
		return result
	}
	a.exitArmed.Store(true)
	return uiapi.OperationResult{OK: true, Status: status.AgentRunning, Message: "exit requested"}
}

func (a *App) consumeExitRequest() bool {
	return a.exitArmed.Swap(false)
}

func (a *App) GetAutoStart() uiapi.AutoStartStatus {
	if a.autoStart == nil {
		return uiapi.AutoStartStatus{Enabled: false, Status: uiapi.StateUnsupported, Message: "autostart manager is not configured"}
	}
	enabled, err := a.autoStart.Enabled()
	if err != nil {
		return uiapi.AutoStartStatus{Enabled: false, Status: status.AgentError, Message: err.Error()}
	}
	return uiapi.AutoStartStatus{Enabled: enabled, Status: status.AgentRunning}
}

func (a *App) SetAutoStart(req uiapi.SetAutoStartRequest) uiapi.AutoStartStatus {
	if err := a.requireAdmin(); err != nil {
		return uiapi.AutoStartStatus{Enabled: false, Status: uiapi.StateLocked, Message: err.Error()}
	}
	if a.autoStart == nil {
		return uiapi.AutoStartStatus{Enabled: false, Status: uiapi.StateUnsupported, Message: "autostart manager is not configured"}
	}
	if err := a.autoStart.SetEnabled(req.Enabled); err != nil {
		return uiapi.AutoStartStatus{Enabled: false, Status: status.AgentError, Message: err.Error()}
	}
	return uiapi.AutoStartStatus{Enabled: req.Enabled, Status: status.AgentRunning}
}

func (a *App) GetMCPServerStatus() uiapi.MCPServerStatus {
	if a.config == nil {
		return uiapi.MCPServerStatus{Enabled: false, Status: uiapi.StateUnknown, Message: "config is not loaded"}
	}
	if !a.config.MCP.Enable {
		return uiapi.MCPServerStatus{Enabled: false, Status: status.AgentStopped, Message: "mcp server is disabled"}
	}
	return uiapi.MCPServerStatus{
		Enabled: true,
		Status:  uiapi.StateConfigured,
		Message: "mcp server switch is enabled; runtime server is reserved for a later version",
	}
}

func (a *App) SetMCPServerEnabled(req uiapi.SetMCPServerEnabledRequest) uiapi.MCPServerStatus {
	if err := a.requireAdmin(); err != nil {
		return uiapi.MCPServerStatus{Enabled: false, Status: uiapi.StateLocked, Message: err.Error()}
	}
	if a.config == nil {
		return uiapi.MCPServerStatus{Enabled: false, Status: status.AgentError, Message: "config is not loaded"}
	}
	a.config.MCP.Enable = req.Enabled
	if err := appconfig.SaveFileWithProtector(a.effectiveConfigPath(), *a.config, a.protectorOrDefault()); err != nil {
		return uiapi.MCPServerStatus{Enabled: a.config.MCP.Enable, Status: status.AgentError, Message: err.Error()}
	}
	return a.GetMCPServerStatus()
}

func (a *App) GetManagedInstallPlan(req uiapi.ManagedInstallRequest) (uiapi.ManagedInstallResponse, error) {
	if a.config == nil {
		return uiapi.ManagedInstallResponse{}, errors.New("config is not loaded")
	}
	plan := installerexec.New().Plan(installerexec.Request{
		Config:       *a.config,
		ConfigPath:   a.effectiveConfigPath(),
		ManifestPath: a.manifestPath(req.ManifestPath),
		Version:      appVersion,
		InstallID:    "nodebridge-local",
	})
	return managedInstallResponse(plan), nil
}

func (a *App) ApplyManagedInstall(req uiapi.ManagedInstallRequest) (uiapi.ManagedInstallResponse, error) {
	if err := a.requireAdmin(); err != nil {
		return uiapi.ManagedInstallResponse{}, err
	}
	if a.config == nil {
		return uiapi.ManagedInstallResponse{}, errors.New("config is not loaded")
	}
	result, err := installerexec.New().Apply(context.Background(), installerexec.Request{
		Config:       *a.config,
		ConfigPath:   a.effectiveConfigPath(),
		ManifestPath: a.manifestPath(req.ManifestPath),
		Version:      appVersion,
		InstallID:    "nodebridge-local",
	})
	return managedInstallResponse(result), err
}

func (a *App) GetAgentProcessStatus() uiapi.AgentProcessStatus {
	result := uiapi.AgentProcessStatus{
		Status:  status.AgentStopped,
		LogPath: agentLogPath(a.effectiveConfigPath()),
	}
	if a.agent == nil {
		result.Status = uiapi.StateUnsupported
		result.LastError = "agent controller is not configured"
		return result
	}
	state := a.agent.Status()
	if state.Status == "" {
		state.Status = result.Status
	}
	if state.LogPath == "" {
		state.LogPath = result.LogPath
	}
	return state
}

func (a *App) manifestPath(override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	return uiManagedManifestPath(a.effectiveConfigPath())
}

func managedInstallResponse(result installerexec.Result) uiapi.ManagedInstallResponse {
	items := make([]uiapi.ManagedInstallOperationDTO, 0, len(result.Operations))
	for _, operation := range result.Operations {
		items = append(items, uiapi.ManagedInstallOperationDTO{
			Component: operation.Component,
			Action:    operation.Action,
			Target:    operation.Target,
			Status:    operation.Status,
			Message:   operation.Message,
		})
	}
	return uiapi.ManagedInstallResponse{
		Mode:         result.Mode,
		ManifestPath: result.ManifestPath,
		Operations:   items,
	}
}

func (a *App) ExportDiagnosticPackage() (uiapi.DiagnosticPackageResponse, error) {
	if err := a.requireAdmin(); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	nodeID := "local"
	if a.config != nil && a.config.Node.ID != "" {
		nodeID = a.config.Node.ID
	}
	var files []diagnostic.File
	addJSON := func(name string, value any) error {
		file, err := diagnostic.JSONFile(name, value)
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	}
	if err := addJSON("config.redacted.json", a.GetConfig()); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	rulesDTO, _ := a.GetSyncRules()
	if err := addJSON("sync-rules.json", rulesDTO); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	if err := addJSON("overview.json", a.GetOverview()); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	if err := addJSON("queues.json", a.GetQueueStatus()); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	failures, _ := a.GetFailedEvents(uiapi.FailedEventsRequest{Limit: 100})
	if err := addJSON("failed-events.json", failures); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	if err := addJSON("logs.json", a.GetLogs(uiapi.LogQuery{Limit: 200})); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	if err := addJSON("agent-process.json", a.GetAgentProcessStatus()); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	if err := addJSON("mcp-status.json", a.GetMCPServerStatus()); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	if a.config != nil {
		plan := installerexec.New().Plan(installerexec.Request{
			Config:       *a.config,
			ConfigPath:   a.effectiveConfigPath(),
			ManifestPath: uiManagedManifestPath(a.effectiveConfigPath()),
			Version:      appVersion,
			InstallID:    "nodebridge-local",
		})
		if err := addJSON("managed-install-plan.json", plan); err != nil {
			return uiapi.DiagnosticPackageResponse{}, err
		}
	}
	if data, err := os.ReadFile(agentLogPath(a.effectiveConfigPath())); err == nil {
		files = append(files, diagnostic.File{Name: "agent-log.txt", Data: data})
	}
	addedDiagnosticNames := map[string]bool{}
	for _, candidate := range stressSummaryCandidates(a.effectiveConfigPath()) {
		if addedDiagnosticNames[candidate.name] {
			continue
		}
		if data, err := os.ReadFile(candidate.path); err == nil {
			files = append(files, diagnostic.File{Name: candidate.name, Data: data})
			addedDiagnosticNames[candidate.name] = true
		}
	}
	meta := map[string]any{"version": appVersion, "created_at": time.Now().Format(time.RFC3339)}
	if err := addJSON("version.json", meta); err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	dir := filepath.Join(os.TempDir(), "NodeBridge", "diagnostics")
	path, err := diagnostic.CreatePackage(dir, nodeID, files)
	if err != nil {
		return uiapi.DiagnosticPackageResponse{}, err
	}
	return uiapi.DiagnosticPackageResponse{Path: path}, nil
}

func uiManagedManifestPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "install-manifest.json")
}

func (a *App) StartAgent() uiapi.OperationResult {
	if err := a.requireAdmin(); err != nil {
		return uiapi.OperationResult{OK: false, Status: uiapi.StateLocked, Message: err.Error()}
	}
	if a.config == nil {
		return uiapi.OperationResult{OK: false, Status: status.AgentError, Message: "config is not loaded"}
	}
	if a.agent == nil {
		return unsupported("agent controller is not configured")
	}
	if err := a.ensureRulesFile(); err != nil {
		return uiapi.OperationResult{OK: false, Status: status.AgentError, Message: err.Error()}
	}
	if err := a.agent.Start(context.Background(), a.effectiveConfigPath(), a.effectiveRulesPath(), agentStopFilePath(a.effectiveConfigPath())); err != nil {
		if errors.Is(err, errAgentAlreadyRunning) {
			return uiapi.OperationResult{OK: true, Status: status.AgentRunning, Message: "agent already running"}
		}
		return uiapi.OperationResult{OK: false, Status: status.AgentError, Message: err.Error()}
	}
	a.runtime.RecordProcessed("agent-control", "", "start", 0)
	return uiapi.OperationResult{OK: true, Status: status.AgentRunning, Message: "agent started"}
}

func (a *App) StopAgent() uiapi.OperationResult {
	if err := a.requireAdmin(); err != nil {
		return uiapi.OperationResult{OK: false, Status: uiapi.StateLocked, Message: err.Error()}
	}
	if a.agent == nil {
		return unsupported("agent controller is not configured")
	}
	stopStatus, err := a.agent.Stop(context.Background(), agentStopFilePath(a.effectiveConfigPath()), 10*time.Second)
	if err != nil {
		if errors.Is(err, errAgentNotRunning) {
			return uiapi.OperationResult{OK: true, Status: status.AgentStopped, Message: "agent is not running"}
		}
		return uiapi.OperationResult{OK: false, Status: status.AgentError, Message: err.Error()}
	}
	a.runtime.RecordStopped("agent-control")
	return uiapi.OperationResult{OK: true, Status: stopStatus, Message: "agent stopped"}
}

func (a *App) RestartAgent() uiapi.OperationResult {
	if err := a.requireAdmin(); err != nil {
		return uiapi.OperationResult{OK: false, Status: uiapi.StateLocked, Message: err.Error()}
	}
	stopResult := a.StopAgent()
	if !stopResult.OK && stopResult.Status != status.AgentStopped {
		return stopResult
	}
	return a.StartAgent()
}

func (a *App) LoadConfig(path string) (*appconfig.Config, error) {
	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		return nil, err
	}
	a.config = cfg
	return cfg, nil
}

func (a *App) probeMySQLStatus() string {
	if a.config == nil {
		return uiapi.StateUnknown
	}
	db, err := mysqlconn.Open(a.config.MySQL)
	if err != nil {
		return status.AgentError
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return status.AgentError
	}
	return status.AgentRunning
}

func (a *App) probeRabbitMQStatus() string {
	if a.config == nil {
		return uiapi.StateUnknown
	}
	url := a.config.RabbitMQ.ServerURL
	if a.config.Mode == appconfig.ModeEdge && a.config.RabbitMQ.LocalURL != "" {
		url = a.config.RabbitMQ.LocalURL
	}
	if url == "" {
		return uiapi.StateUnknown
	}
	conn, err := rabbitmq.Dial(url)
	if err != nil {
		return status.AgentError
	}
	defer conn.Close()
	return status.AgentRunning
}

func (a *App) probeCDCStatus() (string, string) {
	if a.config == nil {
		return uiapi.StateUnknown, "config is not loaded"
	}
	cdcType := strings.TrimSpace(a.config.CDC.Type)
	if cdcType == "" {
		return uiapi.StateUnknown, "cdc.type is not configured"
	}
	if a.agent != nil && a.agent.Running() {
		return status.AgentRunning, cdcType
	}
	switch cdcType {
	case "stub", "canal":
		return uiapi.StateConfigured, cdcType
	default:
		return status.AgentError, "unsupported cdc.type: " + cdcType
	}
}

var errConfigMissing = errors.New("config is not loaded")

func (a *App) openSyncStore() (*syncstore.Store, func(), error) {
	if a.config == nil {
		return nil, func() {}, errConfigMissing
	}
	db, err := mysqlconn.Open(a.config.MySQL)
	if err != nil {
		return nil, func() {}, err
	}
	return syncstore.New(db), func() { _ = db.Close() }, nil
}

func (a *App) failedEventCount(ctx context.Context) (int64, error) {
	store, closeFn, err := a.openSyncStore()
	if err != nil {
		return 0, err
	}
	defer closeFn()
	return store.CountFailedEvents(ctx)
}

func (a *App) agentLogEntries(req uiapi.LogQuery, limit int) []uiapi.LogEntry {
	if limit <= 0 {
		return nil
	}
	data, err := os.ReadFile(agentLogPath(a.effectiveConfigPath()))
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	items := make([]uiapi.LogEntry, 0, limit)
	for i := len(lines) - 1; i >= 0 && len(items) < limit; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if req.Level != "" && !strings.EqualFold(req.Level, "info") {
			continue
		}
		if req.Module != "" && req.Module != "sync-agent" {
			continue
		}
		items = append(items, uiapi.LogEntry{
			Time:    uiapi.TimeString(time.Now()),
			Level:   "INFO",
			Module:  "sync-agent",
			Message: line,
		})
	}
	return items
}

func (a *App) queuePlaceholders() []uiapi.QueueStatusDTO {
	if a.config != nil && a.config.Mode == appconfig.ModeServer {
		return []uiapi.QueueStatusDTO{
			{Name: "server.cdc.ingress.q", Role: "server_ingress", Status: uiapi.StateUnknown},
			{Name: "server.dead.q", Role: "dead_letter", Status: uiapi.StateUnknown},
		}
	}
	return []uiapi.QueueStatusDTO{
		{Name: "edge.upload.cdc.q", Role: "local_upload", Status: uiapi.StateUnknown},
		{Name: "edge.upload.retry.q", Role: "retry", Status: uiapi.StateUnknown},
		{Name: "edge.downlink.q", Role: "downlink", Status: uiapi.StateUnknown},
		{Name: "edge.dead.q", Role: "dead_letter", Status: uiapi.StateUnknown},
	}
}

func markQueues(queues []uiapi.QueueStatusDTO, state string) {
	for i := range queues {
		queues[i].Status = state
	}
}

func defaultDeadLetterQueue(mode string) string {
	if mode == appconfig.ModeServer {
		return "server.dead.q"
	}
	return "edge.dead.q"
}

func bodyPreview(body []byte, limit int) string {
	if limit <= 0 {
		limit = 1000
	}
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit])
}

func headerStrings(headers map[string]any) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string, len(headers))
	for key, value := range headers {
		result[key] = fmt.Sprint(value)
	}
	return result
}

func (a *App) protectorOrDefault() appconfig.SecretProtector {
	if a.protector != nil {
		return a.protector
	}
	return appconfig.DefaultSecretProtector()
}

func (a *App) effectiveConfigPath() string {
	if a.configPath != "" {
		return a.configPath
	}
	return appconfig.DefaultConfigPath()
}

func (a *App) effectiveRulesPath() string {
	if a.rulesPath != "" {
		return a.rulesPath
	}
	return filepath.Join(filepath.Dir(a.effectiveConfigPath()), "sync-rules.yaml")
}

func stressSummaryCandidates(configPath string) []struct {
	name string
	path string
} {
	configDir := filepath.Dir(configPath)
	return []struct {
		name string
		path string
	}{
		{name: "stress-summary.json", path: filepath.Join(configDir, "stress-summary.json")},
		{name: "stress-summary.json", path: filepath.Join("build", "lab-stress-summary.json")},
		{name: "stress-summary-lab11.json", path: filepath.Join("build", "lab-11-stress-summary.json")},
		{name: "soak-summary-lab11.json", path: filepath.Join("build", "lab-11-soak-summary.json")},
		{name: "disconnect-summary-lab11.json", path: filepath.Join("build", "lab-11-disconnect-summary.json")},
	}
}

func loadBundledRules() (*rules.RuleSet, error) {
	for _, path := range []string{"configs/sync-rules.10-edge.example.yaml", "configs/sync-rules.example.yaml"} {
		set, err := rules.LoadFile(path)
		if err == nil {
			return set, nil
		}
	}
	return rules.DefaultFieldRuleSet(), nil
}

func (a *App) ensureRulesFile() error {
	path := a.effectiveRulesPath()
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dto, err := a.GetSyncRules()
	if err != nil {
		return err
	}
	return rules.SaveFile(path, rules.RuleSet{Rules: dto.Rules})
}

func (a *App) requireAdmin() error {
	if a.GetAuthState().Unlocked {
		return nil
	}
	return errors.New("admin unlock required")
}

func (a *App) adminPasswordMatches(password string) bool {
	if a.config == nil {
		return true
	}
	adminPassword := strings.TrimSpace(a.config.Security.AdminPassword)
	if adminPassword == "" {
		// Backward compatible. / 向后兼容。 / 互換維持。
		adminPassword = strings.TrimSpace(a.config.Security.ExitPassword)
	}
	if adminPassword == "" {
		return true
	}
	return password == adminPassword
}

func (a *App) adminTimeout() time.Duration {
	if a.auth.Timeout > 0 {
		return a.auth.Timeout
	}
	return 10 * time.Minute
}

func unsupported(message string) uiapi.OperationResult {
	return uiapi.OperationResult{OK: false, Status: uiapi.StateUnsupported, Message: fmt.Sprintf("%s", message)}
}

type adminAuth struct {
	Until   time.Time
	Timeout time.Duration
}
