package canal

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
)

var (
	alterColumnPattern      = regexp.MustCompile(`(?is)^ALTER\s+TABLE\s+((?:` + "`?" + `[A-Za-z_][A-Za-z0-9_]*` + "`?" + `\.)?` + "`?" + `[A-Za-z_][A-Za-z0-9_]*` + "`?" + `)\s+(ADD|DROP)\s+(?:COLUMN\s+)?(` + "`?" + `[A-Za-z_][A-Za-z0-9_]*` + "`?" + `)(?:\s+(.*))?$`)
	currentTimestampPattern = regexp.MustCompile(`(?i)^CURRENT_TIMESTAMP(?:\(([0-6])\))?$`)
)

type ddlToken struct {
	value  string
	quoted bool
}

func ParseAlterColumnSQL(database, table, rawSQL string) (*dbgovernance.SchemaChange, bool, error) {
	statement := strings.TrimSpace(rawSQL)
	statement = strings.TrimSpace(strings.TrimSuffix(statement, ";"))
	if !strings.HasPrefix(strings.ToUpper(statement), "ALTER TABLE ") {
		return nil, false, nil
	}
	if strings.Contains(statement, "/*") || strings.Contains(statement, "--") || strings.Contains(statement, "#") || strings.ContainsRune(statement, 0) {
		return nil, true, errors.New("DDL comments and NUL bytes are not supported")
	}
	matches := alterColumnPattern.FindStringSubmatch(statement)
	if matches == nil {
		return nil, false, nil
	}
	ddlDatabase, ddlTable, err := splitQualifiedIdentifier(matches[1])
	if err != nil {
		return nil, true, err
	}
	if ddlDatabase != "" && !strings.EqualFold(ddlDatabase, database) {
		return nil, true, fmt.Errorf("DDL database %q does not match Canal header %q", ddlDatabase, database)
	}
	if !strings.EqualFold(ddlTable, table) {
		return nil, true, fmt.Errorf("DDL table %q does not match Canal header %q", ddlTable, table)
	}
	columnName := trimIdentifier(matches[3])
	operation := strings.ToUpper(matches[2]) + "_COLUMN"
	if isNonColumnAlterKeyword(columnName) {
		return nil, false, nil
	}
	if operation == "DROP_COLUMN" {
		if strings.TrimSpace(matches[4]) != "" {
			return nil, true, errors.New("DROP COLUMN options are not supported")
		}
		return &dbgovernance.SchemaChange{Operation: operation, Column: dbgovernance.ColumnDefinition{Name: columnName}}, true, nil
	}
	column, err := parseAddColumnDefinition(columnName, matches[4])
	if err != nil {
		return nil, true, err
	}
	return &dbgovernance.SchemaChange{Operation: operation, Column: column}, true, nil
}

func isNonColumnAlterKeyword(value string) bool {
	switch strings.ToUpper(value) {
	case "INDEX", "KEY", "PRIMARY", "UNIQUE", "FOREIGN", "FULLTEXT", "SPATIAL", "CHECK", "CONSTRAINT", "PARTITION":
		return true
	default:
		return false
	}
}

