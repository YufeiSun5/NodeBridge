package conflict

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
)

var keyType = regexp.MustCompile(`^(tinyint|smallint|mediumint|int|bigint|decimal|char|varchar|binary|varbinary)(?:\(([0-9]+)(?:,([0-9]+))?\))?( unsigned)?(?: zerofill)?$`)
var keyNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]{1,3})?$`)

// CanonicalKey uses actual PRIMARY KEY order, types and MySQL collation weights.
// The caller must pin the key schema for the ledger lifetime and reject DDL drift.
// No business rows are read or locked, so capture can run behind an Apply fence.
func CanonicalKey(ctx context.Context, db rulecheck.SchemaReader, schema rulecheck.Schema, values map[string]any) (RowKey, error) {
	key := RowKey{Database: schema.Database, Table: schema.Table}
	if db == nil || !strings.EqualFold(schema.Engine, "InnoDB") || len(schema.PrimaryKeys) == 0 || len(values) != len(schema.PrimaryKeys) {
		return key, errors.New("conflict_actual_primary_key_required")
	}
	if err := mapper.ValidateIdentifier(schema.Database); err != nil {
		return key, err
	}
	if err := mapper.ValidateIdentifier(schema.Table); err != nil {
		return key, err
	}
	var prefixes int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND INDEX_NAME='PRIMARY' AND SUB_PART IS NOT NULL", schema.Database, schema.Table).Scan(&prefixes); err != nil {
		return key, err
	}
	if prefixes != 0 {
		return key, errors.New("conflict_prefix_primary_key_unsupported")
	}
	type part struct {
		Column string `json:"column"`
		Value  []byte `json:"value"`
	}
	parts := make([]part, 0, len(schema.PrimaryKeys))
	for _, name := range schema.PrimaryKeys {
		var column *rulecheck.Column
		for i := range schema.Columns {
			if schema.Columns[i].Name == name {
				column = &schema.Columns[i]
				break
			}
		}
		value, ok := values[name]
		if column == nil || column.Generated() || column.Nullable || !ok || value == nil {
			return key, errors.New("conflict_actual_primary_key_required")
		}
		canonical, err := canonicalPart(ctx, db, *column, value)
		if err != nil {
			return key, fmt.Errorf("canonical primary key %s: %w", name, err)
		}
		parts = append(parts, part{Column: name, Value: canonical})
	}
	encoded, err := json.Marshal(parts)
	if err != nil {
		return key, err
	}
	if len(encoded) > 4096 {
		return key, errors.New("conflict_canonical_key_too_large")
	}
	key.CanonicalKey = encoded
	return key, nil
}

func canonicalPart(ctx context.Context, db rulecheck.SchemaReader, column rulecheck.Column, value any) ([]byte, error) {
	m := keyType.FindStringSubmatch(strings.ToLower(column.Type))
	if m == nil {
		return nil, fmt.Errorf("conflict_primary_key_type_unsupported: %s", column.Type)
	}
	size, _ := strconv.Atoi(m[2])
	scale, _ := strconv.Atoi(m[3])
	switch m[1] {
	case "tinyint", "smallint", "mediumint", "int", "bigint", "decimal":
		text, err := exactNumberText(value)
		if err != nil {
			return nil, err
		}
		if len(text) > 160 || !keyNumber.MatchString(text) {
			return nil, errors.New("conflict_primary_key_number_invalid")
		}
		number, ok := new(big.Rat).SetString(text)
		if !ok {
			return nil, errors.New("conflict_primary_key_number_invalid")
		}
		if m[1] == "decimal" {
			if size < 1 || size > 65 || scale > 30 || scale > size {
				return nil, errors.New("conflict_primary_key_type_unsupported")
			}
			number.Mul(number, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)))
			limit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(size)), nil)
			if !number.IsInt() || new(big.Int).Abs(number.Num()).Cmp(limit) >= 0 || (m[4] != "" && number.Sign() < 0) {
				return nil, errors.New("conflict_primary_key_number_out_of_range")
			}
			return []byte(number.Num().String()), nil
		}
		bits := map[string]uint{"tinyint": 8, "smallint": 16, "mediumint": 24, "int": 32, "bigint": 64}[m[1]]
		unsigned := m[4] != ""
		max := new(big.Int).Lsh(big.NewInt(1), bits)
		min := new(big.Int)
		if !unsigned {
			max.Rsh(max, 1)
			min.Neg(max)
		}
		if !number.IsInt() || number.Num().Cmp(min) < 0 || number.Num().Cmp(max) >= 0 {
			return nil, errors.New("conflict_primary_key_number_out_of_range")
		}
		return []byte(number.Num().String()), nil
	case "binary", "varbinary":
		var bytes []byte
		switch v := value.(type) {
		case []byte:
			bytes = v
		case rowvalue.Binary:
			bytes = v
		default:
			return nil, errors.New("conflict_primary_key_binary_required")
		}
		if size < 1 || len(bytes) > size || (m[1] == "binary" && len(bytes) != size) {
			return nil, errors.New("conflict_primary_key_binary_length")
		}
		return append([]byte{}, bytes...), nil
	case "char", "varchar":
		text, ok := value.(string)
		if !ok || !utf8.ValidString(text) || size < 1 {
			return nil, errors.New("conflict_primary_key_text_required")
		}
		if err := mapper.ValidateIdentifier(column.Collation); err != nil {
			return nil, err
		}
		var charset, padding string
		if err := db.QueryRowContext(ctx, "SELECT CHARACTER_SET_NAME,PAD_ATTRIBUTE FROM information_schema.COLLATIONS WHERE COLLATION_NAME=?", column.Collation).Scan(&charset, &padding); err != nil {
			return nil, fmt.Errorf("conflict_collation_metadata_required: %w", err)
		}
		if err := mapper.ValidateIdentifier(charset); err != nil {
			return nil, err
		}
		if padding != "PAD SPACE" && padding != "NO PAD" {
			return nil, errors.New("conflict_collation_padding_unsupported")
		}
		if utf8.RuneCountInString(text) > size {
			return nil, errors.New("conflict_primary_key_text_length")
		}
		if m[1] == "char" || padding == "PAD SPACE" {
			text = strings.TrimRight(text, " ")
		}
		query := "SELECT WEIGHT_STRING(CONVERT(? USING " + charset + ") COLLATE " + column.Collation + "),CONVERT(CONVERT(? USING " + charset + ") USING utf8mb4)"
		var weights []byte
		var roundtrip string
		if err := db.QueryRowContext(ctx, query, text, text).Scan(&weights, &roundtrip); err != nil {
			return nil, err
		}
		if roundtrip != text {
			return nil, errors.New("conflict_primary_key_charset_loss")
		}
		return weights, nil
	}
	return nil, errors.New("conflict_primary_key_type_unsupported")
}

func exactNumberText(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v), nil
	default:
		return "", errors.New("conflict_primary_key_exact_number_required")
	}
}
