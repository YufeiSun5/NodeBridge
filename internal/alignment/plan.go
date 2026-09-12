package alignment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

const (
	ToRight = "LEFT_TO_RIGHT"
	ToLeft  = "RIGHT_TO_LEFT"
	NoCopy  = "BOTH_EMPTY"
)

type Observation struct {
	NodeID  string           `json:"node_id"`
	Schema  rulecheck.Schema `json:"schema"`
	HasRows bool             `json:"has_rows"`
}

type ColumnPair struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type Plan struct {
	ID        string       `json:"plan_id"`
	RuleID    string       `json:"rule_id"`
	RuleHash  string       `json:"rule_hash"`
	Direction string       `json:"direction"`
	Source    Observation  `json:"source"`
	Target    Observation  `json:"target"`
	Columns   []ColumnPair `json:"columns"`
	CreatedAt time.Time    `json:"created_at"`
	ExpiresAt time.Time    `json:"expires_at"`
}

// Observe checks emptiness with an indexed, bounded read, never table estimates.
func Observe(ctx context.Context, db *sql.DB, nodeID, database, table string) (Observation, error) {
	result := Observation{NodeID: nodeID}
	if db == nil || nodeID == "" {
		return result, errors.New("alignment_endpoint_required")
	}
	if err := validateTable(database, table); err != nil {
		return result, err
	}
	schema, err := rulecheck.ReadSchema(ctx, db, database, table)
	if err != nil {
		return result, err
	}
	result.Schema = schema
	var exists int
	err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+qualified(schema)+" LIMIT 1)").Scan(&exists)
	result.HasRows = exists != 0
	return result, err
}

// BuildPlan never authorizes a write. Execution must recheck both endpoints under locks.
func BuildPlan(rule rules.SyncRule, left, right Observation, now time.Time) (Plan, error) {
	var p Plan
	if err := (rules.RuleSet{Rules: []rules.SyncRule{rule}}).Validate(); err != nil {
		return p, err
	}
	if rule.Direction == rules.DirectionIgnore {
		return p, errors.New("alignment_ignored_rule")
	}
	if rule.InitialAlignment.EffectivePolicy() != rules.AlignmentManual {
		return p, errors.New("alignment_disabled: select MANUAL before planning")
	}
	if rule.ID == "" || now.IsZero() || left.NodeID == "" || right.NodeID == "" || left.NodeID == right.NodeID {
		return p, errors.New("alignment_identity_invalid")
	}
	if rule.DatabaseName != left.Schema.Database || rule.TableName != left.Schema.Table {
		return p, errors.New("alignment_source_scope_mismatch")
	}
	targetDB, targetTable := rule.TargetDatabaseName, rule.TargetTableName
	if targetDB == "" {
		targetDB = rule.DatabaseName
	}
	if targetTable == "" {
		targetTable = rule.TableName
	}
	if targetDB != right.Schema.Database || targetTable != right.Schema.Table {
		return p, errors.New("alignment_target_scope_mismatch")
	}
	for _, endpoint := range []Observation{left, right} {
		if err := validateTable(endpoint.Schema.Database, endpoint.Schema.Table); err != nil {
			return p, err
		}
		if !strings.EqualFold(endpoint.Schema.Engine, "InnoDB") || len(endpoint.Schema.PrimaryKeys) == 0 {
			return p, errors.New("alignment_requires_innodb_primary_key")
		}
	}
	if left.HasRows && right.HasRows {
		return p, errors.New("alignment_both_nonempty: merge and overwrite are not authorized")
	}
	for _, side := range []struct {
		name   string
		schema rulecheck.Schema
	}{{"source", left.Schema}, {"target", right.Schema}} {
		for _, f := range rulecheck.ValidateLocal(rule, side.schema, side.name) {
			if f.Severity == "error" {
				return p, fmt.Errorf("alignment_schema_invalid: %s: %s", f.Code, f.Message)
			}
		}
	}
	if rule.Direction == rules.DirectionBidirectional {
		if _, err := rulecheck.BuildBidirectionalPair(rule, left.NodeID, right.NodeID, left.Schema, right.Schema); err != nil {
			return p, err
		}
	}
	columns, err := columnPairs(rule, left.Schema, right.Schema)
	if err != nil {
		return p, err
	}
	p = Plan{RuleID: rule.ID, RuleHash: hash(rule), Direction: ToRight, Source: left, Target: right, Columns: columns, CreatedAt: now.UTC(), ExpiresAt: now.UTC().Add(5 * time.Minute)}
	if !left.HasRows && !right.HasRows {
		p.Direction = NoCopy
	}
	if !left.HasRows && right.HasRows {
		p.Direction, p.Source, p.Target = ToLeft, right, left
		for i := range p.Columns {
			p.Columns[i].Source, p.Columns[i].Target = p.Columns[i].Target, p.Columns[i].Source
		}
	}
	// A reverse copy must satisfy the destination's defaults, too.
	selected := map[string]bool{}
	for _, c := range p.Columns {
		selected[c.Target] = true
	}
	for _, c := range p.Target.Schema.Columns {
		if !selected[c.Name] && !c.Generated() && !c.Nullable && !c.HasDefault && !strings.Contains(strings.ToLower(c.Extra), "auto_increment") {
			return Plan{}, fmt.Errorf("alignment_required_target_column: %s", c.Name)
		}
	}
	p.ID = hash(p)
	return p, nil
}