func parseAddColumnDefinition(name, raw string) (dbgovernance.ColumnDefinition, error) {
	tokens, err := tokenizeDDL(raw)
	if err != nil {
		return dbgovernance.ColumnDefinition{}, err
	}
	if len(tokens) == 0 || tokens[0].quoted {
		return dbgovernance.ColumnDefinition{}, errors.New("ADD COLUMN type is required")
	}
	kind := tokens[0].value
	index := 1
	if index < len(tokens) && strings.EqualFold(tokens[index].value, "UNSIGNED") {
		kind += " UNSIGNED"
		index++
	}
	column := dbgovernance.ColumnDefinition{Name: name, Type: kind, Nullable: true}
	for index < len(tokens) {
		token := strings.ToUpper(tokens[index].value)
		switch token {
		case "NULL":
			column.Nullable = true
			index++
		case "NOT":
			if index+1 >= len(tokens) || !strings.EqualFold(tokens[index+1].value, "NULL") {
				return dbgovernance.ColumnDefinition{}, errors.New("only NOT NULL is supported")
			}
			column.Nullable = false
			index += 2
		case "DEFAULT":
			if index+1 >= len(tokens) {
				return dbgovernance.ColumnDefinition{}, errors.New("DEFAULT value is required")
			}
			column.Default, err = parseDefaultToken(tokens[index+1])
			if err != nil {
				return dbgovernance.ColumnDefinition{}, err
			}
			index += 2
		case "COMMENT":
			if index+1 >= len(tokens) || !tokens[index+1].quoted {
				return dbgovernance.ColumnDefinition{}, errors.New("COMMENT requires a quoted string")
			}
			column.Comment = tokens[index+1].value
			index += 2
		default:
			return dbgovernance.ColumnDefinition{}, fmt.Errorf("unsupported ADD COLUMN clause %q", tokens[index].value)
		}
	}
	if err := dbgovernance.ValidateColumnDefinition(column); err != nil {
		return dbgovernance.ColumnDefinition{}, err
	}
	return column, nil
}

func parseDefaultToken(token ddlToken) (dbgovernance.ColumnDefault, error) {
	if token.quoted {
		return dbgovernance.ColumnDefault{Mode: "literal", Value: token.value}, nil
	}
	upper := strings.ToUpper(token.value)
	if upper == "NULL" {
		return dbgovernance.ColumnDefault{Mode: "null"}, nil
	}
	if match := currentTimestampPattern.FindStringSubmatch(token.value); match != nil {
		value := any(nil)
		if match[1] != "" {
			precision, _ := strconv.Atoi(match[1])
			value = precision
		}
		return dbgovernance.ColumnDefault{Mode: "current_timestamp", Value: value}, nil
	}
	if upper == "TRUE" || upper == "FALSE" {
		return dbgovernance.ColumnDefault{Mode: "literal", Value: upper == "TRUE"}, nil
	}
	number := json.Number(token.value)
	if _, err := number.Float64(); err == nil {
		return dbgovernance.ColumnDefault{Mode: "literal", Value: number}, nil
	}
	return dbgovernance.ColumnDefault{}, fmt.Errorf("unsupported unquoted DEFAULT %q", token.value)
}

func tokenizeDDL(raw string) ([]ddlToken, error) {
	text := strings.TrimSpace(raw)
	tokens := make([]ddlToken, 0)
	for index := 0; index < len(text); {
		for index < len(text) && (text[index] == ' ' || text[index] == '\t' || text[index] == '\r' || text[index] == '\n') {
			index++
		}
		if index >= len(text) {
			break
		}
		if text[index] == '\'' {
			index++
			var value strings.Builder
			closed := false
			for index < len(text) {
				if text[index] == '\'' {
					if index+1 < len(text) && text[index+1] == '\'' {
						value.WriteByte('\'')
						index += 2
						continue
					}
					index++
					closed = true
					break
				}
				if text[index] == '\\' && index+1 < len(text) {
					index++
				}
				value.WriteByte(text[index])
				index++
			}
			if !closed {
				return nil, errors.New("unterminated quoted DDL value")
			}
			tokens = append(tokens, ddlToken{value: value.String(), quoted: true})
			continue
		}
		start := index
		for index < len(text) && text[index] != ' ' && text[index] != '\t' && text[index] != '\r' && text[index] != '\n' {
			if text[index] == ';' {
				return nil, errors.New("multiple DDL statements are not supported")
			}
			index++
		}
		tokens = append(tokens, ddlToken{value: text[start:index]})
	}
	return tokens, nil
}

func splitQualifiedIdentifier(value string) (string, string, error) {
	parts := strings.Split(value, ".")
	if len(parts) == 1 {
		return "", trimIdentifier(parts[0]), nil
	}
	if len(parts) == 2 {
		return trimIdentifier(parts[0]), trimIdentifier(parts[1]), nil
	}
	return "", "", fmt.Errorf("invalid qualified table %q", value)
}

func trimIdentifier(value string) string {
	return strings.Trim(strings.TrimSpace(value), "`")
}
