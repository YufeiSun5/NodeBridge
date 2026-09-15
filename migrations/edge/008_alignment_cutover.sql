CREATE TABLE IF NOT EXISTS sync_alignment_cutover (
    job_id CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    proof_id CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    proof_json JSON NOT NULL,
    phase VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    peer_node_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (job_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS sync_alignment_event (
    message_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    proof_id CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    event_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    event_json LONGBLOB NOT NULL,
    reason VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    recorded_at DATETIME(6) NOT NULL,
    PRIMARY KEY (message_hash),
    KEY idx_alignment_event_id (event_id)
) ENGINE=InnoDB;
