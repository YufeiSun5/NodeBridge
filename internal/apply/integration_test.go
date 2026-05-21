package apply_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
)

func TestIntegrationApplyMappedEvent(t *testing.T) {
	dsn := os.Getenv("NODEBRIDGE_APPLY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("NODEBRIDGE_APPLY_MYSQL_DSN not set")
	}

	db, err := mysqlconn.OpenDSN(dsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	createIntegrationTables(t, ctx, db)
	cleanupIntegrationRows(t, ctx, db, "it-event-001")

	worker := apply.NewSQLWorker(db)
	worker.Clock = func() time.Time {
		return time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	}

	result, err := worker.Apply(ctx, mapper.MappedEvent{
		Event: event.SyncEvent{
			EventID:      "it-event-001",
			EventType:    event.TypeInsert,
			OriginNodeID: "edge-001",
			SourceNodeID: "edge-001",
			TargetNodeID: "server-001",
		},
		SourceDatabase:   "scada_edge",
		SourceTable:      "device_config",
		TargetDatabase:   currentDatabase(t, ctx, db),
		TargetTable:      "nodebridge_it_device_settings",
		TargetPrimaryKey: map[string]any{"setting_id": int64(1001)},
		TargetAfter: map[string]any{
			"setting_id":    int64(1001),
			"display_name":  "Pump IT",
			"setting_value": "new",
		},
	})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if result.AlreadyApplied {
		t.Fatal("expected fresh apply")
	}
	cleanupIntegrationRows(t, ctx, db, "it-event-001")
}

func createIntegrationTables(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS nodebridge_it_device_settings (
  setting_id BIGINT PRIMARY KEY,
  display_name VARCHAR(128) NOT NULL,
  setting_value VARCHAR(512) NULL,
  is_deleted TINYINT NOT NULL DEFAULT 0,
  deleted_at DATETIME(3) NULL,
  deleted_by_node VARCHAR(64) NULL,
  updated_by_node VARCHAR(64) NULL,
  last_event_id VARCHAR(128) NULL
)`,
		`CREATE TABLE IF NOT EXISTS sync_apply_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  event_id VARCHAR(128) NOT NULL,
  origin_node_id VARCHAR(64) NOT NULL,
  source_node_id VARCHAR(64) NOT NULL,
  target_node_id VARCHAR(64) NOT NULL,
  database_name VARCHAR(128) NOT NULL,
  table_name VARCHAR(128) NOT NULL,
  target_database_name VARCHAR(128) NULL,
  target_table_name VARCHAR(128) NULL,
  pk_value VARCHAR(512) NOT NULL,
  op_type VARCHAR(16) NOT NULL,
  applied_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_event_id (event_id)
)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("exec integration schema: %v", err)
		}
	}
}

func cleanupIntegrationRows(t *testing.T, ctx context.Context, db *sql.DB, eventID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, "DELETE FROM nodebridge_it_device_settings WHERE setting_id = ?", 1001); err != nil {
		t.Fatalf("cleanup device settings: %v", err)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM sync_apply_log WHERE event_id = ?", eventID); err != nil {
		t.Fatalf("cleanup apply log: %v", err)
	}
}

func currentDatabase(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()
	var name string
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		t.Fatalf("query database: %v", err)
	}
	return name
}
