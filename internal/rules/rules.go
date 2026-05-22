package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DirectionEdgeToServer  = "EDGE_TO_SERVER"
	DirectionBidirectional = "BIDIRECTIONAL"
	DirectionServerToEdge  = "SERVER_TO_EDGE"
	DirectionIgnore        = "IGNORE"

	DispatchAuto          = "AUTO"
	DispatchNone          = "NONE"
	DispatchActiveEdges   = "ACTIVE_EDGES"
	DispatchSelectedEdges = "SELECTED_EDGES"

	ConflictNone         = "NONE"
	ConflictServerWin    = "SERVER_WIN"
	ConflictLastWriteWin = "LAST_WRITE_WIN"
)

type SyncRule struct {
	ID                 string          `json:"id" yaml:"id"`
	DatabaseName       string          `json:"database_name" yaml:"database_name"`
	TableName          string          `json:"table_name" yaml:"table_name"`
	SourceNodeIDs      []string        `json:"source_node_ids,omitempty" yaml:"source_node_ids,omitempty"`
	TargetDatabaseName string          `json:"target_database_name,omitempty" yaml:"target_database_name,omitempty"`
	TargetTableName    string          `json:"target_table_name,omitempty" yaml:"target_table_name,omitempty"`
	Direction          string          `json:"direction" yaml:"direction"`
	DispatchTarget     string          `json:"dispatch_target,omitempty" yaml:"dispatch_target,omitempty"`
	DispatchNodeIDs    []string        `json:"dispatch_node_ids,omitempty" yaml:"dispatch_node_ids,omitempty"`
	ConflictPolicy     string          `json:"conflict_policy" yaml:"conflict_policy"`
	Enable             bool            `json:"enable" yaml:"enable"`
	PrimaryKeys        []string        `json:"primary_keys" yaml:"primary_keys"`
	TargetPrimaryKeys  []string        `json:"target_primary_keys,omitempty" yaml:"target_primary_keys,omitempty"`
	IncludeColumns     []string        `json:"include_columns" yaml:"include_columns"`
	ExcludeColumns     []string        `json:"exclude_columns" yaml:"exclude_columns"`
	ColumnMappings     []ColumnMapping `json:"column_mappings,omitempty" yaml:"column_mappings,omitempty"`
}

type ColumnMapping struct {
	SourceColumn string `json:"source_column" yaml:"source_column"`
	TargetColumn string `json:"target_column" yaml:"target_column"`
}

type RuleSet struct {
	Rules []SyncRule `json:"rules" yaml:"rules"`
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var nodeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func DefaultRuleSet() *RuleSet {
	return &RuleSet{Rules: []SyncRule{
		{
			ID:                 "alarm-history-upload",
			DatabaseName:       "scada_edge",
			TableName:          "alarm_history",
			TargetDatabaseName: "scada_center",
			Direction:          DirectionEdgeToServer,
			ConflictPolicy:     ConflictNone,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
		},
		{
			ID:                 "device-config-remap",
			DatabaseName:       "scada_edge",
			TableName:          "device_config",
			TargetDatabaseName: "scada_center",
			TargetTableName:    "device_settings",
			Direction:          DirectionBidirectional,
			ConflictPolicy:     ConflictLastWriteWin,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
			TargetPrimaryKeys:  []string{"setting_id"},
			IncludeColumns: []string{
				"id",
				"name",
				"value",
				"sync_version",
				"updated_by_node",
				"last_event_id",
				"updated_at",
			},
			ColumnMappings: []ColumnMapping{
				{SourceColumn: "id", TargetColumn: "setting_id"},
				{SourceColumn: "name", TargetColumn: "display_name"},
				{SourceColumn: "value", TargetColumn: "setting_value"},
			},
		},
	}}
}

func DefaultFieldRuleSet() *RuleSet {
	fieldRules := []SyncRule{
		{
			ID:                 "device-config-bidirectional",
			DatabaseName:       "scada_edge",
			TableName:          "device_config",
			TargetDatabaseName: "scada_center",
			TargetTableName:    "device_config",
			Direction:          DirectionBidirectional,
			DispatchTarget:     DispatchActiveEdges,
			ConflictPolicy:     ConflictLastWriteWin,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
		},
		{
			ID:                 "point-config-bidirectional",
			DatabaseName:       "scada_edge",
			TableName:          "point_config",
			TargetDatabaseName: "scada_center",
			TargetTableName:    "point_config",
			Direction:          DirectionBidirectional,
			DispatchTarget:     DispatchActiveEdges,
			ConflictPolicy:     ConflictLastWriteWin,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
		},
		{
			ID:                 "server-device-config-downlink",
			DatabaseName:       "scada_center",
			TableName:          "device_config",
			TargetDatabaseName: "scada_edge",
			TargetTableName:    "device_config",
			Direction:          DirectionServerToEdge,
			DispatchTarget:     DispatchActiveEdges,
			ConflictPolicy:     ConflictLastWriteWin,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
		},
		{
			ID:                 "server-point-config-downlink",
			DatabaseName:       "scada_center",
			TableName:          "point_config",
			TargetDatabaseName: "scada_edge",
			TargetTableName:    "point_config",
			Direction:          DirectionServerToEdge,
			DispatchTarget:     DispatchActiveEdges,
			ConflictPolicy:     ConflictLastWriteWin,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
		},
	}
	for i := 1; i <= 10; i++ {
		nodeID := fmt.Sprintf("edge-%03d", i)
		fieldRules = append(fieldRules, SyncRule{
			ID:                 fmt.Sprintf("data-all-%s", nodeID),
			DatabaseName:       "scada_edge",
			TableName:          "data_all",
			SourceNodeIDs:      []string{nodeID},
			TargetDatabaseName: "scada_center",
			TargetTableName:    fmt.Sprintf("data_all_%s", strings.ReplaceAll(nodeID, "-", "_")),
			Direction:          DirectionEdgeToServer,
			DispatchTarget:     DispatchNone,
			ConflictPolicy:     ConflictNone,
			Enable:             true,
			PrimaryKeys:        []string{"id"},
		})
	}
	return &RuleSet{Rules: fieldRules}
}

func LoadFile(path string) (*RuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules %q: %w", path, err)
	}

	var set RuleSet
	if err := yaml.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("parse rules %q: %w", path, err)
	}
	return &set, nil
}

