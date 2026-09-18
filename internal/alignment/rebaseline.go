package alignment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/atomicfile"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
)

type RebaselineRequest struct {
	MigrationID string           `json:"migration_id"`
	EdgeNode    string           `json:"edge_node_id"`
	ServerNode  string           `json:"server_node_id"`
	Rules       []rules.SyncRule `json:"rules,omitempty"`
}

type RebaselineRule struct {
	Rule rules.SyncRule `json:"rule"`
}
type RebaselineTable struct {
	Schema      rulecheck.Schema `json:"schema"`
	BackupTable string           `json:"backup_table,omitempty"`
}
type RebaselinePlan struct {
	ID             string            `json:"plan_id"`
	MigrationID    string            `json:"migration_id"`
	EdgeNode       string            `json:"edge_node_id"`
	ServerNode     string            `json:"server_node_id"`
	NodeID         string            `json:"node_id"`
	Mode           string            `json:"mode"`
	MySQLUUID      string            `json:"mysql_uuid"`
	OldDatabase    string            `json:"old_system_database"`
	NewDatabase    string            `json:"new_system_database"`
	NewCDCFilter   string            `json:"new_cdc_filter"`
	ConfigRevision string            `json:"config_revision"`
	RulesRevision  string            `json:"rules_revision"`
	Rules          []RebaselineRule  `json:"rules"`
	Tables         []RebaselineTable `json:"local_tables"`
	Retired        []CutoverProof    `json:"retired_proofs"`
}

type RebaselineApply struct {
	Plan                 RebaselinePlan `json:"plan"`
	Confirm              bool           `json:"confirm"`
	TargetWritersStopped bool           `json:"target_writers_stopped"`
}

type RebaselineResult struct {
	Prepared        bool   `json:"prepared"`
	PlanID          string `json:"plan_id"`
	Database        string `json:"system_database"`
	BackupDirectory string `json:"backup_directory"`
	Next            string `json:"next"`
}

