package rulecheck

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// CompileEndpoint joins validated pair projections with existing one-way rules.
// Observations must describe the exact saved rules, not an earlier revision.
func CompileEndpoint(node, mode string, set rules.RuleSet, observations []ObservedPair) (EndpointRules, error) {
	var result EndpointRules
	if node == "" || (mode != "edge" && mode != "server") {
		return result, fmt.Errorf("bidirectional_endpoint_identity_invalid")
	}
	if err := set.ValidateStructure(); err != nil {
		return result, err
	}
	configured := map[string]rules.SyncRule{}
	for _, rule := range set.Rules {
		configured[rule.ID] = rule
	}
	observed := map[string]bool{}
	for _, pair := range observations {
		rule, ok := configured[pair.Rule.ID]
		if !ok || !reflect.DeepEqual(observedRule(rule), observedRule(pair.Rule)) {
			return result, fmt.Errorf("bidirectional_observation_rule_changed: %s", pair.Rule.ID)
		}
		if (pair.ServerNode == node && mode != "server") || (pair.EdgeNode == node && mode != "edge") {
			return result, fmt.Errorf("bidirectional_endpoint_role_mismatch")
		}
		observed[rule.ID] = true
	}
	graph, err := BuildBidirectionalGraph(observations)
	if err != nil {
		return result, err
	}
	result = graph[node]
	projections := append(slices.Clone(result.Capture.Rules), result.Incoming.Rules...)
	for _, rule := range set.Rules {
		if rule.Direction == rules.DirectionBidirectional {
			if rule.Enable && (!observed[rule.ID] || len(projections) == 0) {
				return EndpointRules{}, fmt.Errorf("bidirectional_local_pair_required: %s", rule.ID)
			}
			continue
		}
		for _, projection := range projections {
			if rulesOverlap(rule, projection) || (rule.Enable && projection.Enable && targetName(rule) == targetName(projection)) {
				return EndpointRules{}, fmt.Errorf("bidirectional_one_way_overlap: %s", rule.ID)
			}
		}
		result.Capture.Rules = append(result.Capture.Rules, cloneProjection(rule))
		result.Incoming.Rules = append(result.Incoming.Rules, cloneProjection(rule))
	}
	return result, nil
}

// YAML emits empty lists where JSON may preserve nil; both represent no entries.
func observedRule(rule rules.SyncRule) rules.SyncRule {
	rule.DeleteMode = rule.EffectiveDeleteMode()
	for _, values := range []*[]string{&rule.SourceNodeIDs, &rule.DispatchNodeIDs, &rule.PrimaryKeys, &rule.TargetPrimaryKeys, &rule.IncludeColumns, &rule.ExcludeColumns} {
		if len(*values) == 0 {
			*values = nil
		}
	}
	if len(rule.ColumnMappings) == 0 {
		rule.ColumnMappings = nil
	}
	return rule
}

func targetName(rule rules.SyncRule) string {
	database, table := rule.TargetDatabaseName, rule.TargetTableName
	if database == "" {
		database = rule.DatabaseName
	}
	if table == "" {
		table = rule.TableName
	}
	return database + "." + table
}

func rulesOverlap(a, b rules.SyncRule) bool {
	if !a.Enable || !b.Enable || a.DatabaseName != b.DatabaseName || a.TableName != b.TableName {
		return false
	}
	if len(a.SourceNodeIDs) == 0 || len(b.SourceNodeIDs) == 0 {
		return true
	}
	for _, node := range a.SourceNodeIDs {
		if slices.Contains(b.SourceNodeIDs, node) {
			return true
		}
	}
	return false
}

// VerifyLocalObservations always reads through the current node's own connection.
func VerifyLocalObservations(ctx context.Context, db SchemaReader, node string, observations []ObservedPair) error {
	if db == nil || node == "" {
		return fmt.Errorf("bidirectional_local_schema_reader_required")
	}
	checked := map[string]Schema{}
	for _, pair := range observations {
		var expected Schema
		switch node {
		case pair.EdgeNode:
			expected = pair.Edge
		case pair.ServerNode:
			expected = pair.Server
		default:
			continue
		}
		key := expected.Database + "." + expected.Table
		actual, ok := checked[key]
		if !ok {
			var err error
			actual, err = ReadSchema(ctx, db, expected.Database, expected.Table)
			if err != nil {
				return err
			}
			checked[key] = actual
		}
		if !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("bidirectional_local_schema_changed: %s", key)
		}
	}
	return nil
}
