CREATE TABLE IF NOT EXISTS sync_delete_replay (
    event_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    origin_node_id VARCHAR(128) NOT NULL,
    database_name VARCHAR(128) NOT NULL,
    table_name VARCHAR(128) NOT NULL,
    PRIMARY KEY (event_id)
) ENGINE=InnoDB;
