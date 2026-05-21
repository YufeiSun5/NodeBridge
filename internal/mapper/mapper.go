package mapper

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type MappedEvent struct {
	Event            event.SyncEvent
	SourceDatabase   string
	SourceTable      string
	TargetDatabase   string
	TargetTable      string
	TargetPrimaryKey map[string]any
	TargetBefore     map[string]any
	TargetAfter      map[string]any
}

func MapEvent(evt event.SyncEvent, rule rules.SyncRule) (MappedEvent, error) {
	targetDatabase := defaultString(rule.TargetDatabaseName, evt.DatabaseName)
	targetTable := defaultString(rule.TargetTableName, evt.TableName)
	if err := validateIdentifiers(targetDatabase, targetTable); err != nil {
		return MappedEvent{}, err
	}

	columnMap := buildColumnMap(rule.ColumnMappings)
	targetPrimaryKeys, err := resolveTargetPrimaryKeys(rule.PrimaryKeys, rule.TargetPrimaryKeys, columnMap)
	if err != nil {
		return MappedEvent{}, err
	}
	if err := validateIdentifierList(targetPrimaryKeys); err != nil {
		return MappedEvent{}, err
	}

	include := stringSet(rule.IncludeColumns)
	exclude := stringSet(rule.ExcludeColumns)
	before, err := mapColumns(evt.Before, include, exclude, columnMap)
	if err != nil {
		return MappedEvent{}, err
	}
	after, err := mapColumns(evt.After, include, exclude, columnMap)
	if err != nil {
		return MappedEvent{}, err
	}
	primaryKey, err := mapPrimaryKey(evt.PrimaryKey, rule.PrimaryKeys, targetPrimaryKeys, columnMap)
	if err != nil {
		return MappedEvent{}, err
	}

	mapped := evt
	mapped.DatabaseName = targetDatabase
	mapped.TableName = targetTable
	mapped.PrimaryKey = primaryKey
	mapped.Before = before
	mapped.After = after

	return MappedEvent{
		Event:            mapped,
		SourceDatabase:   evt.DatabaseName,
		SourceTable:      evt.TableName,
		TargetDatabase:   targetDatabase,
		TargetTable:      targetTable,
		TargetPrimaryKey: primaryKey,
		TargetBefore:     before,
		TargetAfter:      after,
	}, nil
}

func ValidateIdentifier(value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("invalid identifier %q", value)
	}
	return nil
}

func mapPrimaryKey(source map[string]any, sourceKeys, targetKeys []string, columnMap map[string]string) (map[string]any, error) {
	if len(sourceKeys) != len(targetKeys) {
		return nil, errors.New("source and target primary key count mismatch")
	}
	if len(source) == 0 {
		return nil, nil
	}

	result := make(map[string]any, len(source))
	for i, sourceKey := range sourceKeys {
		targetKey := targetKeys[i]
		value, ok := source[sourceKey]
		if !ok {
			continue
		}
		result[targetKey] = value
	}
	for sourceKey, value := range source {
		if contains(sourceKeys, sourceKey) {
			continue
		}
		result[mapColumnName(sourceKey, columnMap)] = value
	}
	return result, nil
}

func mapColumns(source map[string]any, include, exclude map[string]bool, columnMap map[string]string) (map[string]any, error) {
	if source == nil {
		return nil, nil
	}

	result := make(map[string]any, len(source))
	for sourceColumn, value := range source {
		if len(include) > 0 && !include[sourceColumn] {
			continue
		}
		if exclude[sourceColumn] {
			continue
		}

		targetColumn := mapColumnName(sourceColumn, columnMap)
		if err := ValidateIdentifier(targetColumn); err != nil {
			return nil, err
		}
		result[targetColumn] = value
	}
	return result, nil
}

func resolveTargetPrimaryKeys(sourceKeys, explicitTargetKeys []string, columnMap map[string]string) ([]string, error) {
	if len(explicitTargetKeys) > 0 {
		if len(sourceKeys) != len(explicitTargetKeys) {
			return nil, errors.New("source and target primary key count mismatch")
		}
		return explicitTargetKeys, nil
	}

	targetKeys := make([]string, 0, len(sourceKeys))
	for _, sourceKey := range sourceKeys {
		targetKeys = append(targetKeys, mapColumnName(sourceKey, columnMap))
	}
	return targetKeys, nil
}

func buildColumnMap(mappings []rules.ColumnMapping) map[string]string {
	result := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		if mapping.SourceColumn == "" || mapping.TargetColumn == "" {
			continue
		}
		result[mapping.SourceColumn] = mapping.TargetColumn
	}
	return result
}

func mapColumnName(source string, columnMap map[string]string) string {
	if target, ok := columnMap[source]; ok {
		return target
	}
	return source
}

func validateIdentifiers(values ...string) error {
	for _, value := range values {
		if err := ValidateIdentifier(value); err != nil {
			return err
		}
	}
	return nil
}

func validateIdentifierList(values []string) error {
	for _, value := range values {
		if err := ValidateIdentifier(value); err != nil {
			return err
		}
	}
	return nil
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
