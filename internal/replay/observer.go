package replay

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
)

type Observer struct {
	DB       *sql.DB
	Database string
	uuid     string
}

func (o *Observer) initialize(ctx context.Context) error {
	if o.DB == nil || o.Database == "" {
		return errors.New("replay_observer_dependencies_required")
	}
	if o.uuid == "" {
		return o.DB.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&o.uuid)
	}
	return nil
}

func (o *Observer) IsMarker(c cdc.ChangeEvent) bool {
	return c.DatabaseName == o.Database && c.TableName == Table
}

// Observe persists evidence before a Canal batch can be acknowledged. Re-reading
// an acknowledged prefix after reconnect is idempotent, including split batches.
func (o *Observer) Observe(ctx context.Context, c cdc.ChangeEvent) error {
	if err := o.initialize(ctx); err != nil {
		return err
	}
	if !o.IsMarker(c) || c.Operation != cdc.OperationInsert {
		return errors.New("replay_marker_operation_invalid")
	}
	str := func(key string) string { value, _ := c.After[key].(string); return value }
	token, phase, database, table := str("token"), str("phase"), str("database_name"), str("table_name")
	decoded, err := hex.DecodeString(token)
	if err != nil || len(decoded) != 32 || (phase != "BEGIN" && phase != "END") || database == "" || table == "" || c.BinlogFile == "" || c.BinlogPos == 0 {
		return errors.New("replay_marker_invalid")
	}
	var count int
	if err := o.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_replay_marker WHERE token=? AND database_name=? AND table_name=? AND phase IN ('BEGIN','END')", token, database, table).Scan(&count); err != nil {
		return err
	}
	if count != 2 {
		return errors.New("replay_marker_pair_not_committed")
	}
	_, err = o.DB.ExecContext(ctx, "INSERT INTO sync_replay_position (server_uuid,binlog_file,binlog_pos,phase,token,database_name,table_name) VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE token=IF(token=VALUES(token),token,NULL)", o.uuid, c.BinlogFile, c.BinlogPos, phase, token, database, table)
	return err
}

func (o *Observer) Contains(ctx context.Context, c cdc.ChangeEvent) (bool, error) {
	if err := o.initialize(ctx); err != nil {
		return false, err
	}
	if c.BinlogFile == "" || c.BinlogPos == 0 {
		return false, errors.New("replay_binlog_position_required")
	}
	var phase, database, table string
	err := o.DB.QueryRowContext(ctx, "SELECT phase,database_name,table_name FROM sync_replay_position WHERE server_uuid=? AND binlog_file=? AND binlog_pos<? ORDER BY binlog_pos DESC,phase DESC LIMIT 1", o.uuid, c.BinlogFile, c.BinlogPos).Scan(&phase, &database, &table)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read replay boundary: %w", err)
	}
	return phase == "BEGIN" && database == c.DatabaseName && table == c.TableName, nil
}

func (o *Observer) ProvenPulse(ctx context.Context, c cdc.ChangeEvent) (bool, error) {
	if err := o.initialize(ctx); err != nil {
		return false, err
	}
	token, _ := c.After["token"].(string)
	var count int
	err := o.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_replay_position WHERE server_uuid=? AND binlog_file=? AND token=? AND ((phase='BEGIN' AND binlog_pos<?) OR (phase='END' AND binlog_pos>?))", o.uuid, c.BinlogFile, token, c.BinlogPos, c.BinlogPos).Scan(&count)
	return count == 2, err
}
