package mcpstdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

type Service interface {
	Overview(ctx context.Context) (any, error)
	QueueStatus(ctx context.Context) (any, error)
	SyncRules(ctx context.Context) (any, error)
	FailedEvents(ctx context.Context, limit int) (any, error)
	Logs(ctx context.Context, limit int) (any, error)
	DiagnosticSummary(ctx context.Context) (any, error)
}

type StaticService struct {
	ConfigPath string
	RulesPath  string
	Config     appconfig.Config
	Rules      []rules.SyncRule
}

func (s StaticService) Overview(context.Context) (any, error) {
	return map[string]any{
		"product_name":  "NodeBridge",
		"mode":          s.Config.Mode,
		"node_id":       s.Config.Node.ID,
		"node_name":     s.Config.Node.Name,
		"config_path":   s.ConfigPath,
		"rules_path":    s.RulesPath,
		"mysql_status":  "unknown",
		"rabbitmq":      uiapi.RedactURL(s.Config.RabbitMQ.ServerURL),
		"mcp_read_only": true,
	}, nil
}

func (s StaticService) QueueStatus(context.Context) (any, error) {
	return map[string]any{"status": "unknown", "message": "mcp stdio alpha does not open RabbitMQ"}, nil
}

func (s StaticService) SyncRules(context.Context) (any, error) {
	return map[string]any{"rules": s.Rules}, nil
}

func (s StaticService) FailedEvents(context.Context, int) (any, error) {
	return map[string]any{"items": []any{}, "message": "mcp stdio alpha does not open MySQL"}, nil
}

func (s StaticService) Logs(context.Context, int) (any, error) {
	return map[string]any{"items": []any{}}, nil
}

func (s StaticService) DiagnosticSummary(context.Context) (any, error) {
	return map[string]any{
		"product_name": "NodeBridge",
		"config_path":  s.ConfigPath,
		"rules_path":   s.RulesPath,
		"safe_mode":    "read_only",
	}, nil
}

type Server struct {
	Service Service
}

type request struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
	Error   *responseError `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s Server) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "\ufeff")
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeResponse(stdout, response{JSONRPC: "2.0", Error: &responseError{Code: -32700, Message: err.Error()}})
			continue
		}
		resp := s.handle(ctx, req)
		writeResponse(stdout, resp)
	}
	return scanner.Err()
}

func (s Server) handle(ctx context.Context, req request) response {
	result, err := s.call(ctx, req.Method, req.Params)
	if err != nil {
		return response{JSONRPC: "2.0", ID: req.ID, Error: &responseError{Code: -32603, Message: err.Error()}}
	}
	return response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s Server) call(ctx context.Context, method string, params json.RawMessage) (any, error) {
	if s.Service == nil {
		return nil, fmt.Errorf("mcp service is not configured")
	}
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]string{"name": "nodebridge", "version": "0.33.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}, nil
	case "tools/list":
		return map[string]any{"tools": tools()}, nil
	case "tools/call":
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(params, &call); err != nil {
			return nil, err
		}
		value, err := s.callTool(ctx, call.Name, call.Arguments)
		if err != nil {
			return nil, err
		}
		data, _ := json.MarshalIndent(value, "", "  ")
		return map[string]any{"content": []map[string]string{{"type": "text", "text": string(data)}}}, nil
	default:
		return nil, fmt.Errorf("unsupported method %s", method)
	}
}

func (s Server) callTool(ctx context.Context, name string, args json.RawMessage) (any, error) {
	limit := limitArg(args)
	switch name {
	case "nodebridge_overview":
		return s.Service.Overview(ctx)
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
	default:
		return nil, fmt.Errorf("unsupported tool %s", name)
	}
}

func tools() []map[string]any {
	names := []string{
		"nodebridge_overview",
		"nodebridge_queue_status",
		"nodebridge_sync_rules",
		"nodebridge_failed_events",
		"nodebridge_logs",
		"nodebridge_diagnostic_summary",
	}
	result := make([]map[string]any, 0, len(names))
	for _, name := range names {
		result = append(result, map[string]any{
			"name":        name,
			"description": "Read-only NodeBridge diagnostic tool",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"limit": map[string]string{"type": "integer"}}},
		})
	}
	return result
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

func writeResponse(stdout io.Writer, resp response) {
	data, _ := json.Marshal(resp)
	fmt.Fprintln(stdout, string(data))
}
