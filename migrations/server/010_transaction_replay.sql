CREATE TABLE IF NOT EXISTS sync_replay_marker (
    token CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    phase VARCHAR(5) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    database_name VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    table_name VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    PRIMARY KEY (token, phase)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS sync_replay_position (
    server_uuid CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    binlog_file VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    binlog_pos BIGINT UNSIGNED NOT NULL,
    phase VARCHAR(5) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    token CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    database_name VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    table_name VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    PRIMARY KEY (server_uuid, binlog_file, binlog_pos, phase)
) ENGINE=InnoDB;
