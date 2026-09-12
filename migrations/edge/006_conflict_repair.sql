CREATE TABLE IF NOT EXISTS sync_conflict_state (
    row_hash BINARY(32) NOT NULL,
    table_hash BINARY(32) NOT NULL,
    row_identity VARBINARY(8192) NOT NULL,
    state_json JSON NOT NULL,
    repair_required BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (row_hash),
    KEY idx_pending_repair (repair_required,table_hash,row_hash)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS sync_repair_replay (
    event_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    node_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    database_name VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    table_name VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    operation VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    PRIMARY KEY (event_id)
) ENGINE=InnoDB;