var migrationName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func generationDatabase(migration, node string) string {
	return "nb_gen_" + hash([]string{migration, node})[:32]
}
func bytesRevision(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func rebaselineFilter(tables []RebaselineTable) string {
	filters := make([]string, len(tables))
	for i, table := range tables {
		filters[i] = regexp.QuoteMeta(table.Schema.Database) + `\.` + regexp.QuoteMeta(table.Schema.Table)
	}
	return strings.Join(filters, ",")
}

func (p RebaselinePlan) Validate() error {
	id := p.ID
	p.ID = ""
	if id == "" || hash(p) != id || !migrationName.MatchString(p.MigrationID) || !migrationName.MatchString(p.EdgeNode) || !migrationName.MatchString(p.ServerNode) || p.EdgeNode == p.ServerNode || p.MySQLUUID == "" || p.OldDatabase == p.NewDatabase || p.NewDatabase != generationDatabase(p.MigrationID, p.NodeID) {
		return errors.New("rebaseline_plan_invalid")
	}
	if p.Mode == appconfig.ModeEdge && p.NodeID != p.EdgeNode || p.Mode == appconfig.ModeServer && p.NodeID != p.ServerNode || p.Mode != appconfig.ModeEdge && p.Mode != appconfig.ModeServer {
		return errors.New("rebaseline_role_invalid")
	}
	if len(p.Rules) == 0 || len(p.Rules) > 256 || len(p.Tables) != len(p.Rules) {
		return errors.New("rebaseline_rule_count_invalid")
	}
	if p.NewCDCFilter != rebaselineFilter(p.Tables) {
		return errors.New("rebaseline_capture_filter_invalid")
	}
	if err := mapper.ValidateIdentifier(p.OldDatabase); err != nil {
		return err
	}
	for i, entry := range p.Tables {
		rule := p.Rules[i].Rule
		database, table := rule.DatabaseName, rule.TableName
		if p.Mode == appconfig.ModeServer {
			if rule.TargetDatabaseName != "" {
				database = rule.TargetDatabaseName
			}
			if rule.TargetTableName != "" {
				table = rule.TargetTableName
			}
		}
		if entry.Schema.Database != database || entry.Schema.Table != table || validateTable(database, table) != nil {
			return errors.New("rebaseline_table_scope_invalid")
		}
		if p.Mode == appconfig.ModeServer && entry.BackupTable != "nb_backup_"+hash([]string{database, table})[:32] {
			return errors.New("rebaseline_backup_invalid")
		}
		for _, column := range entry.Schema.Columns {
			if err := mapper.ValidateIdentifier(column.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizedRebaselineRules(req RebaselineRequest, existing []rules.SyncRule) ([]RebaselineRule, error) {
	if !migrationName.MatchString(req.MigrationID) || !migrationName.MatchString(req.EdgeNode) || !migrationName.MatchString(req.ServerNode) || req.EdgeNode == req.ServerNode {
		return nil, errors.New("rebaseline_identity_invalid")
	}
	input := req.Rules
	if input == nil {
		input = existing
	}
	set := rules.RuleSet{Rules: slices.Clone(input)}
	if len(input) == 0 || len(input) > 256 {
		return nil, errors.New("rebaseline_rule_count_invalid")
	}
	for i := range set.Rules {
		r := &set.Rules[i]
		r.Enable = false
		r.InitialAlignment.Policy = rules.AlignmentManual
		if r.Direction != rules.DirectionEdgeToServer && r.Direction != rules.DirectionBidirectional {
			return nil, errors.New("rebaseline_edge_authority_required: only EDGE_TO_SERVER and BIDIRECTIONAL supported")
		}
		if len(r.SourceNodeIDs) > 0 && (len(r.SourceNodeIDs) != 1 || r.SourceNodeIDs[0] != req.EdgeNode) || len(r.DispatchNodeIDs) > 0 && (len(r.DispatchNodeIDs) != 1 || r.DispatchNodeIDs[0] != req.EdgeNode) {
			return nil, errors.New("rebaseline_pair_only: multi-member migration is not supported")
		}
		if r.Direction == rules.DirectionBidirectional && r.ConflictPolicy != rules.ConflictLastWriteWin {
			return nil, errors.New("rebaseline_bidirectional_lww_required")
		}
	}
	if err := set.Validate(); err != nil {
		return nil, err
	}
	slices.SortFunc(set.Rules, func(a, b rules.SyncRule) int { return strings.Compare(a.ID, b.ID) })
	result := make([]RebaselineRule, len(set.Rules))
	for i, r := range set.Rules {
		result[i] = RebaselineRule{r}
	}
	return result, nil
}

func openRebaselineDB(cfg *appconfig.Config, database string) (*sql.DB, error) {
	dsn, err := mysql.ParseDSN(mysqlconn.DSN(cfg.MySQL))
	if err != nil {
		return nil, err
	}
	dsn.DBName = database
	dsn.Timeout = 5 * time.Second
	dsn.ReadTimeout = 15 * time.Minute
	dsn.WriteTimeout = 15 * time.Minute
	return mysqlconn.OpenDSN(dsn.FormatDSN())
}

func PlanRebaseline(ctx context.Context, configPath, rulesPath string, req RebaselineRequest) (RebaselinePlan, error) {
	var p RebaselinePlan
	if err := CheckRebaselinePreparation(configPath); err != nil {
		return p, err
	}
	cfg, err := appconfig.LoadFile(configPath)
	if err != nil {
		return p, err
	}
	set, revision, err := rules.LoadFileWithRevision(rulesPath)
	if err != nil {
		return p, err
	}
	p.Rules, err = normalizedRebaselineRules(req, set.Rules)
	if err != nil {
		return p, err
	}
	p.MigrationID, p.EdgeNode, p.ServerNode = req.MigrationID, req.EdgeNode, req.ServerNode
	p.NodeID, p.Mode, p.OldDatabase = cfg.Node.ID, cfg.Mode, cfg.MySQL.Database
	p.NewDatabase = generationDatabase(req.MigrationID, cfg.Node.ID)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return p, err
	}
	p.ConfigRevision, p.RulesRevision = bytesRevision(data), revision
	db, err := openRebaselineDB(cfg, p.OldDatabase)
	if err != nil {
		return p, err
	}
	defer db.Close()
	if err := db.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&p.MySQLUUID); err != nil {
		return p, err
	}
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", p.NewDatabase).Scan(&exists); err != nil {
		return p, err
	}
	if exists != 0 {
		return p, errors.New("rebaseline_generation_exists: use the original plan to resume, or a new migration_id")
	}
	seen := map[string]bool{}
	for _, item := range p.Rules {
		database, table := item.Rule.DatabaseName, item.Rule.TableName
		if cfg.Mode == appconfig.ModeServer {
			if item.Rule.TargetDatabaseName != "" {
				database = item.Rule.TargetDatabaseName
			}
			if item.Rule.TargetTableName != "" {
				table = item.Rule.TargetTableName
			}
		}
		if strings.EqualFold(database, p.OldDatabase) || strings.EqualFold(database, p.NewDatabase) {
			return p, errors.New("rebaseline_requires_separate_business_database")
		}
		key := strings.ToLower(database + "." + table)
		if seen[key] {
			return p, errors.New("rebaseline_overlapping_local_tables")
		}
		seen[key] = true
		schema, err := rulecheck.ReadSchema(ctx, db, database, table)
		if err != nil {
			return p, err
		}
		if !strings.EqualFold(schema.Engine, "InnoDB") || len(schema.PrimaryKeys) == 0 {
			return p, errors.New("rebaseline_requires_innodb_primary_key")
		}
		entry := RebaselineTable{Schema: schema}
		if cfg.Mode == appconfig.ModeServer {
			if err := checkReplacementConstraints(ctx, db, schema); err != nil {
				return p, err
			}
			entry.BackupTable = "nb_backup_" + hash([]string{database, table})[:32]
		}
		p.Tables = append(p.Tables, entry)
	}
	p.Retired, err = collectRetiredProofs(ctx, db, req.EdgeNode, req.ServerNode)
	if err != nil {
		return p, err
	}
	p.NewCDCFilter = rebaselineFilter(p.Tables)
	p.ID = hash(p)
	return p, p.Validate()
}

func checkReplacementConstraints(ctx context.Context, db *sql.DB, s rulecheck.Schema) error {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE EVENT_OBJECT_SCHEMA=? AND EVENT_OBJECT_TABLE=?", s.Database, s.Table).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("rebaseline_target_triggers_unsupported: %s.%s", s.Database, s.Table)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_NAME IS NOT NULL AND ((TABLE_SCHEMA=? AND TABLE_NAME=?) OR (REFERENCED_TABLE_SCHEMA=? AND REFERENCED_TABLE_NAME=?))", s.Database, s.Table, s.Database, s.Table).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("rebaseline_target_foreign_keys_unsupported: %s.%s", s.Database, s.Table)
	}
	return nil
}

