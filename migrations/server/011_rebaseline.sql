CREATE TABLE IF NOT EXISTS sync_rebaseline_generation (
    singleton TINYINT NOT NULL PRIMARY KEY,
    plan_json JSON NOT NULL,
    prepared BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS sync_rebaseline_retired (
    proof_id CHAR(64) NOT NULL PRIMARY KEY,
    proof_json JSON NOT NULL
) ENGINE=InnoDB;
