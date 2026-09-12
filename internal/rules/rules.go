package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/atomicfile"
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

	SyncModeOrderedCRUD = "crud_ordered"
	SyncModeAppendOnly  = "append_only"
	SyncModeCRUDCompact = "crud_ordered_compact"
	AlignmentDisabled   = "DISABLED"
	AlignmentManual     = "MANUAL"
	DeleteSoft          = "SOFT"
	DeleteHard          = "HARD"
)

type SyncRule struct {
	pairedRuntime      bool
	ID                 string           `json:"id" yaml:"id"`
	DatabaseName       string           `json:"database_name" yaml:"database_name"`
	TableName          string           `json:"table_name" yaml:"table_name"`
	SourceNodeIDs      []string         `json:"source_node_ids,omitempty" yaml:"source_node_ids,omitempty"`
	TargetDatabaseName string           `json:"target_database_name,omitempty" yaml:"target_database_name,omitempty"`
	TargetTableName    string           `json:"target_table_name,omitempty" yaml:"target_table_name,omitempty"`
	Direction          string           `json:"direction" yaml:"direction"`
	DispatchTarget     string           `json:"dispatch_target,omitempty" yaml:"dispatch_target,omitempty"`
	DispatchNodeIDs    []string         `json:"dispatch_node_ids,omitempty" yaml:"dispatch_node_ids,omitempty"`
	SyncMode           string           `json:"sync_mode,omitempty" yaml:"sync_mode,omitempty"`
	ConflictPolicy     string           `json:"conflict_policy" yaml:"conflict_policy"`
	Enable             bool             `json:"enable" yaml:"enable"`
	PrimaryKeys        []string         `json:"primary_keys" yaml:"primary_keys"`
	TargetPrimaryKeys  []string         `json:"target_primary_keys,omitempty" yaml:"target_primary_keys,omitempty"`
	IncludeColumns     []string         `json:"include_columns" yaml:"include_columns"`
	ExcludeColumns     []string         `json:"exclude_columns" yaml:"exclude_columns"`
	ColumnMappings     []ColumnMapping  `json:"column_mappings,omitempty" yaml:"column_mappings,omitempty"`
	SchemaSync         SchemaSync       `json:"schema_sync,omitempty" yaml:"schema_sync,omitempty"`
	InitialAlignment   InitialAlignment `json:"initial_alignment,omitempty" yaml:"initial_alignment,omitempty"`
	DeleteMode         string           `json:"delete_mode,omitempty" yaml:"delete_mode,omitempty"`
}

func (r SyncRule) EffectiveDeleteMode() string {
	if r.DeleteMode == "" {
		return DeleteSoft
	}
	return r.DeleteMode
}

func (r SyncRule) ValidateRuntimePolicy() error {
	if r.pairedRuntime && r.Direction == DirectionBidirectional && r.ConflictPolicy == ConflictLastWriteWin && (r.SyncMode == "" || r.SyncMode == SyncModeOrderedCRUD) && !r.SchemaSync.AddColumns && !r.SchemaSync.DropColumns {
		return nil
	}
	if r.ConflictPolicy != "" && r.ConflictPolicy != ConflictNone {
		return fmt.Errorf("unsupported_conflict_policy: %s has no implemented arbitration for rule %q", r.ConflictPolicy, r.ID)
	}
	if r.Direction == DirectionBidirectional {
		return fmt.Errorf("unsupported_bidirectional: multi-writer replay and deletion safety is not verified for rule %q", r.ID)
	}
	return nil
}

// BindPairedRuntime is only for the Agent after endpoint compilation and live
// local schema verification. The binding is never serialized into configuration.
func (r SyncRule) BindPairedRuntime() SyncRule {
	r.pairedRuntime = true
	return r
}

type InitialAlignment struct {
	Policy string `json:"policy,omitempty" yaml:"policy,omitempty"`
}

func (a InitialAlignment) EffectivePolicy() string {
	if a.Policy == "" {
		return AlignmentDisabled
	}
	return a.Policy
}

type SchemaSync struct {
	AddColumns  bool `json:"add_columns" yaml:"add_columns"`
	DropColumns bool `json:"drop_columns" yaml:"drop_columns"`
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
			Enable:             false,
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
	for i := range fieldRules {
		if fieldRules[i].ValidateRuntimePolicy() != nil {
			fieldRules[i].Enable = false
		}
	}
	return &RuleSet{Rules: fieldRules}
}

func LoadFile(path string) (*RuleSet, error) {
	set, _, err := LoadFileWithRevision(path)
	return set, err
}

func parseFile(data []byte, path string) (*RuleSet, error) {
	var set RuleSet
	if err := yaml.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("parse rules %q: %w", path, err)
	}
	for i := range set.Rules {
		set.Rules[i].DeleteMode = set.Rules[i].EffectiveDeleteMode()
	}
	return &set, nil
}

func SaveFile(path string, set RuleSet) error {
	lock, err := lockRules(path)
	if err != nil {
		return err
	}
	defer lock.Close()
	return saveFileUnlocked(path, set)
}

