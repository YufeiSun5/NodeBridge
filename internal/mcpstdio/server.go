package mcpstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/buildinfo"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

type Service interface {
	Overview(ctx context.Context) (any, error)
	ConfigSummary(ctx context.Context) (any, error)
	QueueStatus(ctx context.Context) (any, error)
	SyncRules(ctx context.Context) (any, error)
	FailedEvents(ctx context.Context, limit int) (any, error)
	Logs(ctx context.Context, limit int) (any, error)
	DiagnosticSummary(ctx context.Context) (any, error)
	ValidateConfigPatch(ctx context.Context, args json.RawMessage) (any, error)
	SaveConfigPatch(ctx context.Context, args json.RawMessage) (any, error)
	SaveSyncRules(ctx context.Context, args json.RawMessage) (any, error)
}

type StaticService struct {
	LabFullAccess bool
	ConfigPath    string
	RulesPath     string
	LogPath       string
	AuditPath     string
	Config        appconfig.Config
	Rules         []rules.SyncRule
}

func (s StaticService) Overview(context.Context) (any, error) {
	cfg, err := s.currentConfig()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"product_name":   "NodeBridge",
		"mode":           cfg.Mode,
		"node_id":        cfg.Node.ID,
		"node_name":      cfg.Node.Name,
		"config_path":    s.ConfigPath,
		"rules_path":     s.RulesPath,
		"mysql_status":   "unknown",
		"rabbitmq":       uiapi.RedactURL(cfg.RabbitMQ.ServerURL),
		"mcp_read_only":  false,
		"mcp_write_mode": "limited_config_write",
	}, nil
}

func (s StaticService) ConfigSummary(context.Context) (any, error) {
	cfg, err := s.currentConfig()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"config_path": s.ConfigPath,
		"config":      uiapi.RedactConfig(uiapi.ConfigFromApp(cfg)),
		"write_policy": map[string]any{
			"mode":         "limited_patch",
			"dry_run":      "nodebridge_validate_config_patch",
			"secrets":      "rejected",
			"raw_yaml":     "rejected",
			"audit_path":   s.effectiveAuditPath(),
			"allowed_note": "node/mysql/rabbitmq/cdc/sync/log_web non-secret fields only",
		},
	}, nil
}

func (s StaticService) QueueStatus(context.Context) (any, error) {
	return map[string]any{"status": "unknown", "message": "mcp stdio alpha does not open RabbitMQ"}, nil
}

func (s StaticService) SyncRules(context.Context) (any, error) {
	if strings.TrimSpace(s.RulesPath) != "" {
		set, revision, err := rules.LoadFileWithRevision(s.RulesPath)
		if err != nil {
			return nil, err
		}
		return map[string]any{"rules": set.Rules, "saved_revision": revision}, nil
	}
	current, err := s.currentRules()
	if err != nil {
		return nil, err
	}
	return map[string]any{"rules": current}, nil
}

func (s StaticService) FailedEvents(context.Context, int) (any, error) {
	return map[string]any{"items": []any{}, "message": "mcp stdio alpha does not open MySQL"}, nil
}

func (s StaticService) Logs(_ context.Context, limit int) (any, error) {
	return map[string]any{"items": readLogTail(s.LogPath, limit), "log_path": s.LogPath}, nil
}

func (s StaticService) DiagnosticSummary(context.Context) (any, error) {
	return map[string]any{
		"product_name":    "NodeBridge",
		"config_path":     s.ConfigPath,
		"rules_path":      s.RulesPath,
		"log_path":        s.LogPath,
		"audit_path":      s.effectiveAuditPath(),
		"safe_mode":       "limited_write",
		"transport":       "stdio",
		"network_ports":   []any{},
		"write_tools":     []string{"nodebridge_validate_config_patch", "nodebridge_save_config_patch", "nodebridge_save_sync_rules"},
		"allowed_actions": []string{"read_config_summary", "read_rules", "read_logs", "read_diagnostics", "validate_non_secret_config_patch", "save_non_secret_config_patch", "save_sync_rules"},
		"blocked_actions": []string{"write_passwords", "write_tokens", "write_raw_yaml", "rabbitmq_mutation", "mysql_write"},
	}, nil
}

