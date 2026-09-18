package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/syncruntime"
)

func loadRuntimeCutover(ctx context.Context, db *sql.DB, node string, endpoints ...rulecheck.EndpointRules) (*alignment.CutoverFilter, error) {
	var proofs []alignment.CutoverProof
	var err error
	if len(endpoints) == 0 {
		proofs, err = alignment.LoadActiveCutovers(ctx, db)
	} else {
		proofs, err = alignment.LoadActiveCutoversForTables(ctx, db, runtimeTableScopes(endpoints[0]))
	}
	if err != nil {
		return nil, err
	}
	filter, err := alignment.NewCutoverFilter(node, db, proofs)
	if err != nil {
		return nil, err
	}
	if err := alignment.LoadGenerationFilter(ctx, filter); err != nil {
		return nil, err
	}
	return filter, nil
}

func runtimeTableScopes(endpoint rulecheck.EndpointRules) []alignment.TableScope {
	var tables []alignment.TableScope
	for _, rule := range endpoint.Capture.Rules {
		if rule.Enable && rule.Direction != rules.DirectionIgnore {
			tables = append(tables, alignment.TableScope{Database: rule.DatabaseName, Table: rule.TableName})
		}
	}
	for _, rule := range endpoint.Incoming.Rules {
		if !rule.Enable || rule.Direction == rules.DirectionIgnore {
			continue
		}
		database, table := rule.TargetDatabaseName, rule.TargetTableName
		if database == "" {
			database = rule.DatabaseName
		}
		if table == "" {
			table = rule.TableName
		}
		tables = append(tables, alignment.TableScope{Database: database, Table: table})
	}
	return tables
}

func cutoverIncoming(source syncruntime.BatchMessageSource, filter *alignment.CutoverFilter) syncruntime.BatchMessageSource {
	if len(filter.Proofs) == 0 {
		return source
	}
	return alignment.CutoverMessages{Source: source, Filter: filter}
}

func enabledCutovers(filter *alignment.CutoverFilter, endpoint rulecheck.EndpointRules) *alignment.CutoverFilter {
	result := *filter
	result.Proofs = nil
	enabledGroups := map[string]bool{}
	for _, proof := range filter.Proofs {
		local, _ := proof.Local(filter.NodeID)
		enabled := false
		for _, rule := range endpoint.Capture.Rules {
			if rule.Enable && rule.Direction != rules.DirectionIgnore && rule.DatabaseName == local.DatabaseName && rule.TableName == local.TableName {
				enabled = true
			}
		}
		for _, rule := range endpoint.Incoming.Rules {
			database, table := rule.TargetDatabaseName, rule.TargetTableName
			if database == "" {
				database = rule.DatabaseName
			}
			if table == "" {
				table = rule.TableName
			}
			if rule.Enable && rule.Direction != rules.DirectionIgnore && database == local.DatabaseName && table == local.TableName {
				enabled = true
			}
		}
		if enabled {
			pair := proof.ObservedPair()
			enabledGroups[pair.ServerNode+":"+pair.Server.Database+"."+pair.Server.Table] = true
		}
	}
	for _, proof := range filter.Proofs {
		pair := proof.ObservedPair()
		if enabledGroups[pair.ServerNode+":"+pair.Server.Database+"."+pair.Server.Table] {
			result.Proofs = append(result.Proofs, proof)
		}
	}
	return &result
}

func cutoverCapture(source syncruntime.CanalBatchSource, normalizer syncruntime.ChangeNormalizer, filter *alignment.CutoverFilter) (syncruntime.CanalBatchSource, syncruntime.ChangeNormalizer) {
	if len(filter.Proofs) == 0 {
		return source, normalizer
	}
	return &alignment.CutoverSource{Source: source, Filter: filter}, alignment.CutoverNormalizer{Normalizer: normalizer, Filter: filter}
}

func verifyCutoverRules(ctx context.Context, cfg *appconfig.Config, set *rules.RuleSet, endpoints ...rulecheck.EndpointRules) error {
	db, err := openMySQL(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	filter, err := loadRuntimeCutover(ctx, db, cfg.Node.ID, endpoints...)
	if err != nil {
		return err
	}
	return requireRuntimeCutovers(cfg.Node.ID, set, filter.Proofs)
}

func requireRuntimeCutovers(node string, set *rules.RuleSet, proofs []alignment.CutoverProof) error {
	for _, proof := range proofs {
		if _, err := proof.Local(node); err != nil {
			continue
		}
		found := false
		for _, rule := range set.Rules {
			if rule.ID == proof.Rule.ID && proof.MatchesRule(rule) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("alignment_cutover_rule_changed: %s", proof.Rule.ID)
		}
	}
	for _, rule := range set.Rules {
		if !rule.Enable || rule.Direction != rules.DirectionBidirectional {
			continue
		}
		found := false
		for _, proof := range proofs {
			if _, err := proof.Local(node); err == nil && proof.MatchesRule(rule) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("alignment_active_proof_required: %s", rule.ID)
		}
	}
	return nil
}
