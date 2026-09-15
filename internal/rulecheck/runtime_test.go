package rulecheck_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"gopkg.in/yaml.v3"
)

func TestCompileEndpointYAMLAndJSONRoundTrip(t *testing.T) {
	set, observations := runtimeFixture()
	config, err := yaml.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(observations)
	if err != nil {
		t.Fatal(err)
	}
	var loaded rules.RuleSet
	var pairs []rulecheck.ObservedPair
	if err := yaml.Unmarshal(config, &loaded); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(manifest, &pairs); err != nil {
		t.Fatal(err)
	}
	if _, err := rulecheck.CompileEndpoint("edge-1", "edge", loaded, pairs); err != nil {
		t.Fatal(err)
	}
}

func runtimeFixture() (rules.RuleSet, []rulecheck.ObservedPair) {
	observations := graphFixture()
	var set rules.RuleSet
	for i := range observations {
		observations[i].Rule.Enable = true
		set.Rules = append(set.Rules, observations[i].Rule)
	}
	return set, observations
}

func TestCompileEndpointAllowsDisplayNameButRejectsIdentityChange(t *testing.T) {
	set, observations := runtimeFixture()
	for i := range set.Rules {
		set.Rules[i].Name = "中文规则名称"
	}
	if _, err := rulecheck.CompileEndpoint("edge-1", "edge", set, observations); err != nil {
		t.Fatal(err)
	}
	set.Rules[0].ID = "new-id"
	if _, err := rulecheck.CompileEndpoint("edge-1", "edge", set, observations); err == nil {
		t.Fatal("identity replacement bypassed alignment guard")
	}
}

func TestCompileEndpointUsesSavedEnableWithoutChangingManifest(t *testing.T) {
	set, observations := runtimeFixture()
	set.Rules, observations = set.Rules[:1], observations[:1]
	set.Rules[0].Enable = false
	compiled, err := rulecheck.CompileEndpoint("edge-1", "edge", set, observations)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range []rules.RuleSet{compiled.Capture, compiled.Incoming} {
		for _, rule := range group.Rules {
			if rule.Enable {
				t.Fatal("disabled rule remains active")
			}
		}
	}
	if !observations[0].Rule.Enable {
		t.Fatal("caller manifest mutated")
	}
	set.Rules[0].Enable = true
	compiled, err = rulecheck.CompileEndpoint("edge-1", "edge", set, observations)
	if err != nil || len(compiled.Capture.Rules) == 0 || !compiled.Capture.Rules[0].Enable {
		t.Fatal(compiled, err)
	}
	if compiled.Capture.Validate() == nil {
		t.Fatal("observation alone bypasses activation proof")
	}
}

func TestCompileEndpointLocalRulesWithCertifiedRemoteProjections(t *testing.T) {
	set, observations := runtimeFixture()
	set.Rules = set.Rules[:1]
	for _, enabled := range []bool{true, false} {
		set.Rules[0].Enable = enabled
		compiled, err := rulecheck.CompileEndpoint("edge-1", "edge", set, observations)
		if err != nil || len(compiled.Capture.Rules) != 1 || len(compiled.Incoming.Rules) != 2 {
			t.Fatal(compiled, err)
		}
		for _, rule := range append(compiled.Capture.Rules, compiled.Incoming.Rules...) {
			if rule.Enable != enabled {
				t.Fatal("remote projection does not follow local activation")
			}
		}
	}
	if _, err := rulecheck.CompileEndpoint("server-1", "server", set, observations); err == nil {
		t.Fatal("server accepted an unconfigured member")
	}
}

func TestCompileEndpointSplitsCaptureAndIncoming(t *testing.T) {
	set, observations := runtimeFixture()
	for _, endpoint := range []struct{ node, mode string }{{"edge-1", "edge"}, {"edge-2", "edge"}, {"server-1", "server"}} {
		compiled, err := rulecheck.CompileEndpoint(endpoint.node, endpoint.mode, set, observations)
		if err != nil {
			t.Fatal(err)
		}
		if len(compiled.Capture.Rules) != 1 || len(compiled.Incoming.Rules) != 2 {
			t.Fatalf("%s: %+v", endpoint.node, compiled)
		}
		if compiled.Capture.Rules[0].SourceNodeIDs[0] != endpoint.node {
			t.Fatal("capture is not local")
		}
		for _, rule := range compiled.Incoming.Rules {
			if rule.SourceNodeIDs[0] == endpoint.node {
				t.Fatal("incoming includes local capture")
			}
		}
		if compiled.Capture.Validate() == nil || compiled.Incoming.Validate() == nil {
			t.Fatal("compilation bypassed public activation gate")
		}
	}
}

