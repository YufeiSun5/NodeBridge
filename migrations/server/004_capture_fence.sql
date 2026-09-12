CREATE TABLE IF NOT EXISTS sync_capture_fence (
    node_id VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    token CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    PRIMARY KEY (node_id)
) ENGINE=InnoDB;
