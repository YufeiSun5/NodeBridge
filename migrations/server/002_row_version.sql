CREATE TABLE IF NOT EXISTS sync_row_version (
    row_hash BINARY(32) NOT NULL,
    row_identity BLOB NOT NULL,
    version_json JSON NULL,
    PRIMARY KEY (row_hash)
) ENGINE=InnoDB;
