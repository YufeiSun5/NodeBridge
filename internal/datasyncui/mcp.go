package datasyncui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/autostart"
	"github.com/YufeiSun5/NodeBridge/internal/eventstatus"
	rabbitadmin "github.com/YufeiSun5/NodeBridge/internal/installer/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/mcpstdio"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/status"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

// MCPService reuses management operations without creating a Wails window or tray.
// Serve serializes requests; each operation reloads configuration from disk.
type MCPService struct {
	mcpstdio.StaticService
	app            *App
	extra          []mcpTool
	requestLock    *os.File
	rabbitMQAdmin  managedRabbitMQAdmin
	defaultLogPath bool
	bootstrap      bool
}

type managedRabbitMQAdmin interface {
	EnsureForNode(context.Context, appconfig.Config) error
	EnsureServerEdgeUser(context.Context, string) error
}

type mcpTool struct {
	name, description string
	schema            map[string]any
	write             bool
	call              func(context.Context, json.RawMessage) (any, error)
}

type ensureServerEdgeUserRequest struct {
	NodeID string `json:"node_id"`
}

func NewMCPService(configPath, rulesPath, logPath string, lab bool) (*MCPService, error) {
	app := &App{
		configPath: configPath, rulesPath: rulesPath,
		runtime: status.NewRuntimeStore(), protector: appconfig.DefaultSecretProtector(),
		autoStart: autostart.NewCurrentUserManager(), agent: newExternalAgentController(),
		auth: adminAuth{Timeout: defaultAdminTimeout}, mcpLabFullAccess: true,
	}
	defaultLogPath := logPath == ""
	if defaultLogPath {
		logPath = agentLogPath(configPath)
	}
	app.agent.(*externalAgentController).configPath = configPath
	admin := rabbitadmin.NewAdmin()
	s := &MCPService{StaticService: mcpstdio.StaticService{ConfigPath: configPath, RulesPath: rulesPath, LogPath: logPath, LabFullAccess: true}, app: app, rabbitMQAdmin: admin, defaultLogPath: defaultLogPath, bootstrap: lab}
	if err := s.refresh(); err != nil {
		return nil, err
	}
	s.extra = []mcpTool{
		noArgs("nodebridge_node_options", "Read registered edge node candidates from MySQL.", false, func() (any, error) { return app.GetNodeOptions(), nil }),
		noArgs("nodebridge_test_mysql", "Test the saved MySQL connection, including saved credentials.", false, func() (any, error) { return app.TestMySQL(app.config.MySQL), nil }),
		noArgs("nodebridge_test_rabbitmq", "Test saved local and central RabbitMQ connections.", false, func() (any, error) { return app.TestRabbitMQ(app.config.RabbitMQ), nil }),
		noArgs("nodebridge_agent_status", "Inspect the synchronization process.", false, func() (any, error) { return app.GetAgentProcessStatus(), nil }),
		bindTool("nodebridge_mysql_schema", "Inspect table columns and primary keys in the saved MySQL database; optional table filter.", false, s.mysqlSchema),
	}
	s.extra = append(s.extra,
		bindContextTool("nodebridge_event_status", "Inspect bounded runtime failures and apply receipts by event_id or rule_id; includes retry observations, positions and next retry time. Empty results never prove consistency. Does not read business rows or remove messages.", false, func(ctx context.Context, req eventstatus.Request) (eventstatus.Response, error) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return app.eventStatus(ctx, req)
		}),
		bindContextTool("nodebridge_rule_preflight", "Read local source or target schema and non-executing DML privilege checks for a saved rule_id. side must be source or target. Does not change rows; remote compatibility and existing data conflicts remain unverified.", false, func(ctx context.Context, req RulePreflightRequest) (any, error) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return app.preflightSyncRule(ctx, req)
		}),
		bindContextTool("nodebridge_mysql_diagnostics", "Read bounded MySQL connection/query timings, system-table estimates and indexes; optional event_tables adds exact counts and a non-executing failure query plan. No payloads or arbitrary SQL. timeout_seconds defaults to 5, maximum 10.", false, s.mysqlDiagnostics),
		bindTool("nodebridge_mysql_query", "Read governed business rows from the saved MySQL database with structured filters and a bounded limit.", false, s.mysqlGovernanceQuery),
		bindTool("nodebridge_mysql_mutation_plan", "Preview a bounded INSERT, UPDATE, or DELETE and count matching rows without changing data.", false, s.mysqlGovernanceMutationPlan),
		bindTool("nodebridge_mysql_mutation_apply", "Apply a confirmed structured INSERT, UPDATE, or DELETE when expected_rows still matches.", true, s.mysqlGovernanceMutationApply),
		bindTool("nodebridge_mysql_schema_change_plan", "Preview an ADD COLUMN or DROP COLUMN against an existing business table; table creation is disabled.", false, s.mysqlGovernanceSchemaPlan),
		bindTool("nodebridge_mysql_schema_change_apply", "Apply a confirmed ADD COLUMN or DROP COLUMN using a current plan_id; table creation is disabled.", true, s.mysqlGovernanceSchemaApply),
		noArgs("nodebridge_start_agent", "Start synchronization with the saved configuration and rules.", true, func() (any, error) {
			if err := s.Config.Validate(); err != nil {
				return nil, err
			}
			return app.StartAgent(), nil
		}),
		noArgs("nodebridge_stop_agent", "Stop synchronization gracefully.", true, func() (any, error) { return app.StopAgent(), nil }),
		noArgs("nodebridge_restart_agent", "Restart synchronization to apply saved configuration and rules.", true, func() (any, error) {
			if err := s.Config.Validate(); err != nil {
				return nil, err
			}
			return app.RestartAgent(), nil
		}),
		bindTool("nodebridge_retry_failed_event", "Requeue one failed event for retry.", true, app.RetryFailedEvent),
		bindTool("nodebridge_retry_failed_events", "Requeue failed events up to limit.", true, app.RetryFailedEvents),
		bindTool("nodebridge_dead_letters", "Preview dead letters and requeue them without deletion; this can affect queue ordering.", true, app.GetDeadLetters),
		bindContextTool("nodebridge_queue_event_plan", "Plan quarantine of one event matching a saved rule in a local NodeBridge queue. Requires stopped agent and no queue consumers. Scans at most 100 messages/2 MiB and requeues all inspected messages; ordering may change. Never applies business data.", true, app.planQueueEventQuarantine),
		bindContextTool("nodebridge_queue_event_apply", "Apply an explicitly confirmed quarantine plan. A persistent confirmed copy and audit precede ACK of the original. Keeps unrelated events; never purges a queue. Duplicate copies can occur after a lost confirmation; correlate plan_id. This is not successful business synchronization.", true, app.applyQueueEventQuarantine),
		bindTool("nodebridge_queue_event_audit", "Read one persisted quarantine plan/audit by plan_id. confirmed without acked means source ACK is unresolved, not successful synchronization.", false, app.GetQueueEventAudit),
		bindTool("nodebridge_managed_install_plan", "Plan NodeBridge-owned components and RabbitMQ topology.", false, app.GetManagedInstallPlan),
		bindTool("nodebridge_apply_managed_install", "Apply the NodeBridge-owned component plan, including RabbitMQ topology initialization.", true, app.ApplyManagedInstall),
		bindTool("nodebridge_ensure_server_edge_user", "On a center server, create or migrate one edge node RabbitMQ account using its node.id and the managed password.", true, func(req ensureServerEdgeUserRequest) (map[string]any, error) {
			if s.Config.Mode != appconfig.ModeServer || !appconfig.IsManagedRabbitMQ(s.Config) {
				return nil, errors.New("current node must be a server with managed RabbitMQ")
			}
			if err := s.rabbitMQAdmin.EnsureServerEdgeUser(context.Background(), req.NodeID); err != nil {
				return nil, err
			}
			identity, _ := appconfig.ManagedRabbitMQIdentityFor(appconfig.ModeEdge, req.NodeID)
			return map[string]any{"ok": true, "status": "configured", "node_id": req.NodeID, "username": identity.ServerUser, "vhost": identity.ServerVHost}, nil
		}),
		noArgs("nodebridge_export_diagnostics", "Create a local diagnostic ZIP and return its path on the target computer.", true, func() (any, error) { return app.ExportDiagnosticPackage() }),
		noArgs("nodebridge_get_autostart", "Read current Windows user's NodeBridge startup setting.", false, func() (any, error) { return app.GetAutoStart(), nil }),
		bindTool("nodebridge_set_autostart", "Change current Windows user's NodeBridge UI startup setting.", true, func(req uiapi.SetAutoStartRequest) (uiapi.AutoStartStatus, error) { return app.SetAutoStart(req), nil }),
	)
	s.extra = append(s.extra, noArgs("nodebridge_capabilities", "Read implemented capabilities. Initial alignment is not executable in this build.", false, func() (any, error) { return app.GetCapabilities(), nil }))
	if exe, err := os.Executable(); err == nil {
		app.autoStart = autostart.CurrentUserManager{Path: filepath.Join(filepath.Dir(exe), "NodeBridge.exe")}
	}
	return s, nil
}

