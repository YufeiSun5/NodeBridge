CREATE TABLE IF NOT EXISTS sync_conflict_event (
    row_hash BINARY(32) NOT NULL,
    event_hash BINARY(32) NOT NULL,
    event_identity VARBINARY(1024) NOT NULL,
    version_json JSON NOT NULL,
    decision VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    PRIMARY KEY (event_hash),
    KEY idx_conflict_row (row_hash)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS sync_conflict_schema (
    table_hash BINARY(32) NOT NULL,
    table_identity VARBINARY(1024) NOT NULL,
    key_schema JSON NOT NULL,
    PRIMARY KEY (table_hash)
) ENGINE=InnoDB;