func collectRetiredProofs(ctx context.Context, db *sql.DB, edge, server string) ([]CutoverProof, error) {
	intents, err := db.QueryContext(ctx, "SELECT intent_json FROM sync_alignment_topology")
	if err != nil && !isMissingTable(err) {
		return nil, err
	}
	if err == nil {
		for intents.Next() {
			var data []byte
			var intent TopologyIntent
			if err := intents.Scan(&data); err != nil {
				intents.Close()
				return nil, err
			}
			if json.Unmarshal(data, &intent) != nil || intent.Validate() != nil || intent.ServerNode != server || len(intent.Members) != 1 || intent.Members[0].EdgeNode != edge {
				intents.Close()
				return nil, errors.New("rebaseline_existing_other_member: complete topology migration required")
			}
		}
		err := intents.Err()
		intents.Close()
		if err != nil {
			return nil, err
		}
	}
	rows, err := db.QueryContext(ctx, "SELECT proof_json FROM sync_alignment_cutover ORDER BY proof_id")
	if isMissingTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var proofs []CutoverProof
	for rows.Next() {
		var data []byte
		var p CutoverProof
		if err = rows.Scan(&data); err != nil {
			break
		}
		if err = json.Unmarshal(data, &p); err != nil {
			break
		}
		if err = validateRetiredPair(p, edge, server); err != nil {
			break
		}
		proofs = append(proofs, p)
	}
	err = errors.Join(err, rows.Err())
	rows.Close()
	if err != nil {
		return nil, err
	}
	previous, err := readRetiredProofs(ctx, db)
	if err != nil {
		return nil, err
	}
	for _, p := range previous {
		if err := validateRetiredPair(p, edge, server); err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(proofs, func(v CutoverProof) bool { return v.ID == p.ID }) {
			proofs = append(proofs, p)
		}
	}
	slices.SortFunc(proofs, func(a, b CutoverProof) int { return strings.Compare(a.ID, b.ID) })
	return proofs, nil
}

