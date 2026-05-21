CREATE TABLE IF NOT EXISTS alarm_history (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  origin_node_id VARCHAR(64) NOT NULL,
  alarm_code VARCHAR(128) NOT NULL,
  alarm_message VARCHAR(512) NOT NULL,
  alarm_level VARCHAR(32) NOT NULL,
  occurred_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_origin_node (origin_node_id),
  KEY idx_occurred_at (occurred_at)
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

CREATE TABLE IF NOT EXISTS device_settings (
  setting_id BIGINT PRIMARY KEY,
  display_name VARCHAR(128) NOT NULL,
  setting_value VARCHAR(512) NULL,
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

CREATE TABLE IF NOT EXISTS sync_node_registry (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  node_id VARCHAR(64) NOT NULL,
  node_name VARCHAR(128) NOT NULL,
  node_type VARCHAR(32) NOT NULL,
  location VARCHAR(255) NULL,
  status VARCHAR(32) NOT NULL,
  last_heartbeat_at DATETIME(3) NULL,
  last_upload_at DATETIME(3) NULL,
  last_downlink_at DATETIME(3) NULL,
  version VARCHAR(64) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_node_id (node_id)
);

CREATE TABLE IF NOT EXISTS sync_event_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  event_id VARCHAR(128) NOT NULL,
  origin_node_id VARCHAR(64) NOT NULL,
  source_node_id VARCHAR(64) NOT NULL,
  database_name VARCHAR(128) NOT NULL,
  table_name VARCHAR(128) NOT NULL,
  target_database_name VARCHAR(128) NULL,
  target_table_name VARCHAR(128) NULL,
  pk_value VARCHAR(512) NOT NULL,
  op_type VARCHAR(16) NOT NULL,
  direction VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  event_time DATETIME(3) NOT NULL,
  received_at DATETIME(3) NOT NULL,
  applied_at DATETIME(3) NULL,
  error_message TEXT NULL,
  event_payload LONGTEXT NULL,
  UNIQUE KEY uk_event_id (event_id),
  KEY idx_table_pk (table_name, pk_value),
  KEY idx_origin_node (origin_node_id),
  KEY idx_received_at (received_at)
);

CREATE TABLE IF NOT EXISTS sync_ack_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  event_id VARCHAR(128) NOT NULL,
  target_node_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  ack_at DATETIME(3) NULL,
  error_message TEXT NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_event_target (event_id, target_node_id)
);

CREATE TABLE IF NOT EXISTS sync_conflict_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  conflict_id VARCHAR(128) NOT NULL,
  table_name VARCHAR(128) NOT NULL,
  pk_value VARCHAR(512) NOT NULL,
  local_event_id VARCHAR(128) NOT NULL,
  incoming_event_id VARCHAR(128) NOT NULL,
  local_node_id VARCHAR(64) NULL,
  incoming_node_id VARCHAR(64) NULL,
  conflict_type VARCHAR(64) NOT NULL,
  policy VARCHAR(64) NOT NULL,
  resolved_result VARCHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  resolved_at DATETIME(3) NULL,
  UNIQUE KEY uk_conflict_id (conflict_id)
);

CREATE TABLE IF NOT EXISTS sync_dispatch_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  event_id VARCHAR(128) NOT NULL,
  target_node_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  dispatched_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_event_target (event_id, target_node_id)
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

CREATE TABLE IF NOT EXISTS sync_rule_config (
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
