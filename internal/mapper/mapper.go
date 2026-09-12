package mapper

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type MappedEvent struct {
	ConflictPolicy       string
	ConflictSource       []byte
	Event                event.SyncEvent
	SourceDatabase       string
	SourceTable          string
	TargetDatabase       string
	TargetTable          string
	SyncMode             string
	DeleteMode           string
	TrackDeleteReplay    bool
	TargetKeyColumns     []string
	TargetPrimaryKey     map[string]any
	TargetBefore         map[string]any
	TargetAfter          map[string]any
	TargetColumns        map[string]string
	SchemaChangeSelected bool
}

func MapEvent(evt event.SyncEvent, rule rules.SyncRule) (MappedEvent, error) {
	if rule.Enable && rule.Direction != rules.DirectionIgnore {
		if err := rule.ValidateRuntimePolicy(); err != nil {
			return MappedEvent{}, err
		}
	}
	var conflictSource []byte
	if rule.ConflictPolicy == rules.ConflictLastWriteWin {
		var err error
		conflictSource, err = json.Marshal(evt)
		if err != nil {
			return MappedEvent{}, err
		}
	}
	targetDatabase := defaultString(rule.TargetDatabaseName, evt.DatabaseName)
	targetTable := defaultString(rule.TargetTableName, evt.TableName)
	if err := validateIdentifiers(targetDatabase, targetTable); err != nil {
		return MappedEvent{}, err
	}

	columnMap := buildColumnMap(rule.ColumnMappings)
	if evt.EventType == event.TypeUpdate {
		for _, key := range rule.PrimaryKeys {
			before, existsBefore := evt.Before[key]
			after, existsAfter := evt.After[key]
			if existsBefore && existsAfter && !reflect.DeepEqual(before, after) {
				return MappedEvent{}, fmt.Errorf("primary_key_changed: column %s in event %s", key, evt.EventID)
			}
		}
	}
	targetPrimaryKeys, err := resolveTargetPrimaryKeys(rule.PrimaryKeys, rule.TargetPrimaryKeys, columnMap)
	if err != nil {
		return MappedEvent{}, err
	}
	if err := validateIdentifierList(targetPrimaryKeys); err != nil {
		return MappedEvent{}, err
	}
	for i, source := range rule.PrimaryKeys {
		if target, explicit := columnMap[source]; explicit && target != targetPrimaryKeys[i] {
			return MappedEvent{}, fmt.Errorf("primary_key_mapping_mismatch: %s", source)
		}
		columnMap[source] = targetPrimaryKeys[i]
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
	var primaryKey map[string]any
	if evt.EventType != event.TypeAddColumn && evt.EventType != event.TypeDropColumn {
		primaryKey, err = mapPrimaryKey(evt.PrimaryKey, rule.PrimaryKeys, targetPrimaryKeys)
	}
	if err != nil {
		return MappedEvent{}, err
	}
	if evt.EventType == event.TypeInsert || evt.EventType == event.TypeUpdate {
		for key, value := range primaryKey {
			afterValue, present := after[key]
			if evt.EventType == event.TypeInsert && !present {
				return MappedEvent{}, fmt.Errorf("invalid_primary_key: INSERT after image lacks %s", key)
			}
			if present && !reflect.DeepEqual(afterValue, value) {
				return MappedEvent{}, fmt.Errorf("primary_key_image_mismatch: %s", key)
			}
		}
	}

	mapped := evt
	mapped.DatabaseName = targetDatabase
	mapped.TableName = targetTable
	mapped.PrimaryKey = primaryKey
	mapped.Before = before
	mapped.After = after
	schemaChangeSelected := false
	if evt.SchemaChange != nil && columnSelected(evt.SchemaChange.Column.Name, include, exclude) {
		change := *evt.SchemaChange
		change.Column.Name = mapColumnName(change.Column.Name, columnMap)
		if err := ValidateIdentifier(change.Column.Name); err != nil {
			return MappedEvent{}, err
		}
		mapped.SchemaChange = &change
		schemaChangeSelected = true
	}

	return MappedEvent{
		ConflictPolicy:       rule.ConflictPolicy,
		ConflictSource:       conflictSource,
		Event:                mapped,
		SourceDatabase:       evt.DatabaseName,
		SourceTable:          evt.TableName,
		TargetDatabase:       targetDatabase,
		TargetTable:          targetTable,
		SyncMode:             defaultString(rule.SyncMode, rules.SyncModeOrderedCRUD),
		DeleteMode:           rule.EffectiveDeleteMode(),
		TrackDeleteReplay:    rule.Direction == rules.DirectionBidirectional,
		TargetKeyColumns:     append([]string(nil), targetPrimaryKeys...),
		TargetPrimaryKey:     primaryKey,
		TargetBefore:         before,
		TargetAfter:          after,
		TargetColumns:        columnMap,
		SchemaChangeSelected: schemaChangeSelected,
	}, nil
}

func (m MappedEvent) TargetColumn(source string) string {
	return mapColumnName(source, m.TargetColumns)
}

func columnSelected(column string, include, exclude map[string]bool) bool {
	if len(include) > 0 && !include[column] {
		return false
	}
	return !exclude[column]
}

func ValidateIdentifier(value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("invalid identifier %q", value)
	}
	return nil
}

func mapPrimaryKey(source map[string]any, sourceKeys, targetKeys []string) (map[string]any, error) {
	if len(sourceKeys) != len(targetKeys) {
		return nil, errors.New("source and target primary key count mismatch")
	}
	if len(sourceKeys) == 0 || len(source) != len(sourceKeys) {
		return nil, errors.New("invalid_primary_key: event must contain exactly all rule primary keys")
	}

	result := make(map[string]any, len(source))
	for i, sourceKey := range sourceKeys {
		targetKey := targetKeys[i]
		value, ok := source[sourceKey]
		if !ok || value == nil {
			return nil, fmt.Errorf("invalid_primary_key: missing or NULL key %s", sourceKey)
		}
		if _, duplicate := result[targetKey]; duplicate {
			return nil, fmt.Errorf("invalid_primary_key: duplicate mapped key %s", targetKey)
		}
		result[targetKey] = value
	}
	return result, nil
}

func mapColumns(source map[string]any, include, exclude map[string]bool, columnMap map[string]string) (map[string]any, error) {
	if source == nil {
		return nil, nil
	}

	result := make(map[string]any, len(source))
	owners := make(map[string]string, len(source))
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
		if previous, exists := owners[targetColumn]; exists {
			return nil, fmt.Errorf("column_mapping_collision: %s and %s both map to %s", previous, sourceColumn, targetColumn)
		}
		owners[targetColumn] = sourceColumn
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

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