func (s StaticService) ValidateConfigPatch(ctx context.Context, args json.RawMessage) (any, error) {
	next, patch, err := s.BuildConfigPatch(ctx, args)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":          true,
		"status":      "valid",
		"config_path": s.ConfigPath,
		"patch_keys":  sortedKeys(patch),
		"config":      uiapi.RedactConfig(uiapi.ConfigFromApp(next)),
	}, nil
}

func (s StaticService) SaveConfigPatch(ctx context.Context, args json.RawMessage) (any, error) {
	if strings.TrimSpace(s.ConfigPath) == "" {
		return nil, fmt.Errorf("config_path is required")
	}
	next, patch, err := s.BuildConfigPatch(ctx, args)
	if err != nil {
		s.audit("reject_config_patch", map[string]any{"config_path": s.ConfigPath, "reason": err.Error(), "patch_keys": patchKeysFromArgs(args)})
		return nil, err
	}
	if err := appconfig.SaveFile(s.ConfigPath, next); err != nil {
		s.audit("reject_config_patch", map[string]any{"config_path": s.ConfigPath, "reason": err.Error(), "patch_keys": sortedKeys(patch)})
		return nil, err
	}
	s.audit("save_config_patch", map[string]any{"config_path": s.ConfigPath, "patch_keys": sortedKeys(patch)})
	return map[string]any{
		"ok":          true,
		"status":      "saved",
		"config_path": s.ConfigPath,
		"config":      uiapi.RedactConfig(uiapi.ConfigFromApp(next)),
	}, nil
}

func (s StaticService) BuildConfigPatch(_ context.Context, args json.RawMessage) (appconfig.Config, map[string]json.RawMessage, error) {
	var req struct {
		Patch map[string]json.RawMessage `json:"patch"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return appconfig.Config{}, nil, err
	}
	if len(req.Patch) == 0 {
		return appconfig.Config{}, nil, fmt.Errorf("patch is required")
	}
	if err := rejectSensitivePatch(req.Patch, "patch"); err != nil && !s.LabFullAccess {
		return appconfig.Config{}, req.Patch, err
	}
	next, err := s.currentConfig()
	if err != nil {
		return appconfig.Config{}, req.Patch, err
	}
	if s.LabFullAccess {
		previous := next
		data, _ := json.Marshal(req.Patch)
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&next); err != nil {
			return appconfig.Config{}, req.Patch, fmt.Errorf("invalid config patch: %w", err)
		}
		next = appconfig.MergeRedactedSecrets(next, previous)
		if err := next.Validate(); err != nil {
			return appconfig.Config{}, req.Patch, err
		}
		return next, req.Patch, nil
	}
	if err := applyConfigPatch(&next, req.Patch); err != nil {
		return appconfig.Config{}, req.Patch, err
	}
	return next, req.Patch, nil
}

func (s StaticService) SaveSyncRules(ctx context.Context, args json.RawMessage) (any, error) {
	if strings.TrimSpace(s.RulesPath) == "" {
		return nil, fmt.Errorf("rules_path is required")
	}
	var req struct {
		Rules            []rules.SyncRule `json:"rules"`
		ExpectedRevision string           `json:"expected_revision"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	set := rules.RuleSet{Rules: req.Rules}
	if req.Rules == nil {
		return nil, fmt.Errorf("rules array is required; use [] to remove all rules")
	}
	revision, err := rules.SaveFileCAS(s.RulesPath, set, req.ExpectedRevision)
	if err != nil {
		s.audit("reject_sync_rules", map[string]any{"rules_path": s.RulesPath, "reason": err.Error(), "rule_count": len(req.Rules)})
		return nil, err
	}
	s.audit("save_sync_rules", map[string]any{"rules_path": s.RulesPath, "rule_count": len(req.Rules)})
	return map[string]any{
		"ok":             true,
		"status":         "saved",
		"rules_path":     s.RulesPath,
		"rule_count":     len(req.Rules),
		"rules":          req.Rules,
		"saved_revision": revision,
		"activation":     "restart_required",
	}, nil
}

type Server struct {
	Service Service
}

// Extension supplies explicitly registered tools, never arbitrary method dispatch.
type Extension interface {
	Tools() []map[string]any
	CallTool(context.Context, string, json.RawMessage) (any, error)
	BeforeCall(context.Context) error
	AfterCall()
}

type request struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s Server) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "\ufeff")
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := writeResponse(stdout, response{JSONRPC: "2.0", Error: &responseError{Code: -32700, Message: "Parse error"}}); err != nil {
				return err
			}
			continue
		}
		resp, ok := s.handle(ctx, req)
		if !ok {
			continue
		}
		if err := writeResponse(stdout, resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s Server) handle(ctx context.Context, req request) (response, bool) {
	if req.JSONRPC != "2.0" || req.Method == "" || !validID(req.ID) {
		return response{JSONRPC: "2.0", Error: &responseError{Code: -32600, Message: "Invalid Request"}}, true
	}
	if len(req.ID) == 0 {
		return response{}, false
	}
	result, err := s.call(ctx, req.Method, req.Params)
	if err != nil {
		code := -32603
		var protocol *rpcError
		if errors.As(err, &protocol) {
			code = protocol.code
		}
		return response{JSONRPC: "2.0", ID: req.ID, Error: &responseError{Code: code, Message: err.Error()}}, true
	}
	return response{JSONRPC: "2.0", ID: req.ID, Result: result}, true
}

