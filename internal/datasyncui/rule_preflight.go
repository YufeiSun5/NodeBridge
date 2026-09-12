package datasyncui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type RulePreflightRequest struct {
	RuleID       string            `json:"rule_id"`
	Side         string            `json:"side"`
	SourceSchema *rulecheck.Schema `json:"source_schema,omitempty"`
}

func (a *App) checkRuleActivation(ctx context.Context, next rules.RuleSet) error {
	previous := rules.RuleSet{}
	if a.rulesPath != "" {
		loaded, err := rules.LoadFile(a.rulesPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if loaded != nil {
			previous = *loaded
		}
	} else if a.ruleSet != nil {
		previous = *a.ruleSet
	}
	cfg := appconfig.Config{}
	if a.config != nil {
		cfg = *a.config
	}
	return rulecheck.CheckActivation(ctx, cfg, previous, next)
}

func findRuleByID(set *rules.RuleSet, id string) (rules.SyncRule, error) {
	if id == "" {
		return rules.SyncRule{}, fmt.Errorf("rule_id is required")
	}
	var found *rules.SyncRule
	if set != nil {
		for i := range set.Rules {
			if set.Rules[i].ID == id {
				if found != nil {
					return rules.SyncRule{}, fmt.Errorf("rule_id %q is ambiguous", id)
				}
				found = &set.Rules[i]
			}
		}
	}
	if found == nil {
		return rules.SyncRule{}, fmt.Errorf("unknown rule_id %q", id)
	}
	return *found, nil
}

func (a *App) PreflightSyncRule(req RulePreflightRequest) (rulecheck.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.preflightSyncRule(ctx, req)
}

func (a *App) preflightSyncRule(ctx context.Context, req RulePreflightRequest) (rulecheck.Result, error) {
	if req.Side != "source" && req.Side != "target" {
		return rulecheck.Result{}, fmt.Errorf("side must be source or target")
	}
	set, err := rules.LoadFile(a.effectiveRulesPath())
	if err != nil {
		return rulecheck.Result{}, err
	}
	rule, err := findRuleByID(set, req.RuleID)
	if err != nil {
		return rulecheck.Result{}, err
	}
	if a.config == nil {
		return rulecheck.Result{}, errConfigMissing
	}
	db, err := mysqlconn.Open(a.config.MySQL)
	if err != nil {
		return rulecheck.Result{}, err
	}
	defer db.Close()
	if req.Side == "target" && a.config.Mode == appconfig.ModeEdge {
		rule.TargetDatabaseName = rule.DownlinkTargetDatabase(a.config.MySQL.Database)
	}
	result, err := rulecheck.Check(ctx, db, rule, req.Side)
	if err != nil {
		return result, err
	}
	if req.SourceSchema != nil {
		if req.Side != "target" {
			return result, fmt.Errorf("source_schema is only valid for target preflight")
		}
		if req.SourceSchema.Database != rule.DatabaseName || req.SourceSchema.Table != rule.TableName {
			return result, fmt.Errorf("source_schema must describe this rule's source table")
		}
		checks := append(rulecheck.ValidateLocal(rule, *req.SourceSchema, "source"), rulecheck.ValidatePair(rule, *req.SourceSchema, result.Schema)...)
		for _, finding := range checks {
			if finding.Severity == "error" {
				result.OK = false
			}
		}
		result.Findings = append(result.Findings, checks...)
		result.Unverified = append(result.Unverified, "provided_source_schema_freshness")
	}
	return result, nil
}