func TestCompileEndpointSharedRuleAcrossSameSchemaEdges(t *testing.T) {
	_, observations := runtimeFixture()
	shared := observations[0].Rule
	shared.ID = "shared-rule"
	shared.SourceNodeIDs = []string{"edge-1", "edge-2"}
	observations[1].Edge = observations[0].Edge
	for i := range observations {
		observations[i].Rule = shared
	}
	for _, endpoint := range []struct{ node, mode string }{{"edge-1", "edge"}, {"edge-2", "edge"}, {"server-1", "server"}} {
		for _, enabled := range []bool{false, true} {
			shared.Enable = enabled
			compiled, err := rulecheck.CompileEndpoint(endpoint.node, endpoint.mode, rules.RuleSet{Rules: []rules.SyncRule{shared}}, observations)
			if err != nil || len(compiled.Capture.Rules) != 1 || len(compiled.Incoming.Rules) != 2 {
				t.Fatal(endpoint, compiled, err)
			}
			seen := map[string]bool{}
			for _, projection := range compiled.Incoming.Rules {
				if projection.Enable != enabled || seen[projection.SourceNodeIDs[0]] {
					t.Fatal("shared rule lost origin-specific projection")
				}
				seen[projection.SourceNodeIDs[0]] = true
			}
		}
	}
}

func TestCompileEndpointRejectsStaleOrMissingPairs(t *testing.T) {
	for _, change := range []func(*rules.RuleSet, *[]rulecheck.ObservedPair){
		func(_ *rules.RuleSet, p *[]rulecheck.ObservedPair) { *p = nil },
		func(_ *rules.RuleSet, p *[]rulecheck.ObservedPair) { (*p)[0].Rule.ID = "missing" },
		func(s *rules.RuleSet, _ *[]rulecheck.ObservedPair) { s.Rules[0].DispatchTarget = rules.DispatchNone },
		func(s *rules.RuleSet, _ *[]rulecheck.ObservedPair) { s.Rules[0].Enable = false },
	} {
		set, pairs := runtimeFixture()
		change(&set, &pairs)
		if _, err := rulecheck.CompileEndpoint("edge-1", "edge", set, pairs); err == nil {
			t.Fatal("accepted stale observations")
		}
	}
	set, pairs := runtimeFixture()
	for _, endpoint := range []struct{ node, mode string }{{"edge-1", "server"}, {"server-1", "edge"}, {"unknown", "edge"}, {"", "edge"}, {"edge-1", "other"}} {
		if _, err := rulecheck.CompileEndpoint(endpoint.node, endpoint.mode, set, pairs); err == nil {
			t.Fatalf("accepted %+v", endpoint)
		}
	}
}

func TestCompileEndpointPreservesOneWayAndRejectsOverlap(t *testing.T) {
	set, pairs := runtimeFixture()
	oneWay := rules.SyncRule{ID: "one-way", DatabaseName: "log_db", TableName: "logs", Direction: rules.DirectionEdgeToServer, PrimaryKeys: []string{"id"}, Enable: true, ConflictPolicy: rules.ConflictNone}
	set.Rules = append(set.Rules, oneWay)
	got, err := rulecheck.CompileEndpoint("edge-1", "edge", set, pairs)
	if err != nil {
		t.Fatal(err)
	}
	for _, compiled := range []rules.RuleSet{got.Capture, got.Incoming} {
		if !reflect.DeepEqual(compiled.Rules[len(compiled.Rules)-1], oneWay) {
			t.Fatal("one-way behavior changed")
		}
	}
	got.Capture.Rules[len(got.Capture.Rules)-1].PrimaryKeys[0] = "changed"
	if got.Incoming.Rules[len(got.Incoming.Rules)-1].PrimaryKeys[0] != "id" || oneWay.PrimaryKeys[0] != "id" {
		t.Fatal("aliased rule sets")
	}
	for _, sourceOverlap := range []bool{true, false} {
		rule := oneWay
		if sourceOverlap {
			rule.DatabaseName, rule.TableName = pairs[0].Edge.Database, pairs[0].Edge.Table
		} else {
			rule.TargetDatabaseName, rule.TargetTableName = pairs[0].Server.Database, pairs[0].Server.Table
		}
		set.Rules[len(set.Rules)-1] = rule
		if _, err := rulecheck.CompileEndpoint("edge-1", "edge", set, pairs); err == nil {
			t.Fatal("accepted overlapping one-way rule")
		}
	}
	plain := rules.RuleSet{Rules: []rules.SyncRule{oneWay}}
	got, err = rulecheck.CompileEndpoint("edge-1", "edge", plain, nil)
	if err != nil || !reflect.DeepEqual(got.Capture, plain) || !reflect.DeepEqual(got.Incoming, plain) {
		t.Fatalf("one-way regression: %+v %v", got, err)
	}
}
