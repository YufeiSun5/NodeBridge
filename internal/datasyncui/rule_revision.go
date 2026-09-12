package datasyncui

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
)

func (s *MCPService) SyncRules(context.Context) (any, error) { return s.app.GetSyncRules() }

func (s *MCPService) SaveSyncRules(ctx context.Context, args json.RawMessage) (any, error) {
	var req uiapi.SaveSyncRulesRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	if req.Rules != nil {
		set := rules.RuleSet{Rules: req.Rules}
		if err := set.Validate(); err != nil {
			return nil, err
		}
		if err := s.app.checkRuleActivation(ctx, set); err != nil {
			return nil, err
		}
	}
	return s.StaticService.SaveSyncRules(ctx, args)
}

func (a *App) rulesDTO(items []rules.SyncRule, revision string) uiapi.SyncRulesDTO {
	dto := uiapi.SyncRulesDTO{Rules: append([]rules.SyncRule(nil), items...), SavedRevision: revision, Activation: "stopped"}
	state, err := agentstate.Read(a.configPath)
	if err != nil {
		dto.Activation = "unknown"
		return dto
	}
	if state == nil {
		return dto
	}
	wanted, _ := filepath.Abs(a.effectiveRulesPath())
	if !strings.EqualFold(filepath.Clean(state.RulesPath), filepath.Clean(wanted)) || state.RulesRevision == "" {
		dto.Activation = "unknown"
		return dto
	}
	dto.ActiveRevision = state.RulesRevision
	dto.Activation = "active"
	if revision != state.RulesRevision {
		dto.Activation = "restart_required"
	}
	return dto
}