func (p Plan) Validate(rule rules.SyncRule, now time.Time, confirm bool) error {
	if !confirm {
		return errors.New("alignment_confirmation_required")
	}
	if now.Before(p.CreatedAt) || !now.Before(p.ExpiresAt) {
		return errors.New("alignment_plan_expired")
	}
	if p.RuleHash != hash(rule) {
		return errors.New("alignment_rule_changed")
	}
	id := p.ID
	p.ID = ""
	if id == "" || id != hash(p) {
		return errors.New("alignment_plan_changed")
	}
	left, right := p.Source, p.Target
	if p.Direction == ToLeft {
		left, right = right, left
	}
	rebuilt, err := BuildPlan(rule, left, right, p.CreatedAt)
	if err != nil {
		return err
	}
	if rebuilt.ID != id {
		return errors.New("alignment_plan_invalid")
	}
	return nil
}

func columnPairs(rule rules.SyncRule, left, right rulecheck.Schema) ([]ColumnPair, error) {
	mappings := map[string]string{}
	for _, m := range rule.ColumnMappings {
		if _, ok := mappings[m.SourceColumn]; ok {
			return nil, errors.New("alignment_duplicate_mapping")
		}
		mappings[m.SourceColumn] = m.TargetColumn
	}
	for i, key := range rule.PrimaryKeys {
		if len(rule.TargetPrimaryKeys) > 0 {
			if len(rule.TargetPrimaryKeys) != len(rule.PrimaryKeys) {
				return nil, errors.New("alignment_key_mapping_invalid")
			}
			mappings[key] = rule.TargetPrimaryKeys[i]
		}
	}
	target := map[string]rulecheck.Column{}
	for _, c := range right.Columns {
		target[c.Name] = c
	}
	include, exclude := map[string]bool{}, map[string]bool{}
	for _, c := range rule.IncludeColumns {
		include[c] = true
	}
	for _, c := range rule.ExcludeColumns {
		exclude[c] = true
	}
	owners := map[string]bool{}
	pairs := []ColumnPair{}
	for _, c := range left.Columns {
		if c.Generated() || exclude[c.Name] || len(include) > 0 && !include[c.Name] {
			continue
		}
		name := c.Name
		if mappings[name] != "" {
			name = mappings[name]
		}
		if err := mapper.ValidateIdentifier(c.Name); err != nil {
			return nil, err
		}
		if err := mapper.ValidateIdentifier(name); err != nil {
			return nil, err
		}
		t, ok := target[name]
		if !ok || t.Generated() {
			return nil, fmt.Errorf("alignment_target_column_unavailable: %s", name)
		}
		if owners[name] {
			return nil, fmt.Errorf("alignment_mapping_collision: %s", name)
		}
		owners[name] = true
		// Exact types and collations make the map reversible without lossy conversion.
		if !strings.EqualFold(c.Type, t.Type) || c.Collation != t.Collation || c.Nullable != t.Nullable {
			return nil, fmt.Errorf("alignment_incompatible_column: %s -> %s", c.Name, name)
		}
		pairs = append(pairs, ColumnPair{c.Name, name})
	}
	for _, key := range left.PrimaryKeys {
		found := false
		for _, c := range pairs {
			if c.Source == key {
				found = true
			}
		}
		if !found {
			return nil, errors.New("alignment_primary_key_excluded")
		}
	}
	if len(pairs) == 0 {
		return nil, errors.New("alignment_no_columns")
	}
	return pairs, nil
}

func validateTable(database, table string) error {
	if err := mapper.ValidateIdentifier(database); err != nil {
		return err
	}
	if err := mapper.ValidateIdentifier(table); err != nil {
		return err
	}
	if strings.HasPrefix(strings.ToLower(table), "sync_") {
		return errors.New("alignment_system_table_forbidden")
	}
	return nil
}
func qualified(s rulecheck.Schema) string { return "`" + s.Database + "`.`" + s.Table + "`" }
func hash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
