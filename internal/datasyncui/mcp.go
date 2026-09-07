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

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/autostart"
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
	app           *App
	extra         []mcpTool
	requestLock   *os.File
	rabbitMQAdmin managedRabbitMQAdmin
}

type managedRabbitMQAdmin interface {
	EnsureForNode(context.Context, appconfig.Config) error
	EnsureServerEdgeUser(context.Context, string) error
}

type mcpTool struct {
	name, description string
	schema            map[string]any
	write             bool
	call              func(json.RawMessage) (any, error)
}

type ensureServerEdgeUserRequest struct {
	NodeID string `json:"node_id"`
}

func NewMCPService(configPath, rulesPath, logPath string, lab bool) (*MCPService, error) {
	app := &App{
		configPath: configPath, rulesPath: rulesPath,
		runtime: status.NewRuntimeStore(), protector: appconfig.DefaultSecretProtector(),
		autoStart: autostart.NewCurrentUserManager(), agent: newExternalAgentController(),
		auth: adminAuth{Timeout: defaultAdminTimeout}, mcpLabFullAccess: lab,
	}
	if logPath == "" {
		logPath = agentLogPath(configPath)
	}
	app.agent.(*externalAgentController).configPath = configPath
	admin := rabbitadmin.NewAdmin()
	s := &MCPService{StaticService: mcpstdio.StaticService{ConfigPath: configPath, RulesPath: rulesPath, LogPath: logPath, LabFullAccess: lab}, app: app, rabbitMQAdmin: admin}
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
	if lab {
		s.extra = append(s.extra,
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
			bindTool("nodebridge_dead_letters", "Preview dead letters and requeue them without deletion.", false, app.GetDeadLetters),
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
	}
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
	if errors.Is(err, os.ErrNotExist) && s.LabFullAccess {
		cfg, err = &appconfig.Config{}, nil
	}
	if err != nil {
		return err
	}
	if !s.LabFullAccess {
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
	value, err := s.StaticService.Logs(ctx, limit)
	return s.redact(value), err
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
		"config_path": s.ConfigPath, "rules_path": s.RulesPath, "log_path": s.LogPath,
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
			item["inputSchema"] = map[string]any{"type": "object", "properties": map[string]any{"rules": map[string]any{"type": "array", "items": jsonSchema(reflect.TypeOf(rules.SyncRule{}))}}, "required": []string{"rules"}, "additionalProperties": false}
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
		result = s.redact(result)
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
			value, err := tool.call(args)
			if tool.write {
				data, _ := json.Marshal(value)
				var outcome struct {
					OK *bool `json:"ok"`
				}
				_ = json.Unmarshal(data, &outcome)
				s.RecordAudit(name, err == nil && (outcome.OK == nil || *outcome.OK))
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
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
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
	Table string `json:"table,omitempty"`
}

func (s *MCPService) mysqlSchema(req schemaRequest) (any, error) {
	db, err := mysqlconn.Open(s.Config.MySQL)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND (? = '' OR TABLE_NAME = ?) ORDER BY TABLE_NAME, ORDINAL_POSITION LIMIT 5000`, s.Config.MySQL.Database, req.Table, req.Table)
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
	return map[string]any{"database": s.Config.MySQL.Database, "columns": items, "limit": 5000}, rows.Err()
}

func noArgs(name, description string, write bool, call func() (any, error)) mcpTool {
	return bindTool(name, description, write, func(_ struct{}) (any, error) { return call() })
}

func bindTool[T, R any](name, description string, write bool, call func(T) (R, error)) mcpTool {
	typ := reflect.TypeFor[T]()
	schema := jsonSchema(typ)
	required := []string{}
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag != "" && tag != "-" && !strings.Contains(tag, ",omitempty") {
			required = append(required, strings.Split(tag, ",")[0])
		}
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return mcpTool{name: name, description: description, write: write, schema: schema, call: func(raw json.RawMessage) (any, error) {
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
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		return call(req)
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
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64, reflect.Int32:
		return map[string]any{"type": "integer"}
	default:
		return map[string]any{"type": "string"}
	}
}