func CheckRebaselinePreparation(configPath string) error {
	_, err := os.Stat(configPath + ".rebaseline-pending.json")
	if err == nil {
		return errors.New("rebaseline_preparation_pending: resume the original preparation before starting")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func migrationDirectory(mode string) string {
	exe, _ := os.Executable()
	path := filepath.Join(filepath.Dir(exe), "migrations", mode)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return filepath.Join("migrations", mode)
	}
	return path
}

// PrepareRebaseline preserves the old system database as a complete immutable
// generation. Only explicitly listed target tables are backed up and emptied.
// The SQL receipt and the backup/DELETE commit together; retries never erase a
// target twice, including after a lost commit response or configuration write.
func PrepareRebaseline(ctx context.Context, configPath, rulesPath string, req RebaselineApply) (RebaselineResult, error) {
	var result RebaselineResult
	p := req.Plan
	if !req.Confirm || !req.TargetWritersStopped {
		return result, errors.New("rebaseline_confirmation_and_target_maintenance_required")
	}
	if err := p.Validate(); err != nil {
		return result, err
	}
	lock, err := agentstate.Lock(configPath + ".agent.lock")
	if err != nil {
		return result, fmt.Errorf("rebaseline_requires_stopped_agent: %w", err)
	}
	defer lock.Close()
	backup := configPath + ".rebaseline-" + p.ID
	pending := configPath + ".rebaseline-pending.json"
	encoded, _ := json.Marshal(p)
	stored, readErr := os.ReadFile(pending)
	if errors.Is(readErr, os.ErrNotExist) {
		// A completed preparation can also be retried without restoring old files.
		if done, err := os.ReadFile(filepath.Join(backup, "prepared.json")); err == nil && string(done) == string(encoded) {
			current, err := appconfig.LoadFile(configPath)
			if err != nil {
				return result, err
			}
			if current.MySQL.Database != p.NewDatabase {
				return result, errors.New("rebaseline_completed_generation_no_longer_current")
			}
			return RebaselineResult{true, p.ID, p.NewDatabase, backup, "Run initial alignment on BOTH nodes for every planned rule; Agents remain stopped."}, nil
		}
		wanted := RebaselineRequest{MigrationID: p.MigrationID, EdgeNode: p.EdgeNode, ServerNode: p.ServerNode}
		for _, item := range p.Rules {
			wanted.Rules = append(wanted.Rules, item.Rule)
		}
		current, err := PlanRebaseline(ctx, configPath, rulesPath, wanted)
		if err != nil {
			return result, err
		}
		if current.ID != p.ID {
			return result, errors.New("rebaseline_plan_stale")
		}
		if err := os.MkdirAll(backup, 0700); err != nil {
			return result, err
		}
		for _, path := range []string{configPath, rulesPath, rulesPath + ".pairs.json"} {
			data, err := os.ReadFile(path)
			if errors.Is(err, os.ErrNotExist) && path == rulesPath+".pairs.json" {
				continue
			}
			if err != nil {
				return result, err
			}
			if path == configPath && bytesRevision(data) != p.ConfigRevision {
				return result, errors.New("rebaseline_config_changed")
			}
			name := filepath.Join(backup, filepath.Base(path))
			if err := atomicfile.Write(name, data, 0600); err != nil {
				return result, err
			}
		}
		if err := atomicfile.Write(pending, encoded, 0600); err != nil {
			return result, err
		}
	} else if readErr != nil {
		return result, readErr
	} else if string(stored) != string(encoded) {
		return result, errors.New("rebaseline_different_preparation_pending")
	}
	cfg, err := appconfig.LoadFile(filepath.Join(backup, filepath.Base(configPath)))
	if err != nil {
		return result, err
	}
	currentCfg, err := appconfig.LoadFile(configPath)
	if err != nil {
		return result, err
	}
	if currentCfg.MySQL.Database != p.OldDatabase && currentCfg.MySQL.Database != p.NewDatabase {
		return result, errors.New("rebaseline_config_changed")
	}
	currentCfg.MySQL.Database = cfg.MySQL.Database
	if currentCfg.CDC.Filter != cfg.CDC.Filter && currentCfg.CDC.Filter != p.NewCDCFilter {
		return result, errors.New("rebaseline_config_changed")
	}
	currentCfg.CDC.Filter = cfg.CDC.Filter
	if hash(currentCfg) != hash(cfg) {
		return result, errors.New("rebaseline_config_changed")
	}
	set := rules.RuleSet{}
	for _, item := range p.Rules {
		set.Rules = append(set.Rules, item.Rule)
	}
	existing, revision, err := rules.LoadFileWithRevision(rulesPath)
	if err != nil {
		return result, err
	}
	if revision != p.RulesRevision && hash(existing) != hash(&set) {
		return result, errors.New("rebaseline_rules_changed")
	}
	manifest, err := os.ReadFile(rulesPath + ".pairs.json")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	if err == nil {
		original, err := os.ReadFile(filepath.Join(backup, filepath.Base(rulesPath)+".pairs.json"))
		if err != nil || string(original) != string(manifest) {
			return result, errors.New("rebaseline_manifest_changed")
		}
	}
	oldDB, err := openRebaselineDB(cfg, p.OldDatabase)
	if err != nil {
		return result, err
	}
	defer oldDB.Close()
	var uuid string
	if err := oldDB.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&uuid); err != nil {
		return result, err
	}
	if uuid != p.MySQLUUID {
		return result, errors.New("rebaseline_mysql_server_changed")
	}
	if err := initializeGeneration(ctx, oldDB, cfg, p); err != nil {
		return result, err
	}
	newDB, err := openRebaselineDB(cfg, p.NewDatabase)
	if err != nil {
		return result, err
	}
	defer newDB.Close()
	if err := mysqlconn.RunMigrations(ctx, newDB, migrationDirectory(p.Mode)); err != nil {
		return result, err
	}
	if err := prepareGenerationTables(ctx, newDB, p); err != nil {
		return result, err
	}
	existing, revision, err = rules.LoadFileWithRevision(rulesPath)
	if err != nil {
		return result, err
	}
	if revision != p.RulesRevision && hash(existing) != hash(&set) {
		return result, errors.New("rebaseline_rules_changed")
	}
	if revision == p.RulesRevision {
		if _, err := rules.SaveFileCAS(rulesPath, set, revision); err != nil {
			return result, err
		}
	}
	cfg.MySQL.Database = p.NewDatabase
	cfg.CDC.Filter = p.NewCDCFilter
	current, err := appconfig.LoadFile(configPath)
	if err != nil {
		return result, err
	}
	if current.MySQL.Database != p.OldDatabase && current.MySQL.Database != p.NewDatabase {
		return result, errors.New("rebaseline_config_changed")
	}
	current.MySQL.Database = p.NewDatabase
	if current.CDC.Filter != currentCfg.CDC.Filter && current.CDC.Filter != p.NewCDCFilter {
		return result, errors.New("rebaseline_config_changed")
	}
	current.CDC.Filter = p.NewCDCFilter
	if hash(current) != hash(cfg) {
		return result, errors.New("rebaseline_config_changed")
	}
	if err := appconfig.SaveFile(configPath, *cfg); err != nil {
		return result, err
	}
	// The old manifest is copied above; no old proof is used for the new mapping.
	if err := os.Remove(rulesPath + ".pairs.json"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	if err := atomicfile.Write(filepath.Join(backup, "prepared.json"), encoded, 0600); err != nil {
		return result, err
	}
	if err := os.Remove(pending); err != nil {
		return result, err
	}
	return RebaselineResult{true, p.ID, p.NewDatabase, backup, "Run initial alignment on BOTH nodes for every planned rule; Agents remain stopped."}, nil
}

func initializeGeneration(ctx context.Context, old *sql.DB, cfg *appconfig.Config, p RebaselinePlan) error {
	var exists int
	if err := old.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", p.NewDatabase).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		if _, err := old.ExecContext(ctx, "CREATE DATABASE `"+p.NewDatabase+"`"); err != nil {
			return err
		}
	}
	db, err := openRebaselineDB(cfg, p.NewDatabase)
	if err != nil {
		return err
	}
	defer db.Close()
	var others int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME<>'sync_rebaseline_generation'", p.NewDatabase).Scan(&others); err != nil {
		return err
	}
	if others > 0 {
		var raw []byte
		if err := db.QueryRowContext(ctx, "SELECT plan_json FROM sync_rebaseline_generation WHERE singleton=1").Scan(&raw); err != nil {
			return errors.Join(errors.New("rebaseline_database_ownership_unknown"), err)
		}
		var actual RebaselinePlan
		if json.Unmarshal(raw, &actual) != nil || actual.ID != p.ID {
			return errors.New("rebaseline_database_owned_by_other_plan")
		}
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS sync_rebaseline_generation (singleton TINYINT NOT NULL PRIMARY KEY,plan_json JSON NOT NULL,prepared BOOLEAN NOT NULL DEFAULT FALSE,created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)) ENGINE=InnoDB"); err != nil {
		return err
	}
	raw, _ := json.Marshal(p)
	if _, err := db.ExecContext(ctx, "INSERT INTO sync_rebaseline_generation (singleton,plan_json) VALUES (1,?) ON DUPLICATE KEY UPDATE singleton=singleton", string(raw)); err != nil {
		return err
	}
	var actual []byte
	if err := db.QueryRowContext(ctx, "SELECT plan_json FROM sync_rebaseline_generation WHERE singleton=1").Scan(&actual); err != nil {
		return err
	}
	var saved RebaselinePlan
	if json.Unmarshal(actual, &saved) != nil || hash(saved) != hash(p) {
		return errors.New("rebaseline_database_owned_by_other_plan")
	}
	return nil
}

