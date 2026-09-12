package dbgovernance

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultQueryLimit   = 50
	defaultMaxQueryRows = 200
	defaultMaxWriteRows = 100
	operationInsert     = "INSERT"
	operationUpdate     = "UPDATE"
	operationDelete     = "DELETE"
	schemaOperationAdd  = "ADD_COLUMN"
	schemaOperationDrop = "DROP_COLUMN"
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	columnTypePattern = regexp.MustCompile(`(?i)^(?:(?:tinyint|smallint|mediumint|int|integer|bigint)(?:\([1-9][0-9]*\))?(?: unsigned)?|(?:decimal|numeric)\([1-9][0-9]*,[0-9]+\)(?: unsigned)?|(?:float|double)(?:\([1-9][0-9]*,[0-9]+\))?(?: unsigned)?|(?:bit|binary|char|varchar|varbinary)\([1-9][0-9]*\)|date|datetime(?:\([0-6]\))?|timestamp(?:\([0-6]\))?|time(?:\([0-6]\))?|year|tinytext|text|mediumtext|longtext|tinyblob|blob|mediumblob|longblob|json|bool|boolean)$`)
)

type Service struct {
	DB              *sql.DB
	Database        string
	MaxQueryRows    int
	MaxMutationRows int
}

type Filter struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
	Values   []any  `json:"values,omitempty"`
}

type Order struct {
	Column    string `json:"column"`
	Direction string `json:"direction,omitempty"`
}

type QueryRequest struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns,omitempty"`
	Filters []Filter `json:"filters,omitempty"`
	OrderBy []Order  `json:"order_by,omitempty"`
	Limit   int      `json:"limit,omitempty"`
}

type QueryResult struct {
	Database string           `json:"database"`
	Table    string           `json:"table"`
	Columns  []string         `json:"columns"`
	Rows     []map[string]any `json:"rows"`
	Count    int              `json:"count"`
	Limit    int              `json:"limit"`
}

type MutationRequest struct {
	Operation    string         `json:"operation"`
	Table        string         `json:"table"`
	Values       map[string]any `json:"values,omitempty"`
	Filters      []Filter       `json:"filters,omitempty"`
	ExpectedRows int            `json:"expected_rows,omitempty"`
	Confirm      bool           `json:"confirm,omitempty"`
}

type MutationPlan struct {
	Operation      string `json:"operation"`
	Database       string `json:"database"`
	Table          string `json:"table"`
	Statement      string `json:"statement"`
	MatchedRows    int    `json:"matched_rows"`
	MaxAllowedRows int    `json:"max_allowed_rows"`
}

type MutationResult struct {
	OK           bool   `json:"ok"`
	Operation    string `json:"operation"`
	Database     string `json:"database"`
	Table        string `json:"table"`
	AffectedRows int64  `json:"affected_rows"`
	Status       string `json:"status"`
}

type ColumnDefault struct {
	Mode  string `json:"mode,omitempty"`
	Value any    `json:"value,omitempty"`
}

type ColumnDefinition struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	Nullable bool          `json:"nullable"`
	Default  ColumnDefault `json:"default,omitempty"`
	Comment  string        `json:"comment,omitempty"`
}

type SchemaChange struct {
	Operation string           `json:"operation"`
	Column    ColumnDefinition `json:"column"`
}

type SchemaChangeRequest struct {
	Operation string           `json:"operation"`
	Table     string           `json:"table"`
	Column    ColumnDefinition `json:"column"`
}

type SchemaChangeApplyRequest struct {
	Change  SchemaChangeRequest `json:"change"`
	PlanID  string              `json:"plan_id"`
	Confirm bool                `json:"confirm"`
}

type SchemaChangePlan struct {
	Operation string `json:"operation"`
	Database  string `json:"database"`
	Table     string `json:"table"`
	Column    string `json:"column"`
	Statement string `json:"statement"`
	Status    string `json:"status"`
	PlanID    string `json:"plan_id"`
}

