package rulecheck

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type Finding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type Result struct {
	RuleID             string    `json:"rule_id"`
	Side               string    `json:"side"`
	OK                 bool      `json:"ok"`
	Schema             Schema    `json:"schema"`
	Findings           []Finding `json:"findings"`
	CheckedPermissions []string  `json:"checked_permissions"`
	Unverified         []string  `json:"unverified"`
}

func Check(ctx context.Context, db *sql.DB, rule rules.SyncRule, side string) (Result, error) {
	result := Result{RuleID: rule.ID, Side: side, OK: true, Findings: []Finding{}, CheckedPermissions: []string{}, Unverified: []string{"remote_schema_compatibility", "existing_data_conflicts", "cdc_actual_coverage", "dependency_metadata_visibility", "trigger_and_foreign_key_effects"}}
	if side != "source" && side != "target" {
		return result, fmt.Errorf("side must be source or target")
	}
	if err := (rules.RuleSet{Rules: []rules.SyncRule{rule}}).Validate(); err != nil {
		result.add("rule_invalid", "error", err.Error())
		return result, nil
	}
	database, table := rule.DatabaseName, rule.TableName
	if side == "target" {
		if rule.TargetDatabaseName != "" {
			database = rule.TargetDatabaseName
		}
		if rule.TargetTableName != "" {
			table = rule.TargetTableName
		}
	}
	schema, err := ReadSchema(ctx, db, database, table)
	result.Schema = schema
	if err != nil {
		result.add("schema_unavailable", "error", err.Error())
		return result, nil
	}
	result.Findings = append(result.Findings, ValidateLocal(rule, schema, side)...)
	for _, finding := range result.Findings {
		if finding.Severity == "error" {
			result.OK = false
		}
	}
	if !result.OK {
		return result, nil
	}
	checkDependencies(ctx, db, schema, &result)
	columns := []string{}
	for _, column := range schema.Columns {
		if !column.Generated() {
			columns = append(columns, column.Name)
		}
	}
	if len(columns) == 0 {
		result.add("no_writable_columns", "error", "table has no writable columns")
		return result, nil
	}
	statements := permissionStatements(schema, columns, side, rule.EffectiveDeleteMode(), rule.SyncMode)
	for _, statement := range statements {
		rows, err := db.QueryContext(ctx, statement.SQL)
		if err != nil {
			result.add("permission_"+strings.ToLower(statement.Operation), "error", err.Error())
			continue
		}
		for rows.Next() {
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			result.add("permission_"+strings.ToLower(statement.Operation), "error", err.Error())
			continue
		}
		result.CheckedPermissions = append(result.CheckedPermissions, statement.Operation)
	}
	return result, nil
}

func (r *Result) add(code, severity, message string) {
	r.Findings = append(r.Findings, Finding{code, severity, message})
	if severity == "error" {
		r.OK = false
	}
}

