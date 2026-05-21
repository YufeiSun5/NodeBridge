CREATE TABLE IF NOT EXISTS alarm_history (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  alarm_code VARCHAR(128) NOT NULL,
  alarm_message VARCHAR(512) NOT NULL,
  alarm_level VARCHAR(32) NOT NULL,
  occurred_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
);

CREATE TABLE IF NOT EXISTS device_config (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(128) NOT NULL,
  value VARCHAR(512) NULL,
  sync_version BIGINT NOT NULL DEFAULT 0,
  updated_by_node VARCHAR(64) NULL,
  last_event_id VARCHAR(128) NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  is_deleted TINYINT NOT NULL DEFAULT 0,
  deleted_at DATETIME(3) NULL,
  deleted_by_node VARCHAR(64) NULL,
  KEY idx_last_event_id (last_event_id),
  KEY idx_updated_at (updated_at)
);

CREATE TABLE IF NOT EXISTS sync_node_info (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  node_id VARCHAR(64) NOT NULL,
  node_name VARCHAR(128) NOT NULL,
  node_type VARCHAR(32) NOT NULL,
  version VARCHAR(64) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_node_id (node_id)
);

CREATE TABLE IF NOT EXISTS sync_apply_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  event_id VARCHAR(128) NOT NULL,
  origin_node_id VARCHAR(64) NOT NULL,
  source_node_id VARCHAR(64) NOT NULL,
  target_node_id VARCHAR(64) NOT NULL,
  database_name VARCHAR(128) NOT NULL,
  table_name VARCHAR(128) NOT NULL,
  pk_value VARCHAR(512) NOT NULL,
  op_type VARCHAR(16) NOT NULL,
  applied_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_event_id (event_id),
  KEY idx_table_pk (table_name, pk_value),
  KEY idx_applied_at (applied_at)
);

CREATE TABLE IF NOT EXISTS sync_upload_offset (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  reader_name VARCHAR(64) NOT NULL,
  binlog_file VARCHAR(255) NULL,
  binlog_pos BIGINT NULL,
  gtid VARCHAR(512) NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_reader_name (reader_name)
);

CREATE TABLE IF NOT EXISTS sync_rule_snapshot (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  rule_id VARCHAR(128) NOT NULL,
  database_name VARCHAR(128) NOT NULL,
  table_name VARCHAR(128) NOT NULL,
  direction VARCHAR(64) NOT NULL,
  conflict_policy VARCHAR(64) NOT NULL,
  enabled TINYINT NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_rule_id (rule_id)
);

CREATE TABLE IF NOT EXISTS sync_error_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  event_id VARCHAR(128) NULL,
  module VARCHAR(64) NOT NULL,
  error_message TEXT NOT NULL,
  created_at DATETIME(3) NOT NULL,
  KEY idx_event_id (event_id),
  KEY idx_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS sync_runtime_status (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  status_key VARCHAR(128) NOT NULL,
  status_value TEXT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_status_key (status_key)
);