func saveFileUnlocked(path string, set RuleSet) error {
	if err := set.Validate(); err != nil {
		return err
	}
	set.Rules = append([]SyncRule(nil), set.Rules...)
	for i := range set.Rules {
		set.Rules[i].DeleteMode = set.Rules[i].EffectiveDeleteMode()
	}
	data, err := yaml.Marshal(set)
	if err != nil {
		return fmt.Errorf("marshal rules: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create rules directory: %w", err)
	}
	if err := atomicfile.Write(path, data, 0o600); err != nil {
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
	return s.validate(true)
}

// ValidateStructure checks configuration shape, not runtime support or activation.
func (s RuleSet) ValidateStructure() error {
	return s.validate(false)
}

func (s RuleSet) validate(runtimePolicy bool) error {
	seen := map[string]bool{}
	ids := map[string]bool{}
	for _, rule := range s.Rules {
		if rule.ID != "" {
			if ids[rule.ID] {
				return fmt.Errorf("duplicate rule_id %q", rule.ID)
			}
			ids[rule.ID] = true
		}
		if runtimePolicy && rule.Enable && rule.Direction != DirectionIgnore {
			if err := rule.ValidateRuntimePolicy(); err != nil {
				return err
			}
		}
		switch rule.Direction {
		case "", DirectionEdgeToServer, DirectionBidirectional, DirectionServerToEdge, DirectionIgnore:
		default:
			return fmt.Errorf("invalid direction %q for rule %q", rule.Direction, rule.ID)
		}
		switch rule.ConflictPolicy {
		case "", ConflictNone, ConflictServerWin, ConflictLastWriteWin:
		default:
			return fmt.Errorf("invalid conflict_policy %q for rule %q", rule.ConflictPolicy, rule.ID)
		}
		if rule.Direction != DirectionIgnore && len(rule.PrimaryKeys) == 0 {
			return fmt.Errorf("primary_keys are required for rule %q", rule.ID)
		}
		if mode := rule.EffectiveDeleteMode(); mode != DeleteSoft && mode != DeleteHard {
			return fmt.Errorf("invalid delete_mode %q for rule %q", mode, rule.ID)
		}
		if runtimePolicy && rule.Enable && !rule.pairedRuntime && rule.EffectiveDeleteMode() == DeleteHard && rule.Direction == DirectionBidirectional {
			return fmt.Errorf("bidirectional HARD deletion is not supported for rule %q", rule.ID)
		}
		if policy := rule.InitialAlignment.EffectivePolicy(); policy != AlignmentDisabled && policy != AlignmentManual {
			return fmt.Errorf("initial_alignment.policy must be DISABLED or MANUAL for rule %q", rule.ID)
		}
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
		if rule.SchemaSync.DropColumns && rule.Direction != DirectionServerToEdge && len(rule.SourceNodeIDs) != 1 {
			return fmt.Errorf("drop_columns requires exactly one source_node_id for %s unless direction is %s", baseKey, DirectionServerToEdge)
		}
		switch rule.DispatchTarget {
		case "", DispatchAuto, DispatchNone, DispatchActiveEdges, DispatchSelectedEdges:
		default:
			return fmt.Errorf("invalid dispatch_target %q for %s", rule.DispatchTarget, baseKey)
		}
		switch rule.SyncMode {
		case "", SyncModeOrderedCRUD, SyncModeAppendOnly, SyncModeCRUDCompact:
		default:
			return fmt.Errorf("invalid sync_mode %q for %s", rule.SyncMode, baseKey)
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
		for _, columns := range [][]string{rule.PrimaryKeys, rule.TargetPrimaryKeys} {
			keys := map[string]bool{}
			for _, column := range columns {
				if keys[column] {
					return fmt.Errorf("duplicate primary key column %q", column)
				}
				keys[column] = true
			}
		}
		for _, column := range append(append([]string{}, rule.IncludeColumns...), rule.ExcludeColumns...) {
			if err := validateIdentifier(column); err != nil {
				return err
			}
		}
		for _, key := range rule.PrimaryKeys {
			included := len(rule.IncludeColumns) == 0
			for _, column := range rule.IncludeColumns {
				if column == key {
					included = true
				}
			}
			for _, column := range rule.ExcludeColumns {
				if column == key {
					included = false
				}
			}
			if !included {
				return fmt.Errorf("primary key column %s must remain in the selected columns", key)
			}
		}
		targetColumns := map[string]string{}
		sourceColumns := map[string]string{}
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
			if _, exists := sourceColumns[mapping.SourceColumn]; exists {
				return fmt.Errorf("source column %s is mapped more than once", mapping.SourceColumn)
			}
			sourceColumns[mapping.SourceColumn] = mapping.TargetColumn
		}
		if len(rule.TargetPrimaryKeys) > 0 {
			for i, key := range rule.PrimaryKeys {
				if target, ok := sourceColumns[key]; ok && target != rule.TargetPrimaryKeys[i] {
					return fmt.Errorf("primary_key_mapping_mismatch: %s", key)
				}
			}
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