type SchemaChangeResult struct {
	OK        bool   `json:"ok"`
	Operation string `json:"operation"`
	Database  string `json:"database"`
	Table     string `json:"table"`
	Column    string `json:"column"`
	Status    string `json:"status"`
	PlanID    string `json:"plan_id"`
}

func New(db *sql.DB, database string) *Service {
	return &Service{DB: db, Database: database, MaxQueryRows: defaultMaxQueryRows, MaxMutationRows: defaultMaxWriteRows}
}

func ValidateColumnDefinition(column ColumnDefinition) error {
	_, err := buildColumnDefinition(column)
	return err
}

func (s *Service) Query(ctx context.Context, req QueryRequest) (QueryResult, error) {
	if err := s.validate(); err != nil {
		return QueryResult{}, err
	}
	if err := validateIdentifier(req.Table); err != nil {
		return QueryResult{}, fmt.Errorf("table: %w", err)
	}
	limit := req.Limit
	if limit == 0 {
		limit = defaultQueryLimit
	}
	if limit < 1 || limit > s.maxQueryRows() {
		return QueryResult{}, fmt.Errorf("limit must be between 1 and %d", s.maxQueryRows())
	}
	columns := "*"
	if len(req.Columns) > 0 {
		quoted := make([]string, len(req.Columns))
		for i, column := range req.Columns {
			if err := validateIdentifier(column); err != nil {
				return QueryResult{}, fmt.Errorf("column %q: %w", column, err)
			}
			quoted[i] = quoteIdentifier(column)
		}
		columns = strings.Join(quoted, ", ")
	}
	whereSQL, args, err := buildWhere(req.Filters)
	if err != nil {
		return QueryResult{}, err
	}
	orderSQL, err := buildOrder(req.OrderBy)
	if err != nil {
		return QueryResult{}, err
	}
	statement := fmt.Sprintf("SELECT %s FROM %s%s%s LIMIT %d", columns, s.qualifiedTable(req.Table), whereSQL, orderSQL, limit)
	rows, err := s.DB.QueryContext(ctx, statement, args...)
	if err != nil {
		return QueryResult{}, fmt.Errorf("query governed data: %w", err)
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		return QueryResult{}, err
	}
	items := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(names))
		dest := make([]any, len(names))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return QueryResult{}, err
		}
		item := make(map[string]any, len(names))
		for i, name := range names {
			item[name] = normalizeDatabaseValue(values[i])
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, err
	}
	return QueryResult{Database: s.Database, Table: req.Table, Columns: names, Rows: items, Count: len(items), Limit: limit}, nil
}

func (s *Service) PlanMutation(ctx context.Context, req MutationRequest) (MutationPlan, error) {
	operation, statement, countStatement, args, err := s.buildMutation(req)
	if err != nil {
		return MutationPlan{}, err
	}
	matched := 1
	if operation != operationInsert {
		if err := s.DB.QueryRowContext(ctx, countStatement, args...).Scan(&matched); err != nil {
			return MutationPlan{}, fmt.Errorf("count governed rows: %w", err)
		}
	}
	return MutationPlan{
		Operation: operation, Database: s.Database, Table: req.Table,
		Statement: statement, MatchedRows: matched, MaxAllowedRows: s.maxMutationRows(),
	}, nil
}

