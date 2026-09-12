package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"os"
	"regexp"
	"strings"
)

type column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}
type schema struct {
	Columns []column `json:"columns"`
}
type table struct {
	Name   string
	Rename map[string]string
}
type layout struct {
	Stream, Target, State table
	Database, Node        string
	Offset                int64
}
type settings struct {
	RunID           string `json:"run_id"`
	Prefix          string `json:"prefix"`
	Created         string `json:"created"`
	Start           string `json:"start"`
	DurationSeconds int    `json:"duration_seconds"`
	EdgeDSN         string `json:"edge_dsn"`
	ServerDSN       string `json:"server_dsn"`
	SchemaPath      string `json:"schema_path"`
}

var identifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,63}$`)

func readSettings(path string) (settings, schema, error) {
	var c settings
	var s schema
	b, err := os.ReadFile(path)
	if err != nil {
		return c, s, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, s, err
	}
	if !identifier.MatchString(c.Prefix) || len(c.Prefix) > 40 || c.RunID == "" {
		return c, s, fmt.Errorf("invalid run identity")
	}
	b, err = os.ReadFile(c.SchemaPath)
	if err != nil {
		return c, s, err
	}
	if err = json.Unmarshal(b, &s); err != nil {
		return c, s, err
	}
	if len(s.Columns) != 50 {
		return c, s, fmt.Errorf("expected 50 columns")
	}
	seen := map[string]bool{}
	for _, col := range s.Columns {
		if seen[col.Name] || !identifier.MatchString(col.Name) {
			return c, s, fmt.Errorf("invalid column %s", col.Name)
		}
		seen[col.Name] = true
		switch col.Type {
		case "BIGINT", "TINYINT", "DECIMAL(18,6)", "VARCHAR(64)", "VARCHAR(128)", "VARCHAR(512)", "DATETIME(3)", "TEXT":
		default:
			return c, s, fmt.Errorf("unsupported type %s", col.Type)
		}
	}
	return c, s, nil
}

func layouts(prefix string) [2]layout {
	return [2]layout{
		{Stream: table{prefix + "_e_stream", nil}, Target: table{prefix + "_s_inbox", map[string]string{"id": "record_id", "n01": "value01"}}, State: table{prefix + "_e_state", nil}, Database: "scada_edge", Node: "edge-001", Offset: 0},
		{Stream: table{prefix + "_s_stream", nil}, Target: table{prefix + "_e_archive", map[string]string{"id": "record_id", "seq_no": "source_seq"}}, State: table{prefix + "_s_state", map[string]string{"id": "setting_id", "t01": "label01", "n01": "value01"}}, Database: "scada_center", Node: "server-001", Offset: 1000000},
	}
}
func quote(s string) string { return "`" + s + "`" }
func (t table) col(name string) string {
	if v := t.Rename[name]; v != "" {
		return v
	}
	return name
}
func (t table) names(s schema) string {
	out := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		out[i] = quote(t.col(c.Name))
	}
	return strings.Join(out, ",")
}
func (t table) ddl(s schema) string {
	var parts []string
	for _, c := range s.Columns {
		n := " NOT NULL"
		if c.Nullable {
			n = " NULL"
		}
		parts = append(parts, quote(t.col(c.Name))+" "+c.Type+n)
	}
	parts = append(parts, "PRIMARY KEY ("+quote(t.col("id"))+")")
	parts = append(parts, "KEY ix_seq ("+quote(t.col("seq_no"))+")")
	return "CREATE TABLE " + quote(t.Name) + " (" + strings.Join(parts, ",") + ") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin"
}

func makeRow(c settings, l layout, id, revision int64) []any {
	row := []any{id, c.RunID, id, revision, revision, l.Node, "", c.Created, c.Created, int64(0), nil, nil}
	for i := 0; i < 38; i++ {
		row = append(row, businessValue(id, revision, i))
	}
	if revision == 1 && (id == 1000 || id == 1001000) {
		row[12], row[13], row[14] = int64(9007199254740993), int64(9223372036854775807), int64(-9223372036854775808)
		row[24], row[25] = "999999999999.123456", "-999999999999.123456"
	}
	return row
}
func businessValue(id, revision int64, index int) any {
	n := id*100 + revision*41 + int64(index)
	switch {
	case index < 12:
		if (n % 7) == 0 {
			return nil
		}
		if index%2 == 0 {
			return -n
		}
		return n
	case index < 24:
		if (n % 11) == 0 {
			return nil
		}
		return fmt.Sprintf("-%d.%06d", n%1000000000, n%1000000)
	case index < 34:
		if n%13 == 0 {
			return nil
		}
		if n%17 == 0 {
			return ""
		}
		return fmt.Sprintf("\u4e2d\u6587-\u65e5\u672c\u8a9e-'\"\\\n-%d-%d-%d", id, revision, index) + strings.Repeat("x", int(n%55))
	case index == 34:
		return n % 2
	case index == 35:
		return n
	case index == 36:
		size := 256
		if id%997 == 0 {
			size = 4096
		}
		if id%1999 == 0 {
			size = 8192
		}
		return fmt.Sprintf("%d:%d:", id, revision) + strings.Repeat("p", size)
	default:
		if revision%2 == 0 {
			return nil
		}
		return fmt.Sprintf("optional-%d-%d", id, revision)
	}
}

func updateIndices(op int64) []int {
	// Independent operation sequence guarantees all four branches execute.
	branch := op % 20
	if branch >= 10 && branch < 15 {
		out := make([]int, 38)
		for i := range out {
			out[i] = i + 12
		}
		return out
	}
	if branch >= 15 && branch < 18 {
		return []int{12, 24, 36, 49}
	}
	return []int{12 + int((op/20)%38), 12 + int((op/20+1)%38), 12 + int((op/20+2)%38)}
}

func updatedValue(id, revision int64, index int, op int64) any {
	value := businessValue(id, revision, index-12)
	if op%20 < 15 || op%20 >= 18 {
		return value
	}
	// The same key revisits its branch every 1000 operations; use its revision, not op parity.
	if revision%2 == 0 {
		return nil
	}
	if value != nil {
		return value
	}
	switch index {
	case 12:
		return -id - revision
	case 24:
		return "-1.000001"
	default:
		return fmt.Sprintf("nonnull-%d-%d", id, revision)
	}
}

type rate struct{ I, U int }

func rateAt(minute float64) rate {
	switch {
	case minute < 30:
		return rate{20, 10}
	case minute < 240:
		return rate{40, 20}
	case minute < 270:
		return rate{20, 20}
	case minute < 300:
		return rate{80, 40}
	case minute < 330:
		return rate{20, 60}
	case minute < 390:
		return rate{40, 20}
	default:
		return rate{}
	}
}

func addCanonical(h hash.Hash, s schema, values [][]byte) {
	for i, b := range values {
		_, _ = h.Write([]byte(s.Columns[i].Type))
		if b == nil {
			_, _ = h.Write([]byte{0})
			continue
		}
		_, _ = h.Write([]byte{1})
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(b)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(b)
	}
}
func digest(s schema, rows [][][]byte) string {
	h := sha256.New()
	for _, row := range rows {
		addCanonical(h, s, row)
	}
	return hex.EncodeToString(h.Sum(nil))
}