func prepareGenerationTables(ctx context.Context, db *sql.DB, p RebaselinePlan) error {
	var prepared bool
	if err := db.QueryRowContext(ctx, "SELECT prepared FROM sync_rebaseline_generation WHERE singleton=1").Scan(&prepared); err != nil {
		return err
	}
	if prepared {
		return nil
	}
	for _, entry := range p.Tables {
		actual, err := rulecheck.ReadSchema(ctx, db, entry.Schema.Database, entry.Schema.Table)
		if err != nil {
			return err
		}
		if hash(actual) != hash(entry.Schema) {
			return errors.New("rebaseline_schema_changed")
		}
		if p.Mode == appconfig.ModeServer {
			if err := checkReplacementConstraints(ctx, db, entry.Schema); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS `"+entry.BackupTable+"` LIKE "+qualified(entry.Schema)); err != nil {
				return err
			}
		}
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, "SELECT prepared FROM sync_rebaseline_generation WHERE singleton=1 FOR UPDATE").Scan(&prepared); err != nil {
		return err
	}
	if prepared {
		return nil
	}
	for _, entry := range p.Tables {
		if p.Mode != appconfig.ModeServer {
			continue
		}
		if _, err := lockSnapshotTable(ctx, tx, entry.Schema); err != nil {
			return err
		}
		actual, err := rulecheck.ReadSchema(ctx, tx, entry.Schema.Database, entry.Schema.Table)
		if err != nil {
			return err
		}
		if hash(actual) != hash(entry.Schema) {
			return errors.New("rebaseline_schema_changed")
		}
		var columns []string
		for _, column := range entry.Schema.Columns {
			if !column.Generated() {
				columns = append(columns, "`"+column.Name+"`")
			}
		}
		joined := strings.Join(columns, ",")
		if _, err := tx.ExecContext(ctx, "INSERT INTO `"+entry.BackupTable+"` ("+joined+") SELECT "+joined+" FROM "+qualified(entry.Schema)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+qualified(entry.Schema)); err != nil {
			return err
		}
	}
	for _, proof := range p.Retired {
		data, _ := json.Marshal(proof)
		if _, err := tx.ExecContext(ctx, "INSERT INTO sync_rebaseline_retired (proof_id,proof_json) VALUES (?,?) ON DUPLICATE KEY UPDATE proof_id=proof_id", proof.ID, string(data)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sync_rebaseline_generation SET prepared=TRUE WHERE singleton=1"); err != nil {
		return err
	}
	return tx.Commit()
}
