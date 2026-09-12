CREATE TABLE IF NOT EXISTS sync_alignment_job (
    job_id CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    scope_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    plan_id CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    node_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    endpoint_role VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    phase VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    plan_json JSON NOT NULL,
    row_count BIGINT UNSIGNED NULL,
    row_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    capture_marker CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
    capture_boundary JSON NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (job_id),
    UNIQUE KEY uq_alignment_scope (scope_hash),
    KEY idx_alignment_phase (phase)
) ENGINE=InnoDB;
