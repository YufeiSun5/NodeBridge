package mysqlconn_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeMigrationsIncludeManagementFailureStore(t *testing.T) {
	for _, mode := range []string{"edge", "server"} {
		path := filepath.Join("..", "..", "migrations", mode, "001_mvp_tables.sql")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s migration: %v", mode, err)
		}
		if !strings.Contains(string(data), "CREATE TABLE IF NOT EXISTS sync_ack_log") {
			t.Fatalf("%s migration does not create sync_ack_log", mode)
		}
		if !strings.Contains(string(data), "KEY idx_table_op (table_name, op_type)") {
			t.Fatalf("%s migration does not cover apply operation counts", mode)
		}
	}
}
