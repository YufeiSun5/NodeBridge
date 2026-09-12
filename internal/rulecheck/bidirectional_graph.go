package rulecheck

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// ObservedPair must contain the actual schemas from both named endpoints.
type ObservedPair struct {
	Rule       rules.SyncRule `json:"rule"`
	EdgeNode   string         `json:"edge_node_id"`
	ServerNode string         `json:"server_node_id"`
	Edge       Schema         `json:"edge_schema"`
	Server     Schema         `json:"server_schema"`
}

// EndpointRules separates local capture from incoming source-name projections.
// These projections do not authorize activation or replace live schema checks.
type EndpointRules struct {
	Capture  rules.RuleSet
	Incoming rules.RuleSet
}

type checkedPair struct {
	observation ObservedPair
	pair        BidirectionalPair
}

// BuildBidirectionalGraph composes edge-to-edge mappings through server names.
// It never rewrites events or assumes that different edges use the same schema.
func BuildBidirectionalGraph(observations []ObservedPair) (map[string]EndpointRules, error) {
	result := map[string]EndpointRules{}
	groups := map[string][]checkedPair{}
	serverNode := ""
	edgeNodes := map[string]bool{}
	for _, observation := range observations {
		if serverNode != "" && observation.ServerNode != serverNode {
			return nil, fmt.Errorf("bidirectional_single_server_required")
		}
		serverNode = observation.ServerNode
		edgeNodes[observation.EdgeNode] = true
		pair, err := BuildBidirectionalPair(observation.Rule, observation.EdgeNode, observation.ServerNode, observation.Edge, observation.Server)
		if err != nil {
			return nil, err
		}
		key := observation.Server.Database + "." + observation.Server.Table
		group := groups[key]
		for _, previous := range group {
			if previous.observation.EdgeNode == observation.EdgeNode {
				return nil, fmt.Errorf("bidirectional_ambiguous_edge_destination: %s", observation.EdgeNode)
			}
			if !reflect.DeepEqual(previous.observation.Server, observation.Server) {
				return nil, fmt.Errorf("bidirectional_server_schema_observations_disagree: %s", key)
			}
			if previous.observation.Rule.Enable != observation.Rule.Enable || previous.observation.Rule.EffectiveDeleteMode() != observation.Rule.EffectiveDeleteMode() {
				return nil, fmt.Errorf("bidirectional_endpoint_policies_disagree: %s", key)
			}
		}
		groups[key] = append(group, checkedPair{observation, pair})
	}
	if edgeNodes[serverNode] {
		return nil, fmt.Errorf("bidirectional_node_role_collision")
	}
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool { return group[i].observation.EdgeNode < group[j].observation.EdgeNode })
		targets := make([]string, 0, len(group))
		for _, item := range group {
			targets = append(targets, item.observation.EdgeNode)
		}
		capture := identityProjection(group[0].pair.Reverse)
		capture.DispatchNodeIDs = slices.Clone(targets)
		if err := addProjection(result, serverNode, true, capture); err != nil {
			return nil, err
		}
		for _, source := range group {
			if err := addProjection(result, source.observation.EdgeNode, true, source.pair.Forward); err != nil {
				return nil, err
			}
			if err := addProjection(result, source.observation.EdgeNode, false, source.pair.Reverse); err != nil {
				return nil, err
			}
			forward := cloneProjection(source.pair.Forward)
			forward.DispatchNodeIDs = nil
			for _, destination := range group {
				node := destination.observation.EdgeNode
				if node == source.observation.EdgeNode || !permitsRelay(source.observation.Rule, node) {
					continue
				}
				forward.DispatchNodeIDs = append(forward.DispatchNodeIDs, node)
				relay, err := composeRelay(source, destination)
				if err != nil {
					return nil, err
				}
				if err := addProjection(result, node, false, relay); err != nil {
					return nil, err
				}
			}
			if len(forward.DispatchNodeIDs) == 0 {
				forward.DispatchTarget = rules.DispatchNone
			}
			if err := addProjection(result, serverNode, false, forward); err != nil {
				return nil, err
			}
		}
	}
	for node, endpoint := range result {
		for _, set := range []*rules.RuleSet{&endpoint.Capture, &endpoint.Incoming} {
			sort.Slice(set.Rules, func(i, j int) bool { return set.Rules[i].ID < set.Rules[j].ID })
			if err := set.ValidateStructure(); err != nil {
				return nil, err
			}
		}
		result[node] = endpoint
	}
	return result, nil
}

