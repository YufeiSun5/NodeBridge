package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type pairManifest struct {
	Version int                      `json:"version"`
	Pairs   []rulecheck.ObservedPair `json:"pairs"`
}

func readPairManifest(path string) ([]rulecheck.ObservedPair, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return nil, errors.New("bidirectional_manifest_size_invalid")
	}
	decoder := json.NewDecoder(io.LimitReader(f, 16<<20+1))
	decoder.DisallowUnknownFields()
	var manifest pairManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("bidirectional_manifest_trailing_data")
	}
	if manifest.Version != 1 || len(manifest.Pairs) == 0 || len(manifest.Pairs) > 256 {
		return nil, errors.New("bidirectional_manifest_version_or_count_invalid")
	}
	return manifest.Pairs, nil
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