func (s Server) call(ctx context.Context, method string, params json.RawMessage) (any, error) {
	if s.Service == nil {
		return nil, fmt.Errorf("mcp service is not configured")
	}
	switch method {
	case "initialize":
		var init struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(params) > 0 && json.Unmarshal(params, &init) != nil {
			return nil, &rpcError{-32602, "Invalid initialize params"}
		}
		version := init.ProtocolVersion
		switch version {
		case "2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25":
		default:
			version = "2025-11-25"
		}
		return map[string]any{
			"protocolVersion": version,
			"serverInfo":      map[string]string{"name": "nodebridge", "version": buildinfo.Version},
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		if ext, ok := s.Service.(Extension); ok {
			return map[string]any{"tools": ext.Tools()}, nil
		}
		return map[string]any{"tools": tools()}, nil
	case "tools/call":
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(params, &call); err != nil {
			return nil, &rpcError{-32602, "Invalid tools/call params"}
		}
		if len(call.Arguments) == 0 {
			call.Arguments = json.RawMessage(`{}`)
		}
		if raw := bytes.TrimSpace(call.Arguments); len(raw) == 0 || raw[0] != '{' {
			return nil, &rpcError{-32602, "arguments must be an object"}
		}
		var value any
		var err error
		if ext, ok := s.Service.(Extension); ok {
			if err = ext.BeforeCall(ctx); err == nil {
				defer ext.AfterCall()
				value, err = ext.CallTool(ctx, call.Name, call.Arguments)
			}
		} else {
			value, err = s.callTool(ctx, call.Name, call.Arguments)
		}
		if err != nil {
			var protocol *rpcError
			if errors.As(err, &protocol) {
				return nil, err
			}
			return map[string]any{"isError": true, "content": []map[string]string{{"type": "text", "text": err.Error()}}}, nil
		}
		data, _ := json.MarshalIndent(value, "", "  ")
		var outcome struct {
			OK     *bool  `json:"ok"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal(data, &outcome)
		failed := outcome.OK != nil && !*outcome.OK || outcome.Status == "error" || outcome.Status == "locked"
		return map[string]any{"isError": failed, "content": []map[string]string{{"type": "text", "text": string(data)}}}, nil
	case "resources/list":
		return map[string]any{"resources": resources()}, nil
	case "resources/read":
		if ext, ok := s.Service.(Extension); ok {
			if err := ext.BeforeCall(ctx); err != nil {
				return nil, err
			}
			defer ext.AfterCall()
		}
		var read struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(params, &read); err != nil {
			return nil, err
		}
		value, err := s.readResource(ctx, read.URI)
		if err != nil {
			return nil, err
		}
		data, _ := json.MarshalIndent(value, "", "  ")
		return map[string]any{"contents": []map[string]string{{"uri": read.URI, "mimeType": "application/json", "text": string(data)}}}, nil
	default:
		return nil, &rpcError{-32601, "unsupported method " + method}
	}
}

func (s Server) callTool(ctx context.Context, name string, args json.RawMessage) (any, error) {
	limit := limitArg(args)
	switch name {
	case "nodebridge_overview":
		return s.Service.Overview(ctx)
	case "nodebridge_config_summary":
		return s.Service.ConfigSummary(ctx)
	case "nodebridge_queue_status":
		return s.Service.QueueStatus(ctx)
	case "nodebridge_sync_rules":
		return s.Service.SyncRules(ctx)
	case "nodebridge_failed_events":
		return s.Service.FailedEvents(ctx, limit)
	case "nodebridge_logs":
		return s.Service.Logs(ctx, limit)
	case "nodebridge_diagnostic_summary":
		return s.Service.DiagnosticSummary(ctx)
	case "nodebridge_validate_config_patch":
		return s.Service.ValidateConfigPatch(ctx, args)
	case "nodebridge_save_config_patch":
		return s.Service.SaveConfigPatch(ctx, args)
	case "nodebridge_save_sync_rules":
		return s.Service.SaveSyncRules(ctx, args)
	default:
		return nil, UnknownTool(name)
	}
}

func (s Server) readResource(ctx context.Context, uri string) (any, error) {
	switch uri {
	case "nodebridge://overview":
		return s.Service.Overview(ctx)
	case "nodebridge://config":
		return s.Service.ConfigSummary(ctx)
	case "nodebridge://sync-rules":
		return s.Service.SyncRules(ctx)
	case "nodebridge://diagnostic-summary":
		return s.Service.DiagnosticSummary(ctx)
	case "nodebridge://logs":
		return s.Service.Logs(ctx, 100)
	default:
		return nil, fmt.Errorf("unsupported resource %s", uri)
	}
}

func tools() []map[string]any {
	items := []struct {
		name        string
		description string
	}{
		{"nodebridge_overview", "Read NodeBridge mode, node, config paths, and redacted broker summary."},
		{"nodebridge_config_summary", "Read redacted NodeBridge configuration and MCP write policy."},
		{"nodebridge_queue_status", "Read queue status placeholder without opening RabbitMQ."},
		{"nodebridge_sync_rules", "Read configured synchronization rules, including mapping and dispatch policy."},
		{"nodebridge_failed_events", "Read failed event placeholder without opening MySQL."},
		{"nodebridge_logs", "Read the configured SyncAgent log tail when a log file is provided."},
		{"nodebridge_diagnostic_summary", "Read MCP safety boundary and diagnostic file locations."},
		{"nodebridge_validate_config_patch", "Validate a whitelisted non-secret config patch and return the redacted result without saving."},
		{"nodebridge_save_config_patch", "Save a whitelisted non-secret config patch; passwords, tokens, raw YAML, RabbitMQ mutation, and MySQL writes are rejected."},
		{"nodebridge_save_sync_rules", "Save synchronization rules after identifier, mapping, dispatch, and sync_mode validation."},
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"name":        item.name,
			"description": item.description,
			"inputSchema": inputSchemaForTool(item.name),
		})
	}
	return result
}

func inputSchemaForTool(name string) map[string]any {
	switch name {
	case "nodebridge_validate_config_patch", "nodebridge_save_config_patch":
		return map[string]any{
			"type":     "object",
			"required": []string{"patch"},
			"properties": map[string]any{
				"patch": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
					"description":          "Whitelisted non-secret NodeBridge config patch.",
				},
			},
		}
	case "nodebridge_save_sync_rules":
		return map[string]any{
			"type":     "object",
			"required": []string{"rules"},
			"properties": map[string]any{
				"expected_revision": map[string]any{"type": "string", "description": "Required when replacing an existing file. Use saved_revision from nodebridge_sync_rules; stale revisions are rejected."},
				"rules": map[string]any{"type": "array", "items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"delete_mode":     map[string]any{"type": "string", "enum": []string{"HARD", "SOFT"}, "description": "Missing legacy value means SOFT. HARD only applies source DELETE events, not extra target rows."},
						"conflict_policy": map[string]any{"type": "string", "description": "Only NONE is supported for enabled rules; SERVER_WIN and LAST_WRITE_WIN cannot be enabled."},
					},
				}},
			},
		}
	case "nodebridge_failed_events", "nodebridge_logs":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 500},
			},
		}
	default:
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
}

func resources() []map[string]any {
	return []map[string]any{
		{"uri": "nodebridge://overview", "name": "NodeBridge Overview", "mimeType": "application/json"},
		{"uri": "nodebridge://config", "name": "NodeBridge Redacted Config", "mimeType": "application/json"},
		{"uri": "nodebridge://sync-rules", "name": "NodeBridge Sync Rules", "mimeType": "application/json"},
		{"uri": "nodebridge://diagnostic-summary", "name": "NodeBridge Diagnostic Summary", "mimeType": "application/json"},
		{"uri": "nodebridge://logs", "name": "NodeBridge Logs", "mimeType": "application/json"},
	}
}

func applyConfigPatch(cfg *appconfig.Config, patch map[string]json.RawMessage) error {
	for key, raw := range patch {
		switch key {
		case "mode":
			if err := decodeScalar(raw, &cfg.Mode); err != nil {
				return fieldError(key, err)
			}
		case "node":
			if err := applyNodePatch(&cfg.Node, raw); err != nil {
				return err
			}
		case "mysql":
			if err := applyMySQLPatch(&cfg.MySQL, raw); err != nil {
				return err
			}
		case "rabbitmq":
			if err := applyRabbitMQPatch(&cfg.RabbitMQ, raw); err != nil {
				return err
			}
		case "cdc":
			if err := applyCDCPatch(&cfg.CDC, raw); err != nil {
				return err
			}
		case "sync":
			if err := applySyncPatch(&cfg.Sync, raw); err != nil {
				return err
			}
		case "log_web":
			if err := applyLogWebPatch(&cfg.LogWeb, raw); err != nil {
				return err
			}
		default:
			return fmt.Errorf("config patch field %q is not allowed", key)
		}
	}
	return cfg.Validate()
}

func applyNodePatch(node *appconfig.NodeConfig, raw json.RawMessage) error {
	return applyObject(raw, "node", map[string]func(json.RawMessage) error{
		"id":       func(v json.RawMessage) error { return decodeScalar(v, &node.ID) },
		"name":     func(v json.RawMessage) error { return decodeScalar(v, &node.Name) },
		"location": func(v json.RawMessage) error { return decodeScalar(v, &node.Location) },
	})
}

func applyMySQLPatch(mysql *appconfig.MySQLConfig, raw json.RawMessage) error {
	return applyObject(raw, "mysql", map[string]func(json.RawMessage) error{
		"host":     func(v json.RawMessage) error { return decodeScalar(v, &mysql.Host) },
		"port":     func(v json.RawMessage) error { return decodeScalar(v, &mysql.Port) },
		"username": func(v json.RawMessage) error { return decodeScalar(v, &mysql.Username) },
		"database": func(v json.RawMessage) error { return decodeScalar(v, &mysql.Database) },
	})
}

func applyRabbitMQPatch(rabbit *appconfig.RabbitMQConfig, raw json.RawMessage) error {
	return applyObject(raw, "rabbitmq", map[string]func(json.RawMessage) error{
		"mode":           func(v json.RawMessage) error { return decodeScalar(v, &rabbit.Mode) },
		"install":        func(v json.RawMessage) error { return decodeScalar(v, &rabbit.Install) },
		"local_url":      func(v json.RawMessage) error { return decodeSafeURL(v, &rabbit.LocalURL, "rabbitmq.local_url") },
		"server_url":     func(v json.RawMessage) error { return decodeSafeURL(v, &rabbit.ServerURL, "rabbitmq.server_url") },
		"management_url": func(v json.RawMessage) error { return decodeScalar(v, &rabbit.ManagementURL) },
		"username":       func(v json.RawMessage) error { return decodeScalar(v, &rabbit.Username) },
		"vhost":          func(v json.RawMessage) error { return decodeScalar(v, &rabbit.VHost) },
	})
}

func applyCDCPatch(cdc *appconfig.CDCConfig, raw json.RawMessage) error {
	return applyObject(raw, "cdc", map[string]func(json.RawMessage) error{
		"type":         func(v json.RawMessage) error { return decodeScalar(v, &cdc.Type) },
		"mode":         func(v json.RawMessage) error { return decodeScalar(v, &cdc.Mode) },
		"install":      func(v json.RawMessage) error { return decodeScalar(v, &cdc.Install) },
		"reader_name":  func(v json.RawMessage) error { return decodeScalar(v, &cdc.ReaderName) },
		"canal_addr":   func(v json.RawMessage) error { return decodeScalar(v, &cdc.CanalAddr) },
		"config_dir":   func(v json.RawMessage) error { return decodeScalar(v, &cdc.ConfigDir) },
		"service_name": func(v json.RawMessage) error { return decodeScalar(v, &cdc.ServiceName) },
		"destination":  func(v json.RawMessage) error { return decodeScalar(v, &cdc.Destination) },
		"username":     func(v json.RawMessage) error { return decodeScalar(v, &cdc.Username) },
		"filter":       func(v json.RawMessage) error { return decodeScalar(v, &cdc.Filter) },
		"batch_size":   func(v json.RawMessage) error { return decodeScalar(v, &cdc.BatchSize) },
		"use_gtid":     func(v json.RawMessage) error { return decodeScalar(v, &cdc.UseGTID) },
	})
}

func applySyncPatch(sync *appconfig.SyncConfig, raw json.RawMessage) error {
	return applyObject(raw, "sync", map[string]func(json.RawMessage) error{
		"upload_batch_size":          func(v json.RawMessage) error { return decodeScalar(v, &sync.UploadBatchSize) },
		"dispatch_batch_size":        func(v json.RawMessage) error { return decodeScalar(v, &sync.DispatchBatchSize) },
		"apply_lanes":                func(v json.RawMessage) error { return decodeScalar(v, &sync.ApplyLanes) },
		"enable_crud_compact":        func(v json.RawMessage) error { return decodeScalar(v, &sync.EnableCRUDCompact) },
		"flush_interval_millis":      func(v json.RawMessage) error { return decodeScalar(v, &sync.FlushIntervalMillis) },
		"retry_interval_seconds":     func(v json.RawMessage) error { return decodeScalar(v, &sync.RetryIntervalSeconds) },
		"heartbeat_interval_seconds": func(v json.RawMessage) error { return decodeScalar(v, &sync.HeartbeatIntervalSecond) },
		"node_timeout_seconds":       func(v json.RawMessage) error { return decodeScalar(v, &sync.NodeTimeoutSeconds) },
	})
}

func applyLogWebPatch(logWeb *appconfig.LogWebConfig, raw json.RawMessage) error {
	return applyObject(raw, "log_web", map[string]func(json.RawMessage) error{
		"enable": func(v json.RawMessage) error { return decodeScalar(v, &logWeb.Enable) },
		"bind":   func(v json.RawMessage) error { return decodeScalar(v, &logWeb.Bind) },
		"port":   func(v json.RawMessage) error { return decodeScalar(v, &logWeb.Port) },
	})
}

func applyObject(raw json.RawMessage, prefix string, setters map[string]func(json.RawMessage) error) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return fieldError(prefix, err)
	}
	for key, value := range values {
		set, ok := setters[key]
		if !ok {
			return fmt.Errorf("config patch field %q is not allowed", prefix+"."+key)
		}
		if err := set(value); err != nil {
			return fieldError(prefix+"."+key, err)
		}
	}
	return nil
}

func decodeScalar(raw json.RawMessage, target any) error {
	return json.Unmarshal(raw, target)
}

func decodeSafeURL(raw json.RawMessage, target *string, field string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if hasURLPassword(value) {
		return fmt.Errorf("%s must not include a password in MCP config patch", field)
	}
	*target = value
	return nil
}

func rejectSensitivePatch(values map[string]json.RawMessage, prefix string) error {
	for key, raw := range values {
		lower := strings.ToLower(key)
		switch lower {
		case "password", "token", "admin_password", "exit_password", "security":
			return fmt.Errorf("sensitive field %q is not allowed through MCP", prefix+"."+key)
		}
		if len(raw) > 0 && raw[0] == '{' {
			var nested map[string]json.RawMessage
			if err := json.Unmarshal(raw, &nested); err != nil {
				return fieldError(prefix+"."+key, err)
			}
			if err := rejectSensitivePatch(nested, prefix+"."+key); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasURLPassword(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return false
	}
	_, ok := parsed.User.Password()
	return ok
}

func fieldError(field string, err error) error {
	return fmt.Errorf("%s: %w", field, err)
}

func sortedKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func patchKeysFromArgs(args json.RawMessage) []string {
	var req struct {
		Patch map[string]json.RawMessage `json:"patch"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return []string{}
	}
	return sortedKeys(req.Patch)
}

func (s StaticService) currentConfig() (appconfig.Config, error) {
	if strings.TrimSpace(s.ConfigPath) == "" {
		return s.Config, nil
	}
	load := appconfig.LoadFile
	if s.LabFullAccess {
		load = appconfig.LoadFileAllowIncomplete
	}
	cfg, err := load(s.ConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.Config, nil
		}
		return appconfig.Config{}, err
	}
	return *cfg, nil
}

func (s StaticService) currentRules() ([]rules.SyncRule, error) {
	if strings.TrimSpace(s.RulesPath) == "" {
		return s.Rules, nil
	}
	set, err := rules.LoadFile(s.RulesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.Rules, nil
		}
		return nil, err
	}
	return append([]rules.SyncRule(nil), set.Rules...), nil
}

func (s StaticService) audit(action string, detail map[string]any) {
	path := s.effectiveAuditPath()
	if strings.TrimSpace(path) == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	entry := map[string]any{
		"time":   time.Now().Format(time.RFC3339Nano),
		"module": "mcpstdio",
		"action": action,
		"detail": detail,
	}
	data, _ := json.Marshal(entry)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

func (s StaticService) effectiveAuditPath() string {
	if strings.TrimSpace(s.AuditPath) != "" {
		return s.AuditPath
	}
	if strings.TrimSpace(s.ConfigPath) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(s.ConfigPath), "logs", "mcp-audit.log")
}

func limitArg(raw json.RawMessage) int {
	var args struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(raw, &args)
	if args.Limit <= 0 || args.Limit > 500 {
		return 100
	}
	return args.Limit
}

func writeResponse(stdout io.Writer, resp response) error {
	return json.NewEncoder(stdout).Encode(resp)
}

func readLogTail(path string, limit int) []string {
	if strings.TrimSpace(path) == "" {
		return []string{}
	}
	lines, err := agentlog.Tail(path, limit)
	if err != nil {
		return []string{"log unavailable: " + err.Error()}
	}
	return lines
}