func permitsRelay(rule rules.SyncRule, node string) bool {
	return rule.DispatchTarget != rules.DispatchSelectedEdges || slices.Contains(rule.DispatchNodeIDs, node)
}

func composeRelay(source, destination checkedPair) (rules.SyncRule, error) {
	rule := cloneProjection(source.pair.Forward)
	rule.TargetDatabaseName, rule.TargetTableName = destination.observation.Edge.Database, destination.observation.Edge.Table
	rule.TargetPrimaryKeys = slices.Clone(destination.observation.Edge.PrimaryKeys)
	inverse := map[string]string{}
	for _, column := range destination.pair.Reverse.ColumnMappings {
		inverse[column.SourceColumn] = column.TargetColumn
	}
	for i := range rule.ColumnMappings {
		rule.ColumnMappings[i].TargetColumn = inverse[rule.ColumnMappings[i].TargetColumn]
	}
	// BuildBidirectionalPair validates destination membership, so include both
	// endpoints only during composition, then restrict the actual receiver.
	rule.DispatchNodeIDs = []string{source.observation.EdgeNode, destination.observation.EdgeNode}
	pair, err := BuildBidirectionalPair(rule, source.observation.EdgeNode, destination.observation.EdgeNode, source.observation.Edge, destination.observation.Edge)
	if err != nil {
		return rules.SyncRule{}, err
	}
	pair.Forward.DispatchNodeIDs = []string{destination.observation.EdgeNode}
	return pair.Forward, nil
}

func identityProjection(rule rules.SyncRule) rules.SyncRule {
	rule = cloneProjection(rule)
	rule.TargetDatabaseName, rule.TargetTableName = rule.DatabaseName, rule.TableName
	rule.TargetPrimaryKeys = slices.Clone(rule.PrimaryKeys)
	rule.ColumnMappings = nil
	return rule
}

func cloneProjection(rule rules.SyncRule) rules.SyncRule {
	rule.SourceNodeIDs = slices.Clone(rule.SourceNodeIDs)
	rule.DispatchNodeIDs = slices.Clone(rule.DispatchNodeIDs)
	rule.PrimaryKeys = slices.Clone(rule.PrimaryKeys)
	rule.TargetPrimaryKeys = slices.Clone(rule.TargetPrimaryKeys)
	rule.IncludeColumns = slices.Clone(rule.IncludeColumns)
	rule.ExcludeColumns = slices.Clone(rule.ExcludeColumns)
	rule.ColumnMappings = slices.Clone(rule.ColumnMappings)
	return rule
}

func addProjection(result map[string]EndpointRules, node string, capture bool, rule rules.SyncRule) error {
	endpoint := result[node]
	set := &endpoint.Incoming
	if capture {
		set = &endpoint.Capture
	}
	for _, existing := range set.Rules {
		if existing.DatabaseName == rule.DatabaseName && existing.TableName == rule.TableName && slices.Equal(existing.SourceNodeIDs, rule.SourceNodeIDs) {
			return fmt.Errorf("bidirectional_ambiguous_projection: %s:%s.%s", node, rule.DatabaseName, rule.TableName)
		}
	}
	rule = cloneProjection(rule)
	rule.ID = fmt.Sprintf("bidi-%x", sha256.Sum256([]byte(fmt.Sprintf("%s|%t|%s|%s|%s", node, capture, rule.SourceNodeIDs[0], rule.DatabaseName, rule.TableName))))
	set.Rules = append(set.Rules, rule)
	result[node] = endpoint
	return nil
}
