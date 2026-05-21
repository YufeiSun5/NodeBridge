package cdc

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type MySQLOffsetStore struct {
	DB    *sql.DB
	Clock func() time.Time
}

func NewMySQLOffsetStore(db *sql.DB) *MySQLOffsetStore {
	return &MySQLOffsetStore{DB: db, Clock: time.Now}
}

func (s *MySQLOffsetStore) Load(ctx context.Context, readerName string) (Offset, bool, error) {
	if s.DB == nil {
		return Offset{}, false, fmt.Errorf("mysql offset store db is required")
	}
	row := s.DB.QueryRowContext(ctx, `
SELECT reader_name, binlog_file, binlog_pos, gtid, updated_at
FROM sync_upload_offset
WHERE reader_name = ?
`, readerName)

	var offset Offset
	var binlogFile sql.NullString
	var binlogPos sql.NullInt64
	var gtid sql.NullString
	if err := row.Scan(&offset.ReaderName, &binlogFile, &binlogPos, &gtid, &offset.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return Offset{}, false, nil
		}
		return Offset{}, false, fmt.Errorf("load cdc offset: %w", err)
	}
	if binlogFile.Valid {
		offset.BinlogFile = binlogFile.String
	}
	if binlogPos.Valid {
		offset.BinlogPos = uint32(binlogPos.Int64)
	}
	if gtid.Valid {
		offset.GTID = gtid.String
	}
	return offset, true, nil
}

func (s *MySQLOffsetStore) Save(ctx context.Context, offset Offset) error {
	if s.DB == nil {
		return fmt.Errorf("mysql offset store db is required")
	}
	if err := offset.Validate(); err != nil {
		return err
	}
	if offset.UpdatedAt.IsZero() {
		offset.UpdatedAt = s.now()
	}

	_, err := s.DB.ExecContext(ctx, `
INSERT INTO sync_upload_offset (reader_name, binlog_file, binlog_pos, gtid, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  binlog_file = VALUES(binlog_file),
  binlog_pos = VALUES(binlog_pos),
  gtid = VALUES(gtid),
  updated_at = VALUES(updated_at)
`, offset.ReaderName, nullableString(offset.BinlogFile), nullablePos(offset.BinlogPos), nullableString(offset.GTID), offset.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save cdc offset: %w", err)
	}
	return nil
}

func (s *MySQLOffsetStore) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullablePos(value uint32) any {
	if value == 0 {
		return nil
	}
	return int64(value)
}
