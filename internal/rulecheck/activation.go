package rulecheck

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func CheckActivation(ctx context.Context, cfg appconfig.Config, previous, next rules.RuleSet) error {
	checks := []rules.SyncRule{}
	for _, rule := range next.Rules {
		if !rule.Enable || rule.Direction == rules.DirectionIgnore {
			continue
		}
		unchanged := false
		for _, old := range previous.Rules {
			if old.Enable && reflect.DeepEqual(old, rule) {
				unchanged = true
				break
			}
		}
		if !unchanged {
			checks = append(checks, rule)
		}
	}
	if len(checks) == 0 {
		return nil
	}
	if cfg.Mode != appconfig.ModeEdge && cfg.Mode != appconfig.ModeServer {
		return fmt.Errorf("rule_preflight_required: configure the node before enabling rules")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	db, err := mysqlconn.Open(cfg.MySQL)
	if err != nil {
		return fmt.Errorf("rule_preflight_required: %w", err)
	}
	defer db.Close()
	for _, rule := range checks {
		side := "source"
		if cfg.Mode == appconfig.ModeServer && rule.Direction == rules.DirectionEdgeToServer || cfg.Mode == appconfig.ModeEdge && rule.Direction == rules.DirectionServerToEdge {
			side = "target"
		}
		if cfg.Mode == appconfig.ModeEdge && side == "source" && len(rule.SourceNodeIDs) > 0 {
			local := false
			for _, node := range rule.SourceNodeIDs {
				if node == cfg.Node.ID {
					local = true
				}
			}
			if !local {
				continue
			}
		}
		if side == "target" && cfg.Mode == appconfig.ModeEdge {
			rule.TargetDatabaseName = rule.DownlinkTargetDatabase(cfg.MySQL.Database)
		}
		result, err := Check(ctx, db, rule, side)
		if err != nil {
			return err
		}
		if !result.OK {
			for _, finding := range result.Findings {
				if finding.Severity == "error" {
					return fmt.Errorf("rule_preflight_failed: %s (%s): %s: %s", rule.ID, side, finding.Code, finding.Message)
				}
			}
		}
	}
	return nil
}
