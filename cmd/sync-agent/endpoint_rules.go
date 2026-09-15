package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type pairManifest = rulecheck.PairManifest

func readPairManifest(path string) ([]rulecheck.ObservedPair, error) {
	return rulecheck.ReadPairManifest(path)
}

func resolveEndpointRules(ctx context.Context, cfg *appconfig.Config, set *rules.RuleSet, path string) (rulecheck.EndpointRules, error) {
	if cfg == nil || set == nil {
		return rulecheck.EndpointRules{}, errors.New("runtime_rules_required")
	}
	var observations []rulecheck.ObservedPair
	if needsConflictRuntime(set) {
		if path == "" {
			return rulecheck.EndpointRules{}, errors.New("bidirectional_pair_manifest_required")
		}
		var err error
		observations, err = readPairManifest(path)
		if err != nil {
			return rulecheck.EndpointRules{}, fmt.Errorf("read pair manifest: %w", err)
		}
		observations = activePairGroups(cfg.Node.ID, cfg.Mode, *set, observations)
	}
	compiled, err := rulecheck.CompileEndpoint(cfg.Node.ID, cfg.Mode, *set, observations)
	if err != nil {
		return rulecheck.EndpointRules{}, err
	}
	if len(observations) != 0 {
		db, err := openMySQL(cfg)
		if err != nil {
			return rulecheck.EndpointRules{}, err
		}
		defer db.Close()
		proofs, err := alignment.LoadActiveCutoversForTables(ctx, db, runtimeTableScopes(compiled))
		if err != nil {
			return rulecheck.EndpointRules{}, err
		}
		if err := alignment.VerifyObservedPairs(observations, proofs); err != nil {
			return rulecheck.EndpointRules{}, err
		}
		if err := rulecheck.VerifyLocalObservations(ctx, db, cfg.Node.ID, observations); err != nil {
			return rulecheck.EndpointRules{}, err
		}
		for _, projected := range []*rules.RuleSet{&compiled.Capture, &compiled.Incoming} {
			for i, rule := range projected.Rules {
				if rule.Direction == rules.DirectionBidirectional {
					projected.Rules[i] = rule.BindPairedRuntime()
				}
			}
		}
	}
	for _, projected := range []rules.RuleSet{compiled.Capture, compiled.Incoming} {
		if err := projected.Validate(); err != nil {
			return rulecheck.EndpointRules{}, err
		}
	}
	return compiled, nil
}

// A local enabled pair needs every remote mapping in its certified group.
func activePairGroups(node, mode string, set rules.RuleSet, pairs []rulecheck.ObservedPair) []rulecheck.ObservedPair {
	type group struct{ node, database, table string }
	key := func(pair rulecheck.ObservedPair) group {
		return group{pair.ServerNode, pair.Server.Database, pair.Server.Table}
	}
	enabled := map[string]bool{}
	for _, rule := range set.Rules {
		if rule.Enable && rule.Direction == rules.DirectionBidirectional {
			enabled[rule.ID] = true
		}
	}
	groups := map[group]bool{}
	for _, pair := range pairs {
		local := mode == "edge" && pair.EdgeNode == node || mode == "server" && pair.ServerNode == node
		if local && enabled[pair.Rule.ID] {
			groups[key(pair)] = true
		}
	}
	var active []rulecheck.ObservedPair
	for _, pair := range pairs {
		if groups[key(pair)] {
			active = append(active, pair)
		}
	}
	return active
}
