package mcpstdio

import (
	"context"
	"encoding/json"
)

type rpcError struct {
	code    int
	message string
}

func (e *rpcError) Error() string { return e.message }

func UnknownTool(name string) error { return &rpcError{-32602, "unsupported tool " + name} }

func validID(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	switch value.(type) {
	case string, float64:
		return true
	default:
		return false
	}
}

func BaseTools() []map[string]any { return tools() }

func (s StaticService) RecordAudit(action string, success bool) {
	s.audit(action, map[string]any{"success": success})
}

func (s StaticService) RecordAuditDetails(action string, detail map[string]any) {
	s.audit(action, detail)
}

func (s Server) CallTool(ctx context.Context, name string, args json.RawMessage) (any, error) {
	return s.callTool(ctx, name, args)
}