func SaveFile(path string, set RuleSet) error {
	if err := set.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(set)
	if err != nil {
		return fmt.Errorf("marshal rules: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create rules directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write rules %q: %w", path, err)
	}
	return nil
}

func (s RuleSet) Find(databaseName, tableName string) *SyncRule {
	for i := range s.Rules {
		rule := &s.Rules[i]
		if len(rule.SourceNodeIDs) == 0 && rule.DatabaseName == databaseName && rule.TableName == tableName {
			return rule
		}
	}
	return nil
}

func (s RuleSet) FindForNode(databaseName, tableName, originNodeID, sourceNodeID string) *SyncRule {
	var fallback *SyncRule
	for i := range s.Rules {
		rule := &s.Rules[i]
		if rule.DatabaseName != databaseName || rule.TableName != tableName {
			continue
		}
		if len(rule.SourceNodeIDs) == 0 {
			if fallback == nil {
				fallback = rule
			}
			continue
		}
		if rule.matchesNode(originNodeID, sourceNodeID) {
			return rule
		}
	}
	return fallback
}

func (s RuleSet) Validate() error {
	seen := map[string]bool{}
	for _, rule := range s.Rules {
		baseKey := rule.DatabaseName + "." + rule.TableName
		if baseKey == "." {
			return fmt.Errorf("rule database_name and table_name are required")
		}
		for _, scope := range ruleScopes(rule.SourceNodeIDs) {
			key := baseKey + "@" + scope
			if seen[key] {
				return fmt.Errorf("duplicate rule for %s", key)
			}
			seen[key] = true
		}
		if err := validateIdentifier(rule.DatabaseName); err != nil {
			return err
		}
		if err := validateIdentifier(rule.TableName); err != nil {
			return err
		}
		for _, nodeID := range rule.SourceNodeIDs {
			if err := validateNodeID(nodeID); err != nil {
				return err
			}
		}
		switch rule.DispatchTarget {
		case "", DispatchAuto, DispatchNone, DispatchActiveEdges, DispatchSelectedEdges:
		default:
			return fmt.Errorf("invalid dispatch_target %q for %s", rule.DispatchTarget, baseKey)
		}
		if rule.DispatchTarget == DispatchSelectedEdges && len(rule.DispatchNodeIDs) == 0 {
			return fmt.Errorf("dispatch_node_ids are required for selected dispatch on %s", baseKey)
		}
		for _, nodeID := range rule.DispatchNodeIDs {
			if err := validateNodeID(nodeID); err != nil {
				return err
			}
		}
		if rule.TargetDatabaseName != "" {
			if err := validateIdentifier(rule.TargetDatabaseName); err != nil {
				return err
			}
		}
		if rule.TargetTableName != "" {
			if err := validateIdentifier(rule.TargetTableName); err != nil {
				return err
			}
		}
		if len(rule.TargetPrimaryKeys) > 0 && len(rule.PrimaryKeys) != len(rule.TargetPrimaryKeys) {
			return fmt.Errorf("source and target primary key count mismatch for %s", baseKey)
		}
		for _, column := range append(append([]string{}, rule.PrimaryKeys...), rule.TargetPrimaryKeys...) {
			if err := validateIdentifier(column); err != nil {
				return err
			}
		}
		targetColumns := map[string]string{}
		for _, mapping := range rule.ColumnMappings {
			if err := validateIdentifier(mapping.SourceColumn); err != nil {
				return err
			}
			if err := validateIdentifier(mapping.TargetColumn); err != nil {
				return err
			}
			if previous, ok := targetColumns[mapping.TargetColumn]; ok && previous != mapping.SourceColumn {
				return fmt.Errorf("target column %s is mapped more than once", mapping.TargetColumn)
			}
			targetColumns[mapping.TargetColumn] = mapping.SourceColumn
		}
	}
	return nil
}

func (r SyncRule) matchesNode(originNodeID, sourceNodeID string) bool {
	for _, nodeID := range r.SourceNodeIDs {
		if nodeID == originNodeID || nodeID == sourceNodeID {
			return true
		}
	}
	return false
}

func ruleScopes(sourceNodeIDs []string) []string {
	if len(sourceNodeIDs) == 0 {
		return []string{""}
	}
	return sourceNodeIDs
}

func validateIdentifier(value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("invalid identifier %q", value)
	}
	return nil
}

func validateNodeID(value string) error {
	if !nodeIDPattern.MatchString(value) {
		return fmt.Errorf("invalid source node id %q", value)
	}
	return nil
}
