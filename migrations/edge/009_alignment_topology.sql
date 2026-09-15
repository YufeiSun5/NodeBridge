CREATE TABLE IF NOT EXISTS sync_alignment_topology (
    intent_id CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    scope_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    pending_scope CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    node_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    intent_json JSON NOT NULL,
    topology_json JSON NULL,
    phase VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (intent_id),
    UNIQUE KEY uk_alignment_pending_scope (pending_scope),
    KEY idx_alignment_topology_scope (scope_hash)
) ENGINE=InnoDB;