func (s *Service) ApplyMutation(ctx context.Context, req MutationRequest) (MutationResult, error) {
	if !req.Confirm {
		return MutationResult{}, errors.New("confirm must be true")
	}
	if req.ExpectedRows < 1 || req.ExpectedRows > s.maxMutationRows() {
		return MutationResult{}, fmt.Errorf("expected_rows must be between 1 and %d", s.maxMutationRows())
	}
	operation, statement, countStatement, args, err := s.buildMutation(req)
	if err != nil {
		return MutationResult{}, err
	}
	if operation == operationInsert && req.ExpectedRows != 1 {
		return MutationResult{}, errors.New("INSERT expected_rows must be 1")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return MutationResult{}, fmt.Errorf("begin governed mutation: %w", err)
	}
	defer tx.Rollback()
	if operation != operationInsert {
		var matched int
		if err := tx.QueryRowContext(ctx, countStatement, args...).Scan(&matched); err != nil {
			return MutationResult{}, fmt.Errorf("count governed rows: %w", err)
		}
		if matched != req.ExpectedRows {
			return MutationResult{}, fmt.Errorf("matched rows changed: expected %d, got %d", req.ExpectedRows, matched)
		}
		statement += fmt.Sprintf(" LIMIT %d", req.ExpectedRows)
	}
	result, err := tx.ExecContext(ctx, statement, mutationExecArgs(operation, req, args)...)
	if err != nil {
		return MutationResult{}, fmt.Errorf("apply governed mutation: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return MutationResult{}, err
	}
	if affected != int64(req.ExpectedRows) {
		return MutationResult{}, fmt.Errorf("affected rows changed: expected %d, got %d", req.ExpectedRows, affected)
	}
	if err := tx.Commit(); err != nil {
		return MutationResult{}, fmt.Errorf("commit governed mutation: %w", err)
	}
	return MutationResult{OK: true, Operation: operation, Database: s.Database, Table: req.Table, AffectedRows: affected, Status: "applied"}, nil
}

func (s *Service) PlanSchemaChange(ctx context.Context, req SchemaChangeRequest) (SchemaChangePlan, error) {
	if err := s.validate(); err != nil {
		return SchemaChangePlan{}, err
	}
	operation := strings.ToUpper(strings.TrimSpace(req.Operation))
	if operation != schemaOperationAdd && operation != schemaOperationDrop {
		return SchemaChangePlan{}, errors.New("operation must be ADD_COLUMN or DROP_COLUMN")
	}
	if err := validateWritableTable(req.Table); err != nil {
		return SchemaChangePlan{}, err
	}
	if err := validateIdentifier(req.Column.Name); err != nil {
		return SchemaChangePlan{}, fmt.Errorf("column: %w", err)
	}
	var tableCount int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, s.Database, req.Table).Scan(&tableCount); err != nil {
		return SchemaChangePlan{}, err
	}
	if tableCount == 0 {
		return SchemaChangePlan{}, fmt.Errorf("target table %s.%s does not exist; table creation is disabled", s.Database, req.Table)
	}
	column, exists, err := s.readColumn(ctx, req.Table, req.Column.Name)
	if err != nil {
		return SchemaChangePlan{}, err
	}
	status := "ready"
	statement := ""
	state := "missing"
	if exists {
		state = column.Type + "/" + column.Nullable + "/" + column.Key
	}
	if operation == schemaOperationAdd {
		definition, err := buildColumnDefinition(req.Column)
		if err != nil {
			return SchemaChangePlan{}, err
		}
		if exists {
			if !strings.EqualFold(column.Type, strings.TrimSpace(req.Column.Type)) || (column.Nullable == "YES") != req.Column.Nullable {
				return SchemaChangePlan{}, fmt.Errorf("column %s already exists with incompatible definition %s nullable=%s", req.Column.Name, column.Type, column.Nullable)
			}
			status = "already_applied"
		} else {
			statement = fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", s.qualifiedTable(req.Table), quoteIdentifier(req.Column.Name), definition)
		}
	} else {
		if !exists {
			status = "already_applied"
		} else {
			if column.Key != "" {
				return SchemaChangePlan{}, fmt.Errorf("column %s is a key column and cannot be dropped", req.Column.Name)
			}
			var indexCount int
			if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`, s.Database, req.Table, req.Column.Name).Scan(&indexCount); err != nil {
				return SchemaChangePlan{}, err
			}
			if indexCount > 0 {
				return SchemaChangePlan{}, fmt.Errorf("column %s is indexed and cannot be dropped", req.Column.Name)
			}
			statement = fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", s.qualifiedTable(req.Table), quoteIdentifier(req.Column.Name))
		}
	}
	planID := schemaPlanID(operation, s.Database, req.Table, req.Column, state, statement)
	return SchemaChangePlan{Operation: operation, Database: s.Database, Table: req.Table, Column: req.Column.Name, Statement: statement, Status: status, PlanID: planID}, nil
}

func (s *Service) ApplySchemaChange(ctx context.Context, req SchemaChangeApplyRequest) (SchemaChangeResult, error) {
	if !req.Confirm {
		return SchemaChangeResult{}, errors.New("confirm must be true")
	}
	plan, err := s.PlanSchemaChange(ctx, req.Change)
	if err != nil {
		return SchemaChangeResult{}, err
	}
	if req.PlanID == "" || req.PlanID != plan.PlanID {
		return SchemaChangeResult{}, errors.New("plan_id is missing or stale; request a new schema change plan")
	}
	if plan.Status == "already_applied" {
		return SchemaChangeResult{OK: true, Operation: plan.Operation, Database: plan.Database, Table: plan.Table, Column: plan.Column, Status: plan.Status, PlanID: plan.PlanID}, nil
	}
	if _, err := s.DB.ExecContext(ctx, plan.Statement); err != nil {
		return SchemaChangeResult{}, fmt.Errorf("apply governed schema change: %w", err)
	}
	return SchemaChangeResult{OK: true, Operation: plan.Operation, Database: plan.Database, Table: plan.Table, Column: plan.Column, Status: "applied", PlanID: plan.PlanID}, nil
}

func (s *Service) buildMutation(req MutationRequest) (operation, statement, countStatement string, filterArgs []any, err error) {
	if err = s.validate(); err != nil {
		return
	}
	operation = strings.ToUpper(strings.TrimSpace(req.Operation))
	if operation != operationInsert && operation != operationUpdate && operation != operationDelete {
		err = errors.New("operation must be INSERT, UPDATE, or DELETE")
		return
	}
	if err = validateWritableTable(req.Table); err != nil {
		return
	}
	whereSQL, args, whereErr := buildWhere(req.Filters)
	if whereErr != nil {
		err = whereErr
		return
	}
	filterArgs = args
	table := s.qualifiedTable(req.Table)
	switch operation {
	case operationInsert:
		if len(req.Filters) > 0 {
			err = errors.New("INSERT does not accept filters")
			return
		}
		columns, placeholders, _, valueErr := sortedValues(req.Values)
		if valueErr != nil {
			err = valueErr
			return
		}
		statement = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
		countStatement = ""
	case operationUpdate:
		if len(req.Filters) == 0 {
			err = errors.New("UPDATE requires at least one filter")
			return
		}
		columns, _, _, valueErr := sortedValues(req.Values)
		if valueErr != nil {
			err = valueErr
			return
		}
		sets := make([]string, len(columns))
		for i, column := range columns {
			sets[i] = column + " = ?"
		}
		statement = fmt.Sprintf("UPDATE %s SET %s%s", table, strings.Join(sets, ", "), whereSQL)
		countStatement = "SELECT COUNT(*) FROM " + table + whereSQL
	case operationDelete:
		if len(req.Filters) == 0 {
			err = errors.New("DELETE requires at least one filter")
			return
		}
		if len(req.Values) > 0 {
			err = errors.New("DELETE does not accept values")
			return
		}
		statement = "DELETE FROM " + table + whereSQL
		countStatement = "SELECT COUNT(*) FROM " + table + whereSQL
	}
	return
}

func mutationExecArgs(operation string, req MutationRequest, filterArgs []any) []any {
	if operation != operationUpdate {
		if operation == operationInsert {
			_, _, values, _ := sortedValues(req.Values)
			return values
		}
		return filterArgs
	}
	_, _, values, _ := sortedValues(req.Values)
	return append(values, filterArgs...)
}

func sortedValues(values map[string]any) (columns, placeholders []string, args []any, err error) {
	if len(values) == 0 {
		return nil, nil, nil, errors.New("values must not be empty")
	}
	names := make([]string, 0, len(values))
	for name, value := range values {
		if err := validateIdentifier(name); err != nil {
			return nil, nil, nil, fmt.Errorf("value column %q: %w", name, err)
		}
		if err := validateScalar(value); err != nil {
			return nil, nil, nil, fmt.Errorf("value column %q: %w", name, err)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		columns = append(columns, quoteIdentifier(name))
		placeholders = append(placeholders, "?")
		args = append(args, values[name])
	}
	return
}

func buildWhere(filters []Filter) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	parts := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))
	for _, filter := range filters {
		if err := validateIdentifier(filter.Column); err != nil {
			return "", nil, fmt.Errorf("filter column %q: %w", filter.Column, err)
		}
		operator := strings.ToUpper(strings.Join(strings.Fields(filter.Operator), " "))
		column := quoteIdentifier(filter.Column)
		switch operator {
		case "=", "!=", "<>", "<", "<=", ">", ">=", "LIKE":
			if err := validateScalar(filter.Value); err != nil {
				return "", nil, fmt.Errorf("filter %s: %w", filter.Column, err)
			}
			parts = append(parts, column+" "+operator+" ?")
			args = append(args, filter.Value)
		case "IS NULL", "IS NOT NULL":
			parts = append(parts, column+" "+operator)
		case "IN", "NOT IN":
			if len(filter.Values) == 0 || len(filter.Values) > defaultMaxWriteRows {
				return "", nil, fmt.Errorf("filter %s values must contain 1..%d items", filter.Column, defaultMaxWriteRows)
			}
			placeholders := make([]string, len(filter.Values))
			for i, value := range filter.Values {
				if err := validateScalar(value); err != nil {
					return "", nil, fmt.Errorf("filter %s: %w", filter.Column, err)
				}
				placeholders[i] = "?"
				args = append(args, value)
			}
			parts = append(parts, column+" "+operator+" ("+strings.Join(placeholders, ", ")+")")
		default:
			return "", nil, fmt.Errorf("unsupported filter operator %q", filter.Operator)
		}
	}
	return " WHERE " + strings.Join(parts, " AND "), args, nil
}

func buildOrder(orders []Order) (string, error) {
	if len(orders) == 0 {
		return "", nil
	}
	parts := make([]string, len(orders))
	for i, order := range orders {
		if err := validateIdentifier(order.Column); err != nil {
			return "", fmt.Errorf("order column %q: %w", order.Column, err)
		}
		direction := strings.ToUpper(strings.TrimSpace(order.Direction))
		if direction == "" {
			direction = "ASC"
		}
		if direction != "ASC" && direction != "DESC" {
			return "", fmt.Errorf("order direction must be ASC or DESC")
		}
		parts[i] = quoteIdentifier(order.Column) + " " + direction
	}
	return " ORDER BY " + strings.Join(parts, ", "), nil
}

type currentColumn struct {
	Type, Nullable, Key string
}

func (s *Service) readColumn(ctx context.Context, table, column string) (currentColumn, bool, error) {
	var result currentColumn
	err := s.DB.QueryRowContext(ctx, `SELECT COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`, s.Database, table, column).Scan(&result.Type, &result.Nullable, &result.Key)
	if errors.Is(err, sql.ErrNoRows) {
		return currentColumn{}, false, nil
	}
	if err != nil {
		return currentColumn{}, false, err
	}
	return result, true, nil
}

func buildColumnDefinition(column ColumnDefinition) (string, error) {
	kind := strings.TrimSpace(column.Type)
	if !columnTypePattern.MatchString(kind) {
		return "", fmt.Errorf("unsupported or unsafe column type %q", column.Type)
	}
	parts := []string{strings.ToUpper(kind)}
	if column.Nullable {
		parts = append(parts, "NULL")
	} else {
		parts = append(parts, "NOT NULL")
	}
	mode := strings.ToLower(strings.TrimSpace(column.Default.Mode))
	switch mode {
	case "", "none":
	case "null":
		parts = append(parts, "DEFAULT NULL")
	case "literal":
		literal, err := sqlLiteral(column.Default.Value)
		if err != nil {
			return "", fmt.Errorf("default: %w", err)
		}
		parts = append(parts, "DEFAULT "+literal)
	case "current_timestamp":
		if !strings.HasPrefix(strings.ToLower(kind), "timestamp") && !strings.HasPrefix(strings.ToLower(kind), "datetime") {
			return "", errors.New("current_timestamp default requires TIMESTAMP or DATETIME")
		}
		expression := "CURRENT_TIMESTAMP"
		if column.Default.Value != nil {
			fsp, ok := integerValue(column.Default.Value)
			if !ok || fsp < 0 || fsp > 6 {
				return "", errors.New("current_timestamp precision must be between 0 and 6")
			}
			expression += fmt.Sprintf("(%d)", fsp)
		}
		parts = append(parts, "DEFAULT "+expression)
	default:
		return "", fmt.Errorf("unsupported default mode %q", column.Default.Mode)
	}
	if len(column.Comment) > 1024 {
		return "", errors.New("column comment exceeds 1024 characters")
	}
	if column.Comment != "" {
		parts = append(parts, "COMMENT "+quoteString(column.Comment))
	}
	return strings.Join(parts, " "), nil
}

func integerValue(value any) (int, bool) {
	switch value := value.(type) {
	case int:
		return value, true
	case int64:
		return int(value), true
	case float64:
		if value == math.Trunc(value) {
			return int(value), true
		}
	case json.Number:
		parsed, err := strconv.Atoi(value.String())
		return parsed, err == nil
	}
	return 0, false
}

func sqlLiteral(value any) (string, error) {
	if err := validateScalar(value); err != nil {
		return "", err
	}
	switch value := value.(type) {
	case nil:
		return "NULL", nil
	case string:
		return quoteString(value), nil
	case bool:
		if value {
			return "1", nil
		}
		return "0", nil
	case json.Number:
		return value.String(), nil
	case float64:
		return strconv.FormatFloat(value, 'g', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(value), 'g', -1, 32), nil
	case int:
		return strconv.Itoa(value), nil
	case int8, int16, int32, int64:
		return fmt.Sprintf("%d", value), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", value), nil
	default:
		return "", fmt.Errorf("unsupported scalar type %T", value)
	}
}

func validateScalar(value any) error {
	switch value := value.(type) {
	case nil, string, bool, json.Number,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("non-finite numbers are not supported")
		}
		return nil
	case float32:
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return errors.New("non-finite numbers are not supported")
		}
		return nil
	default:
		return fmt.Errorf("only scalar JSON values are supported, got %T", value)
	}
}

func (s *Service) validate() error {
	if s.DB == nil {
		return errors.New("mysql db is nil")
	}
	if err := validateIdentifier(s.Database); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	return nil
}

func validateWritableTable(table string) error {
	if err := validateIdentifier(table); err != nil {
		return fmt.Errorf("table: %w", err)
	}
	if strings.HasPrefix(strings.ToLower(table), "sync_") {
		return fmt.Errorf("NodeBridge internal table %q is read-only through governance tools", table)
	}
	return nil
}

func validateIdentifier(value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("invalid identifier %q", value)
	}
	return nil
}

func (s *Service) qualifiedTable(table string) string {
	return quoteIdentifier(s.Database) + "." + quoteIdentifier(table)
}

func quoteIdentifier(value string) string { return "`" + value + "`" }

func quoteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func normalizeDatabaseValue(value any) any {
	switch value := value.(type) {
	case []byte:
		return string(value)
	case time.Time:
		return value.Format(time.RFC3339Nano)
	default:
		return value
	}
}

func schemaPlanID(operation, database, table string, column ColumnDefinition, state, statement string) string {
	payload, _ := json.Marshal(struct {
		Operation, Database, Table, State, Statement string
		Column                                       ColumnDefinition
	}{operation, database, table, state, statement, column})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func (s *Service) maxQueryRows() int {
	if s.MaxQueryRows > 0 {
		return s.MaxQueryRows
	}
	return defaultMaxQueryRows
}

func (s *Service) maxMutationRows() int {
	if s.MaxMutationRows > 0 {
		return s.MaxMutationRows
	}
	return defaultMaxWriteRows
}
