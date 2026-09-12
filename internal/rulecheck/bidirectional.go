package rulecheck

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// BidirectionalPair contains per-endpoint projections, not activation permission.
type BidirectionalPair struct {
	Forward rules.SyncRule
	Reverse rules.SyncRule
	Relay   rules.SyncRule
}

// BuildBidirectionalPair requires observed schemas. It performs no IO or writes.
// Forward uses edge names, Reverse uses server names, Relay retains edge names.
func BuildBidirectionalPair(rule rules.SyncRule, edgeNode, serverNode string, edge, server Schema) (BidirectionalPair, error) {
	var empty BidirectionalPair
	if err := (rules.RuleSet{Rules: []rules.SyncRule{rule}}).ValidateStructure(); err != nil {
		return empty, err
	}
	if rule.ID == "" || rule.Direction != rules.DirectionBidirectional || rule.ConflictPolicy != rules.ConflictLastWriteWin {
		return empty, fmt.Errorf("bidirectional_lww_rule_required")
	}
	if edgeNode == "" || serverNode == "" || edgeNode == serverNode {
		return empty, fmt.Errorf("bidirectional_distinct_nodes_required")
	}
	if len(rule.SourceNodeIDs) > 0 && !slices.Contains(rule.SourceNodeIDs, edgeNode) {
		return empty, fmt.Errorf("bidirectional_edge_outside_source_scope")
	}
	if rule.DispatchTarget == rules.DispatchNone || rule.DispatchTarget == rules.DispatchSelectedEdges && !slices.Contains(rule.DispatchNodeIDs, edgeNode) {
		return empty, fmt.Errorf("bidirectional_edge_outside_dispatch_scope")
	}
	if rule.SyncMode != "" && rule.SyncMode != rules.SyncModeOrderedCRUD || rule.SchemaSync.AddColumns || rule.SchemaSync.DropColumns {
		return empty, fmt.Errorf("bidirectional_ordered_crud_without_ddl_required")
	}
	targetDB, targetTable := rule.TargetDatabaseName, rule.TargetTableName
	if targetDB == "" {
		targetDB = rule.DatabaseName
	}
	if targetTable == "" {
		targetTable = rule.TableName
	}
	if edge.Database != rule.DatabaseName || edge.Table != rule.TableName || server.Database != targetDB || server.Table != targetTable {
		return empty, fmt.Errorf("bidirectional_schema_scope_mismatch")
	}
	if !strings.EqualFold(edge.Engine, "InnoDB") || !strings.EqualFold(server.Engine, "InnoDB") {
		return empty, fmt.Errorf("bidirectional_innodb_required")
	}
	mapping := map[string]string{}
	for _, column := range rule.ColumnMappings {
		mapping[column.SourceColumn] = column.TargetColumn
	}
	for i, key := range rule.PrimaryKeys {
		if len(rule.TargetPrimaryKeys) > 0 {
			mapping[key] = rule.TargetPrimaryKeys[i]
		}
	}
	keys := make([]string, len(rule.PrimaryKeys))
	for i, key := range rule.PrimaryKeys {
		keys[i] = key
		if name := mapping[key]; name != "" {
			keys[i] = name
		}
	}
	if !reflect.DeepEqual(rule.PrimaryKeys, edge.PrimaryKeys) || !reflect.DeepEqual(keys, server.PrimaryKeys) {
		return empty, fmt.Errorf("bidirectional_actual_primary_key_mismatch")
	}
	targetColumns := map[string]Column{}
	for _, column := range server.Columns {
		if _, exists := targetColumns[column.Name]; exists {
			return empty, fmt.Errorf("bidirectional_duplicate_schema_column")
		}
		targetColumns[column.Name] = column
	}
	owners, sources := map[string]bool{}, map[string]bool{}
	writable := map[string]bool{}
	forwardColumns, reverseColumns := []rules.ColumnMapping{}, []rules.ColumnMapping{}
	for _, column := range edge.Columns {
		if sources[column.Name] {
			return empty, fmt.Errorf("bidirectional_duplicate_schema_column")
		}
		sources[column.Name] = true
		if column.Generated() {
			continue
		}
		writable[column.Name] = true
		if len(rule.IncludeColumns) > 0 && !slices.Contains(rule.IncludeColumns, column.Name) || slices.Contains(rule.ExcludeColumns, column.Name) {
			return empty, fmt.Errorf("bidirectional_full_writable_row_required: %s", column.Name)
		}
		name := column.Name
		if mapped := mapping[name]; mapped != "" {
			name = mapped
		}
		target, ok := targetColumns[name]
		if !ok || target.Generated() {
			return empty, fmt.Errorf("bidirectional_target_column_unavailable: %s", name)
		}
		if owners[name] {
			return empty, fmt.Errorf("bidirectional_mapping_collision: %s", name)
		}
		if !strings.EqualFold(column.Type, target.Type) || column.Collation != target.Collation || column.Nullable != target.Nullable {
			return empty, fmt.Errorf("bidirectional_incompatible_column: %s", column.Name)
		}
		owners[name] = true
		forwardColumns = append(forwardColumns, rules.ColumnMapping{SourceColumn: column.Name, TargetColumn: name})
		reverseColumns = append(reverseColumns, rules.ColumnMapping{SourceColumn: name, TargetColumn: column.Name})
	}
	for _, column := range server.Columns {
		if !column.Generated() && !owners[column.Name] {
			return empty, fmt.Errorf("bidirectional_unmapped_target_column: %s", column.Name)
		}
	}
	for name := range mapping {
		if !writable[name] {
			return empty, fmt.Errorf("bidirectional_mapping_source_unavailable: %s", name)
		}
	}
	for _, name := range edge.PrimaryKeys {
		if !writable[name] {
			return empty, fmt.Errorf("bidirectional_writable_primary_key_required: %s", name)
		}
	}
	for _, name := range append(slices.Clone(rule.IncludeColumns), rule.ExcludeColumns...) {
		if !sources[name] {
			return empty, fmt.Errorf("bidirectional_selected_column_unavailable: %s", name)
		}
	}
	// The current replay protocol owns these names on both endpoints.
	for _, name := range []string{"last_event_id", "updated_by_node"} {
		if !writable[name] || !owners[name] || mapping[name] != "" && mapping[name] != name {
			return empty, fmt.Errorf("bidirectional_canonical_replay_columns_required: %s", name)
		}
	}
	forward := rule
	forward.SourceNodeIDs = []string{edgeNode}
	forward.TargetDatabaseName, forward.TargetTableName = server.Database, server.Table
	forward.PrimaryKeys, forward.TargetPrimaryKeys = slices.Clone(edge.PrimaryKeys), slices.Clone(server.PrimaryKeys)
	forward.IncludeColumns, forward.ExcludeColumns = nil, nil
	for _, column := range forwardColumns {
		forward.IncludeColumns = append(forward.IncludeColumns, column.SourceColumn)
	}
	forward.ColumnMappings = forwardColumns
	forward.DispatchTarget, forward.DispatchNodeIDs = rules.DispatchSelectedEdges, []string{edgeNode}
	reverse := forward
	reverse.DatabaseName, reverse.TableName = server.Database, server.Table
	reverse.TargetDatabaseName, reverse.TargetTableName = edge.Database, edge.Table
	reverse.SourceNodeIDs = []string{serverNode}
	reverse.PrimaryKeys, reverse.TargetPrimaryKeys = slices.Clone(server.PrimaryKeys), slices.Clone(edge.PrimaryKeys)
	reverse.ColumnMappings = reverseColumns
	reverse.IncludeColumns = nil
	for _, column := range reverseColumns {
		reverse.IncludeColumns = append(reverse.IncludeColumns, column.SourceColumn)
	}
	reverse.DispatchNodeIDs = []string{edgeNode}
	relay := forward
	relay.TargetDatabaseName, relay.TargetTableName = edge.Database, edge.Table
	relay.SourceNodeIDs = []string{edgeNode}
	relay.PrimaryKeys, relay.TargetPrimaryKeys = slices.Clone(edge.PrimaryKeys), slices.Clone(edge.PrimaryKeys)
	relay.ColumnMappings = nil
	relay.IncludeColumns = slices.Clone(forward.IncludeColumns)
	relay.DispatchNodeIDs = []string{edgeNode}
	for _, projected := range []rules.SyncRule{forward, reverse, relay} {
		if err := (rules.RuleSet{Rules: []rules.SyncRule{projected}}).ValidateStructure(); err != nil {
			return empty, err
		}
	}
	for _, endpoint := range []struct {
		rule   rules.SyncRule
		schema Schema
	}{{forward, server}, {reverse, edge}} {
		for _, finding := range ValidateLocal(endpoint.rule, endpoint.schema, "target") {
			if finding.Severity == "error" {
				return empty, fmt.Errorf("bidirectional_schema_invalid: %s: %s", finding.Code, finding.Message)
			}
		}
	}
	return BidirectionalPair{Forward: forward, Reverse: reverse, Relay: relay}, nil
}
