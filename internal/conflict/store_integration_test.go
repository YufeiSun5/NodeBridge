package conflict_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/conflict"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
)

func TestRealMySQLVersionTransactionsAndEmptyDirection(t *testing.T) {
	raw := os.Getenv("NODEBRIDGE_ALIGNMENT_TEST_DSN")
	if raw == "" {
		t.Skip("isolated loopback NODEBRIDGE_ALIGNMENT_TEST_DSN required")
	}
	cfg, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatal("invalid test DSN")
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil || cfg.Net != "tcp" || cfg.DBName != "" || (host != "127.0.0.1" && host != "::1") {
		t.Fatal("test requires loopback TCP and no existing database")
	}
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 5*time.Second, 10*time.Second, 10*time.Second
	cfg.InterpolateParams = true
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	name := fmt.Sprintf("nb_alignment_core_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"`"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		c, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if _, err := admin.ExecContext(c, "DROP DATABASE `"+name+"`"); err != nil {
			t.Errorf("owned database cleanup: %v", err)
		}
	}()
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/server"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"CREATE TABLE source_rows (id BIGINT PRIMARY KEY, value BIGINT NOT NULL) ENGINE=InnoDB", "CREATE TABLE target_rows (id BIGINT PRIMARY KEY, value BIGINT NOT NULL) ENGINE=InnoDB", "CREATE TABLE owned_receipts (event_id VARCHAR(100) PRIMARY KEY, decision VARCHAR(20)) ENGINE=InnoDB", "INSERT INTO source_rows VALUES (1,0)"} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	l, err := alignment.Observe(ctx, db, "edge", name, "source_rows")
	if err != nil {
		t.Fatal(err)
	}
	r, err := alignment.Observe(ctx, db, "server", name, "target_rows")
	if err != nil {
		t.Fatal(err)
	}
	rule := rules.SyncRule{ID: "owned", DatabaseName: name, TableName: "source_rows", TargetDatabaseName: name, TargetTableName: "target_rows", PrimaryKeys: []string{"id"}, DeleteMode: rules.DeleteHard, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}}
	p, err := alignment.BuildPlan(rule, l, r, time.Now())
	if err != nil || p.Direction != alignment.ToRight {
		t.Fatalf("plan %s %v", p.Direction, err)
	}
	copyResult, err := alignment.CopySnapshot(ctx, db, db, p, rule, true)
	if err != nil || copyResult.Rows != 1 || len(copyResult.Digest) != 64 {
		t.Fatalf("snapshot copy %+v %v", copyResult, err)
	}
	if _, err := alignment.CopySnapshot(ctx, db, db, p, rule, true); err == nil {
		t.Fatal("copy into newly nonempty target accepted")
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM target_rows"); err != nil {
		t.Fatal(err)
	}
	verifyMappedSnapshotCopy(t, ctx, db, name)
	verifyTimestampSnapshot(t, ctx, name)
	verifyStreamSnapshot(t, ctx, db, name)
	verifyPreparedSnapshot(t, ctx, db, name)
	if _, err := db.ExecContext(ctx, "INSERT INTO target_rows VALUES (1,1)"); err != nil {
		t.Fatal(err)
	}
	r, err = alignment.Observe(ctx, db, "server", name, "target_rows")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alignment.BuildPlan(rule, l, r, time.Now()); err == nil {
		t.Fatal("both nonempty incorrectly accepted")
	}
	key := conflict.RowKey{Database: name, Table: "source_rows", CanonicalKey: []byte("BIGINT:1")}
	store := conflict.Store{DB: db}
	makeVersion := func(i int) conflict.Version {
		return conflict.Version{Time: time.Unix(1000+int64(i), 0), TimeSource: conflict.SourceBinlog, OriginNodeID: fmt.Sprintf("node-%d", i%3), EventID: fmt.Sprintf("event-%d", i), PayloadHash: strings.Repeat("a", 64), BinlogFile: "mysql-bin.000001", BinlogPos: uint32(i + 4)}
	}
	applyVersion := func(i int, deleted, failReceipt bool) (string, error) {
		v := makeVersion(i)
		v.Deleted = deleted
		return store.Apply(ctx, key, v, func(ctx context.Context, tx *sql.Tx) error {
			var err error
			if deleted {
				_, err = tx.ExecContext(ctx, "DELETE FROM source_rows WHERE id=1")
			} else {
				_, err = tx.ExecContext(ctx, "UPDATE source_rows SET value=? WHERE id=1", i)
			}
			return err
		}, func(ctx context.Context, tx *sql.Tx, decision string) error {
			if failReceipt {
				return errors.New("owned injected receipt failure")
			}
			_, err := tx.ExecContext(ctx, "INSERT INTO owned_receipts VALUES (?,?) ON DUPLICATE KEY UPDATE event_id=event_id", v.EventID, decision)
			return err
		})
	}
	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for i := 1; i <= 24; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, err := applyVersion(i, false, false); errs <- err }(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var value int
	if err := db.QueryRowContext(ctx, "SELECT value FROM source_rows WHERE id=1").Scan(&value); err != nil || value != 24 {
		t.Fatalf("concurrent winner %d %v", value, err)
	}
	if _, err := applyVersion(25, false, true); err == nil {
		t.Fatal("injected receipt failure accepted")
	}
	if err := db.QueryRowContext(ctx, "SELECT value FROM source_rows WHERE id=1").Scan(&value); err != nil || value != 24 {
		t.Fatalf("failed transaction not rolled back: %d %v", value, err)
	}
	if got, err := applyVersion(25, true, false); err != nil || got != conflict.Apply {
		t.Fatalf("delete %s %v", got, err)
	}
	if got, err := applyVersion(24, false, false); err != nil || got != conflict.Duplicate {
		t.Fatalf("old retry %s %v", got, err)
	}
	if got, err := applyVersion(25, true, false); err != nil || got != conflict.Duplicate {
		t.Fatalf("duplicate delete %s %v", got, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM source_rows").Scan(&value); err != nil || value != 0 {
		t.Fatalf("deleted row resurrected: %d %v", value, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM owned_receipts").Scan(&value); err != nil || value != 25 {
		t.Fatalf("receipt count %d %v", value, err)
	}
	changed := makeVersion(24)
	changed.PayloadHash = strings.Repeat("b", 64)
	if _, err := store.Apply(ctx, key, changed, func(context.Context, *sql.Tx) error {
		t.Error("changed historical event wrote business row")
		return nil
	}, func(context.Context, *sql.Tx, string) error { return nil }); err == nil || !strings.Contains(err.Error(), "identity_reused") {
		t.Fatalf("superseded event identity reuse accepted: %v", err)
	}
	if got, err := applyVersion(0, false, false); err != nil || got != conflict.Superseded {
		t.Fatalf("unseen stale event: %s %v", got, err)
	}
	if got, err := applyVersion(0, false, false); err != nil || got != conflict.Duplicate {
		t.Fatalf("seen losing event: %s %v", got, err)
	}
	verifySQLCanonicalKeysAndLocalRecorder(t, ctx, db, name)
	verifySoftConflictRepair(t, ctx, db, name)
	verifySQLBidirectionalPair(t, ctx, db, name)
	t.Log("isolated MySQL: actual empty-table observation, 24 concurrent versions, atomic rollback, delete tombstone and stale retry verified; not production Apply/CDC integration")
}

func verifyMappedSnapshotCopy(t *testing.T, ctx context.Context, db *sql.DB, database string) {
	t.Helper()
	exec := func(q string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE TABLE copy_left (id BIGINT UNSIGNED PRIMARY KEY, amount DECIMAL(30,10) NOT NULL, raw_value VARBINARY(20), text_value VARCHAR(20), at_value DATETIME(6)) ENGINE=InnoDB")
	exec("CREATE TABLE copy_right (mapped_id BIGINT UNSIGNED PRIMARY KEY, mapped_amount DECIMAL(30,10) NOT NULL, raw_value VARBINARY(20), text_value VARCHAR(20), at_value DATETIME(6)) ENGINE=InnoDB")
	exec("INSERT INTO copy_left VALUES (9007199254740993,12345678901234567890.1234567890,X'00017F80FF',NULL,'2026-09-11 13:22:33.123456'),(18446744073709551615,0.0000000001,X'','',NULL)")
	rule := rules.SyncRule{ID: "copy-mapped", DatabaseName: database, TableName: "copy_left", TargetDatabaseName: database, TargetTableName: "copy_right", PrimaryKeys: []string{"id"}, TargetPrimaryKeys: []string{"mapped_id"}, DeleteMode: rules.DeleteHard, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}, ColumnMappings: []rules.ColumnMapping{{SourceColumn: "amount", TargetColumn: "mapped_amount"}}}
	plan := func() alignment.Plan {
		t.Helper()
		l, err := alignment.Observe(ctx, db, "left", database, "copy_left")
		if err != nil {
			t.Fatal(err)
		}
		r, err := alignment.Observe(ctx, db, "right", database, "copy_right")
		if err != nil {
			t.Fatal(err)
		}
		p, err := alignment.BuildPlan(rule, l, r, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := plan()
	got, err := alignment.CopySnapshot(ctx, db, db, p, rule, true)
	if err != nil || got.Rows != 2 {
		t.Fatalf("mapped precision copy %+v %v", got, err)
	}
	var binary, amount, date string
	if err := db.QueryRowContext(ctx, "SELECT HEX(raw_value),CAST(mapped_amount AS CHAR),CAST(at_value AS CHAR) FROM copy_right WHERE mapped_id=9007199254740993").Scan(&binary, &amount, &date); err != nil {
		t.Fatal(err)
	}
	if binary != "00017F80FF" || amount != "12345678901234567890.1234567890" || date != "2026-09-11 13:22:33.123456" {
		t.Fatalf("precision lost: %s %s %s", binary, amount, date)
	}
	exec("DELETE FROM copy_left")
	p = plan()
	if p.Direction != alignment.ToLeft {
		t.Fatalf("reverse direction %s", p.Direction)
	}
	reversed, err := alignment.CopySnapshot(ctx, db, db, p, rule, true)
	if err != nil || reversed.Rows != 2 || reversed.Digest != got.Digest {
		t.Fatalf("reverse copy %+v %v", reversed, err)
	}
	exec("DELETE FROM copy_right")
	exec("ALTER TABLE copy_right ADD CONSTRAINT positive_amount CHECK (mapped_amount >= 0)")
	exec("UPDATE copy_left SET amount=-1 WHERE id=18446744073709551615")
	p = plan()
	if _, err := alignment.CopySnapshot(ctx, db, db, p, rule, true); err == nil {
		t.Fatal("constraint failure accepted")
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM copy_right").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial copy persisted: %d %v", count, err)
	}
	exec("DELETE FROM copy_left")
	p = plan()
	if p.Direction != alignment.NoCopy {
		t.Fatal("both empty did not produce no copy")
	}
	if result, err := alignment.CopySnapshot(ctx, db, db, p, rule, true); err != nil || result.Rows != 0 {
		t.Fatalf("both empty %+v %v", result, err)
	}
	exec("INSERT INTO copy_left VALUES (1,1,NULL,NULL,NULL)")
	p = plan()
	exec("ALTER TABLE copy_right ADD COLUMN later_column INT NULL")
	if _, err := alignment.CopySnapshot(ctx, db, db, p, rule, true); err == nil || err.Error() != "alignment_schema_changed" {
		t.Fatalf("schema drift accepted: %v", err)
	}
	exec("ALTER TABLE copy_right DROP COLUMN later_column")
	exec("CREATE TRIGGER copy_guard BEFORE INSERT ON copy_right FOR EACH ROW SET NEW.mapped_amount=2")
	p = plan()
	if _, err := alignment.CopySnapshot(ctx, db, db, p, rule, true); err == nil || err.Error() != "alignment_trigger_check: table_triggers_unsupported" {
		t.Fatalf("trigger side effect accepted: %v", err)
	}
	exec("DROP TRIGGER copy_guard")
	t.Log("isolated snapshot copy: mapped PK/columns, exact unsigned/decimal/binary/null/empty/datetime, reverse empty direction, stale plan and transactional rollback verified; no CDC cutover")
}
