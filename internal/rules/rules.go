package rules

const (
	DirectionEdgeToServer  = "EDGE_TO_SERVER"
	DirectionBidirectional = "BIDIRECTIONAL"
	DirectionServerToEdge  = "SERVER_TO_EDGE"
	DirectionIgnore        = "IGNORE"

	ConflictNone         = "NONE"
	ConflictServerWin    = "SERVER_WIN"
	ConflictLastWriteWin = "LAST_WRITE_WIN"
)

type SyncRule struct {
	ID                 string          `json:"id" yaml:"id"`
	DatabaseName       string          `json:"database_name" yaml:"database_name"`
	TableName          string          `json:"table_name" yaml:"table_name"`
	TargetDatabaseName string          `json:"target_database_name,omitempty" yaml:"target_database_name,omitempty"`
	TargetTableName    string          `json:"target_table_name,omitempty" yaml:"target_table_name,omitempty"`
	Direction          string          `json:"direction" yaml:"direction"`
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

func (s RuleSet) Find(databaseName, tableName string) *SyncRule {
	for i := range s.Rules {
		rule := &s.Rules[i]
		if rule.DatabaseName == databaseName && rule.TableName == tableName {
			return rule
		}
	}
	return nil
}
