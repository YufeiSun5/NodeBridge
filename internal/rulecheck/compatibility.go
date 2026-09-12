package rulecheck

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

var typeParts = regexp.MustCompile(`^([a-z]+)(?:\(([0-9]+)(?:,([0-9]+))?\))?( unsigned)?$`)

func ValidatePair(rule rules.SyncRule, source, target Schema) []Finding {
	findings := []Finding{}
	add := func(code, severity, message string) { findings = append(findings, Finding{code, severity, message}) }
	mapping := map[string]string{}
	for _, pair := range rule.ColumnMappings {
		mapping[pair.SourceColumn] = pair.TargetColumn
	}
	for i, key := range rule.PrimaryKeys {
		if len(rule.TargetPrimaryKeys) == len(rule.PrimaryKeys) {
			mapping[key] = rule.TargetPrimaryKeys[i]
		}
	}
	columns := map[string]Column{}
	for _, column := range target.Columns {
		columns[column.Name] = column
	}
	selectedTarget := map[string]bool{}
	for _, src := range source.Columns {
		if !selected(src.Name, rule) {
			continue
		}
		name := src.Name
		if mapping[name] != "" {
			name = mapping[name]
		}
		if selectedTarget[name] {
			add("column_mapping_collision", "error", name)
			continue
		}
		selectedTarget[name] = true
		dst, exists := columns[name]
		if !exists {
			add("target_column_missing", "error", name)
			continue
		}
		if dst.Generated() {
			add("target_generated_column", "error", name+" is not writable")
		}
		if src.Nullable && !dst.Nullable {
			add("nullable_source_to_required_target", "error", name)
		}
		if !typeFits(src.Type, dst.Type) {
			add("column_type_incompatible", "error", fmt.Sprintf("%s: %s -> %s requires an explicit conversion or schema change", name, src.Type, dst.Type))
		}
		if src.Collation != dst.Collation {
			add("collation_difference", "warning", name+": comparison and unique-key semantics require data validation")
		}
	}
	for _, dst := range target.Columns {
		if !selectedTarget[dst.Name] && !dst.Nullable && !dst.HasDefault && !dst.Generated() && !strings.Contains(strings.ToLower(dst.Extra), "auto_increment") {
			add("unmapped_required_target_column", "error", dst.Name)
		}
	}
	return findings
}

func typeFits(source, target string) bool {
	source, target = strings.ToLower(source), strings.ToLower(target)
	if source == target {
		return true
	}
	s, t := typeParts.FindStringSubmatch(source), typeParts.FindStringSubmatch(target)
	if s == nil || t == nil {
		return false
	}
	ints := map[string]int{"tinyint": 8, "smallint": 16, "mediumint": 24, "int": 32, "integer": 32, "bigint": 64}
	if bits := ints[s[1]]; bits > 0 {
		targetBits := ints[t[1]]
		if targetBits == 0 {
			return false
		}
		if s[4] == "" && t[4] != "" {
			return false
		}
		if s[4] != "" && t[4] == "" {
			return targetBits > bits
		}
		return targetBits >= bits
	}
	if s[1] != t[1] {
		return false
	}
	a, _ := strconv.Atoi(s[2])
	b, _ := strconv.Atoi(t[2])
	as, _ := strconv.Atoi(s[3])
	bs, _ := strconv.Atoi(t[3])
	switch s[1] {
	case "decimal", "numeric":
		return b-bs >= a-as && bs >= as && (s[4] != "" || t[4] == "")
	case "binary":
		return b == a
	case "varchar", "varbinary", "char", "bit", "datetime", "timestamp", "time":
		return b >= a
	default:
		return false
	}
}