func (s *MCPService) BeforeCall(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		f, err := agentstate.Lock(s.ConfigPath + ".mcp.lock")
		if err == nil {
			s.requestLock = f
			break
		}
		if !errors.Is(err, agentstate.ErrRunning) {
			return err
		}
		select {
		case <-ctx.Done():
			return errors.New("another MCP operation is busy; retry")
		case <-time.After(50 * time.Millisecond):
		}
	}
	if err := s.refresh(); err != nil {
		s.AfterCall()
		return err
	}
	return nil
}

func (s *MCPService) AfterCall() {
	if s.requestLock != nil {
		_ = s.requestLock.Close()
		s.requestLock = nil
	}
}

func (s *MCPService) refresh() error {
	cfg, err := appconfig.LoadFileAllowIncomplete(s.ConfigPath)
	if errors.Is(err, os.ErrNotExist) && s.bootstrap {
		cfg, err = &appconfig.Config{}, nil
	}
	if err != nil {
		return err
	}
	if !s.bootstrap {
		if !cfg.MCP.Enable {
			return errors.New("mcp_server.enable must be true to start mcp-stdio")
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
	}
	s.app.config = cfg
	s.app.ruleSet = nil
	s.Config = *cfg
	return nil
}

func (s *MCPService) Overview(context.Context) (any, error) {
	data, _ := json.Marshal(s.app.GetOverview())
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	result["mcp_read_only"] = false
	result["mcp_lab_full_access"] = s.LabFullAccess
	return result, nil
}
func (s *MCPService) QueueStatus(context.Context) (any, error) { return s.app.GetQueueStatus(), nil }
func (s *MCPService) FailedEvents(_ context.Context, limit int) (any, error) {
	return s.app.GetFailedEvents(uiapi.FailedEventsRequest{Limit: limit})
}
func (s *MCPService) Logs(ctx context.Context, limit int) (any, error) {
	service := s.StaticService
	service.LogPath = s.effectiveLogPath()
	if s.defaultLogPath && service.LogPath == agentlog.Path(s.ConfigPath) {
		if limit <= 0 || limit > 500 {
			limit = 100
		}
		lines := make([]string, 0, limit)
		for backup := 0; backup <= 4 && len(lines) < limit; backup++ {
			path := service.LogPath
			if backup > 0 {
				path = fmt.Sprintf("%s.%d", path, backup)
			}
			part, err := agentlog.Tail(path, limit-len(lines))
			if err != nil {
				if !os.IsNotExist(err) {
					return nil, fmt.Errorf("read runtime log: %w", err)
				}
				continue
			}
			lines = append(part, lines...)
		}
		for i, line := range lines {
			var record any
			if json.Unmarshal([]byte(line), &record) == nil {
				redacted, err := json.Marshal(s.redact(record))
				if err != nil {
					return nil, fmt.Errorf("encode runtime log: %w", err)
				}
				lines[i] = string(redacted)
			} else {
				lines[i] = s.redactText(line)
			}
		}
		return map[string]any{"items": lines, "log_path": s.redactText(service.LogPath)}, nil
	}
	value, err := service.Logs(ctx, limit)
	return s.redact(value), err
}

func (s *MCPService) effectiveLogPath() string {
	if s.defaultLogPath {
		path := agentlog.Path(s.ConfigPath)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return s.LogPath
}
func (s *MCPService) ConfigSummary(ctx context.Context) (any, error) {
	if !s.LabFullAccess {
		return s.StaticService.ConfigSummary(ctx)
	}
	return map[string]any{"config_path": s.ConfigPath, "config": s.app.GetConfig(), "write_policy": map[string]any{
		"mode": "lab_full_access", "secrets": "writable_encrypted_at_rest_redacted_on_read", "admin_unlock": "bypassed", "restart_required": true,
	}}, nil
}
func (s *MCPService) DiagnosticSummary(context.Context) (any, error) {
	return map[string]any{"transport": "stdio", "remote_transport": "ssh", "lab_full_access": s.LabFullAccess,
		"config_path": s.ConfigPath, "rules_path": s.RulesPath, "log_path": s.effectiveLogPath(), "runtime_log_path": agentlog.Path(s.ConfigPath),
		"audit_path": filepath.Join(filepath.Dir(s.ConfigPath), "logs", "mcp-audit.log"), "tools": s.Tools()}, nil
}

func (s *MCPService) Tools() []map[string]any {
	items := mcpstdio.BaseTools()
	for _, item := range items {
		name := item["name"].(string)
		switch name {
		case "nodebridge_overview":
			item["description"] = "Read node overview with live connection probes."
		case "nodebridge_queue_status":
			item["description"] = "Inspect actual RabbitMQ queue counts and consumers."
		case "nodebridge_failed_events":
			item["description"] = "Read failed events from MySQL."
		case "nodebridge_save_sync_rules":
			item["inputSchema"] = map[string]any{"type": "object", "properties": map[string]any{"rules": map[string]any{"type": "array", "items": jsonSchema(reflect.TypeOf(rules.SyncRule{}))}, "expected_revision": map[string]any{"type": "string"}}, "required": []string{"rules"}, "additionalProperties": false}
		case "nodebridge_validate_config_patch":
			if s.LabFullAccess {
				item["description"] = "Validate a partial NodeBridge config. All config sections including credentials, security and mcp_server are writable. Omitted fields are preserved; ****** preserves an existing secret."
				item["inputSchema"] = map[string]any{"type": "object", "properties": map[string]any{"patch": jsonSchema(reflect.TypeOf(appconfig.Config{}))}, "required": []string{"patch"}, "additionalProperties": false}
			}
		case "nodebridge_save_config_patch":
			if s.LabFullAccess {
				item["description"] = "Save a partial NodeBridge config. Set restart_agent=true to reload the saved config and restart SyncAgent immediately. Omitted fields are preserved; ****** preserves an existing secret."
				item["inputSchema"] = map[string]any{"type": "object", "properties": map[string]any{
					"patch": jsonSchema(reflect.TypeOf(appconfig.Config{})), "restart_agent": map[string]any{"type": "boolean"},
				}, "required": []string{"patch"}, "additionalProperties": false}
			}
		}
		write := strings.HasPrefix(name, "nodebridge_save_")
		item["annotations"] = map[string]any{"readOnlyHint": !write, "destructiveHint": write, "openWorldHint": true}
	}
	for _, tool := range s.extra {
		items = append(items, map[string]any{"name": tool.name, "description": tool.description, "inputSchema": tool.schema,
			"annotations": map[string]any{"readOnlyHint": !tool.write, "destructiveHint": tool.write, "openWorldHint": true}})
	}
	return items
}

func (s *MCPService) CallTool(ctx context.Context, name string, args json.RawMessage) (result any, resultErr error) {
	defer func() {
		// Logs already redacts parsed JSONL; redacting its serialization corrupts numbers.
		if name != "nodebridge_logs" {
			result = s.redact(result)
		}
		if resultErr != nil {
			if clean := s.redactText(resultErr.Error()); clean != resultErr.Error() {
				resultErr = errors.New(clean)
			}
		}
	}()
	if name == "nodebridge_save_config_patch" && s.LabFullAccess {
		return s.saveConfigPatch(ctx, args)
	}
	for _, tool := range s.extra {
		if tool.name == name {
			value, err := tool.call(ctx, args)
			if tool.write {
				data, _ := json.Marshal(value)
				var outcome map[string]any
				_ = json.Unmarshal(data, &outcome)
				success := err == nil
				if ok, exists := outcome["ok"].(bool); exists {
					success = success && ok
				}
				detail := map[string]any{"success": success}
				for _, key := range []string{"operation", "database", "table", "column", "status", "affected_rows", "plan_id"} {
					if field, exists := outcome[key]; exists {
						detail[key] = field
					}
				}
				s.RecordAuditDetails(name, detail)
			}
			return value, err
		}
	}
	return (mcpstdio.Server{Service: s}).CallTool(ctx, name, args)
}

func (s *MCPService) saveConfigPatch(ctx context.Context, args json.RawMessage) (any, error) {
	var req struct {
		Patch        map[string]json.RawMessage `json:"patch"`
		RestartAgent bool                       `json:"restart_agent,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	baseArgs, err := json.Marshal(struct {
		Patch map[string]json.RawMessage `json:"patch"`
	}{Patch: req.Patch})
	if err != nil {
		return nil, err
	}
	next, patch, err := s.BuildConfigPatch(ctx, baseArgs)
	if err != nil {
		return nil, err
	}
	if err := appconfig.NormalizeManagedRabbitMQ(&next); err != nil {
		return nil, err
	}
	if appconfig.IsManagedRabbitMQ(next) {
		if err := s.rabbitMQAdmin.EnsureForNode(ctx, next); err != nil {
			s.RecordAudit("nodebridge_save_config_patch_rabbitmq", false)
			return nil, fmt.Errorf("RabbitMQ service was not changed; config was not saved: %w", err)
		}
		s.RecordAudit("nodebridge_save_config_patch_rabbitmq", true)
	}
	if err := appconfig.SaveFile(s.ConfigPath, next); err != nil {
		return nil, err
	}
	s.RecordAudit("nodebridge_save_config_patch", true)
	saved := map[string]any{
		"ok": true, "status": "saved", "config_path": s.ConfigPath,
		"patch_keys": sortedRawKeys(patch), "config": uiapi.RedactConfig(uiapi.ConfigFromApp(next)),
	}
	if err != nil || !req.RestartAgent {
		return saved, err
	}
	if err := s.refresh(); err != nil {
		return nil, fmt.Errorf("config saved but reload failed: %w", err)
	}
	if err := s.Config.Validate(); err != nil {
		return nil, fmt.Errorf("config saved but agent restart validation failed: %w", err)
	}
	restarted := s.app.RestartAgent()
	if !restarted.OK {
		s.RecordAudit("nodebridge_save_config_patch_restart", false)
		return nil, fmt.Errorf("config saved but agent restart failed: %s", restarted.Message)
	}
	s.RecordAudit("nodebridge_save_config_patch_restart", true)
	return map[string]any{
		"ok": true, "status": "saved_and_restarted", "config_saved": true,
		"config_result": saved, "agent": restarted,
	}, nil
}

func sortedRawKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *MCPService) redact(value any) any {
	switch typed := value.(type) {
	case uiapi.AgentProcessStatus:
		typed.LastError = s.redactText(typed.LastError)
		typed.ExecutablePath = s.redactText(typed.ExecutablePath)
		typed.LogPath = s.redactText(typed.LogPath)
		return typed
	case uiapi.CapabilitiesResponse:
		return typed
	}
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&decoded) != nil {
		return value
	}
	var walk func(any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case string:
			return s.redactText(x)
		case []any:
			for i := range x {
				x[i] = walk(x[i])
			}
		case map[string]any:
			for k := range x {
				x[k] = walk(x[k])
			}
		}
		return v
	}
	return walk(decoded)
}

func (s *MCPService) redactText(text string) string {
	cfg := s.Config
	secrets := []string{cfg.MySQL.Password, cfg.RabbitMQ.Password, cfg.CDC.Password, cfg.LogWeb.Token, cfg.Security.AdminPassword, cfg.Security.ExitPassword}
	for _, raw := range []string{cfg.RabbitMQ.LocalURL, cfg.RabbitMQ.ServerURL} {
		if parsed, err := url.Parse(raw); err == nil && parsed.User != nil {
			if password, ok := parsed.User.Password(); ok {
				secrets = append(secrets, password, url.QueryEscape(password))
			}
		}
	}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	for _, secret := range secrets {
		if secret != "" && secret != appconfig.RedactedSecret {
			text = strings.ReplaceAll(text, secret, appconfig.RedactedSecret)
		}
	}
	return text
}

type schemaRequest struct {
	Table  string `json:"table,omitempty"`
	RuleID string `json:"rule_id,omitempty"`
	Side   string `json:"side,omitempty"`
}

func (s *MCPService) mysqlSchema(req schemaRequest) (any, error) {
	database, err := s.ruleDatabase(RuleDatabaseScope{RuleID: req.RuleID, Side: req.Side}, req.Table)
	if err != nil {
		return nil, err
	}
	db, err := mysqlconn.Open(s.Config.MySQL)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND (? = '' OR TABLE_NAME = ?) ORDER BY TABLE_NAME, ORDINAL_POSITION LIMIT 5000`, database, req.Table, req.Table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var table, column, kind, nullable, key string
		if err := rows.Scan(&table, &column, &kind, &nullable, &key); err != nil {
			return nil, err
		}
		items = append(items, map[string]string{"table": table, "column": column, "type": kind, "nullable": nullable, "key": key})
	}
	return map[string]any{"database": database, "columns": items, "limit": 5000}, rows.Err()
}

func noArgs(name, description string, write bool, call func() (any, error)) mcpTool {
	return bindTool(name, description, write, func(_ struct{}) (any, error) { return call() })
}

func bindTool[T, R any](name, description string, write bool, call func(T) (R, error)) mcpTool {
	return bindContextTool(name, description, write, func(_ context.Context, req T) (R, error) { return call(req) })
}

func bindContextTool[T, R any](name, description string, write bool, call func(context.Context, T) (R, error)) mcpTool {
	typ := reflect.TypeFor[T]()
	schema := jsonSchema(typ)
	required := jsonRequiredFields(typ)
	if len(required) > 0 {
		schema["required"] = required
	}
	return mcpTool{name: name, description: description, write: write, schema: schema, call: func(ctx context.Context, raw json.RawMessage) (any, error) {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, err
		}
		for _, key := range required {
			if value, ok := fields[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, fmt.Errorf("%s is required", key)
			}
		}
		var req T
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return call(ctx, req)
	}}
}

// Derive field names from the same DTOs accepted by management operations.
func jsonSchema(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.Pointer:
		return jsonSchema(t.Elem())
	case reflect.Struct:
		properties := map[string]any{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous && f.Tag.Get("json") == "" && f.Type.Kind() == reflect.Struct {
				child := jsonSchema(f.Type)["properties"].(map[string]any)
				for key, value := range child {
					properties[key] = value
				}
				continue
			}
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name != "" && name != "-" {
				properties[name] = jsonSchema(f.Type)
			}
		}
		return map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": jsonSchema(t.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": jsonSchema(t.Elem())}
	case reflect.Interface:
		return map[string]any{}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64, reflect.Int32:
		return map[string]any{"type": "integer"}
	default:
		return map[string]any{"type": "string"}
	}
}

func jsonRequiredFields(typ reflect.Type) []string {
	required := []string{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("json")
		if field.Anonymous && tag == "" && field.Type.Kind() == reflect.Struct {
			required = append(required, jsonRequiredFields(field.Type)...)
			continue
		}
		if tag != "" && tag != "-" && !strings.Contains(tag, ",omitempty") {
			required = append(required, strings.Split(tag, ",")[0])
		}
	}
	return required
}