func ValidateLocal(rule rules.SyncRule, schema Schema, side string) []Finding {
	findings := []Finding{}
	add := func(code, severity, message string) { findings = append(findings, Finding{code, severity, message}) }
	if !strings.EqualFold(schema.Engine, "InnoDB") {
		add("non_transactional_table", "error", "synchronization requires an InnoDB base table")
	}
	keys := append([]string(nil), rule.PrimaryKeys...)
	mappings := map[string]string{}
	for _, mapping := range rule.ColumnMappings {
		mappings[mapping.SourceColumn] = mapping.TargetColumn
	}
	if side == "target" {
		if len(rule.TargetPrimaryKeys) > 0 {
			keys = append([]string(nil), rule.TargetPrimaryKeys...)
		} else {
			for i, key := range keys {
				if target := mappings[key]; target != "" {
					keys[i] = target
				}
			}
		}
	}
	actual := append([]string(nil), schema.PrimaryKeys...)
	sort.Strings(keys)
	sort.Strings(actual)
	if len(keys) == 0 || !reflect.DeepEqual(keys, actual) {
		add("primary_key_mismatch", "error", fmt.Sprintf("rule keys %v do not match actual primary key %v", keys, actual))
	}
	columns := map[string]Column{}
	for _, c := range schema.Columns {
		columns[c.Name] = c
	}
	if side == "source" {
		owners := map[string]string{}
		for _, c := range schema.Columns {
			if !selected(c.Name, rule) {
				continue
			}
			target := c.Name
			if mappings[c.Name] != "" {
				target = mappings[c.Name]
			}
			for i, key := range rule.PrimaryKeys {
				if key == c.Name && len(rule.TargetPrimaryKeys) == len(rule.PrimaryKeys) {
					target = rule.TargetPrimaryKeys[i]
				}
			}
			if previous, exists := owners[target]; exists {
				add("column_mapping_collision", "error", previous+" and "+c.Name+" map to "+target)
			}
			owners[target] = c.Name
		}
		for _, c := range rule.IncludeColumns {
			if _, ok := columns[c]; !ok {
				add("source_column_missing", "error", c)
			}
		}
	} else {
		for _, mapping := range rule.ColumnMappings {
			if selected(mapping.SourceColumn, rule) {
				if _, ok := columns[mapping.TargetColumn]; !ok {
					add("target_column_missing", "error", mapping.TargetColumn)
				}
			}
		}
		if rule.EffectiveDeleteMode() == rules.DeleteSoft && rule.SyncMode != rules.SyncModeAppendOnly {
			for _, name := range []string{"is_deleted", "deleted_at", "deleted_by_node", "updated_by_node", "last_event_id"} {
				target := name
				if mappings[name] != "" {
					target = mappings[name]
				}
				c, exists := columns[target]
				if !exists {
					add("soft_delete_column_missing", "error", target)
					continue
				}
				valid := false
				typ := strings.ToLower(c.Type)
				switch name {
				case "is_deleted":
					valid = strings.Contains(typ, "int") || strings.HasPrefix(typ, "bit")
				case "deleted_at":
					valid = strings.HasPrefix(typ, "datetime") || strings.HasPrefix(typ, "timestamp")
				default:
					valid = strings.Contains(typ, "char") || strings.Contains(typ, "text")
				}
				if !valid || c.Generated() {
					add("soft_delete_column_type", "error", target+": "+c.Type)
				}
			}
		}
	}
	if len(schema.UniqueIndexes) > 1 {
		add("additional_unique_constraints", "warning", "business unique keys require cross-node collision checks; source_node_ids does not isolate keys")
	}
	return findings
}

func selected(column string, rule rules.SyncRule) bool {
	for _, excluded := range rule.ExcludeColumns {
		if excluded == column {
			return false
		}
	}
	if len(rule.IncludeColumns) == 0 {
		return true
	}
	for _, included := range rule.IncludeColumns {
		if included == column {
			return true
		}
	}
	return false
}

type permissionStatement struct{ Operation, SQL string }

func permissionStatements(schema Schema, columns []string, side, deleteMode, syncMode string) []permissionStatement {
	quotedColumns, assignments := []string{}, []string{}
	for _, column := range columns {
		quotedColumns = append(quotedColumns, quoted(column))
		assignments = append(assignments, quoted(column)+"="+quoted(column))
	}
	list, table := strings.Join(quotedColumns, ","), qualified(schema)
	result := []permissionStatement{{"SELECT", "EXPLAIN SELECT " + list + " FROM " + table + " WHERE 0"}}
	if side == "source" {
		return result
	}
	result = append(result, permissionStatement{"INSERT", "EXPLAIN INSERT INTO " + table + " (" + list + ") SELECT " + list + " FROM " + table + " WHERE 0"})
	if syncMode != rules.SyncModeAppendOnly {
		result = append(result, permissionStatement{"UPDATE", "EXPLAIN UPDATE " + table + " SET " + strings.Join(assignments, ",") + " WHERE 0"})
		if deleteMode == rules.DeleteHard {
			result = append(result, permissionStatement{"DELETE", "EXPLAIN DELETE FROM " + table + " WHERE 0"})
		}
	}
	return result
}
