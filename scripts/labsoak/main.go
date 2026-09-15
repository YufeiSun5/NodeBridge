package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

var root = flag.String("root", "", "owned evidence directory")
var action = flag.String("action", "prepare", "prepare, workload, verify")
var run = flag.String("run", "", "unique lowercase identifier")
var mode = flag.String("mode", "server", "server or edge")
var migrations = flag.String("migrations", "migrations", "migration root")
var remotePort = flag.Int("remote-port", 13316, "SSH MySQL port")
var duration = flag.Duration("duration", 7*time.Hour, "writing duration")

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func write(name string, value any) {
	b, e := json.MarshalIndent(value, "", "  ")
	must(e)
	path := filepath.Join(*root, name)
	must(os.WriteFile(path+".tmp", b, 0600))
	must(os.Rename(path+".tmp", path))
}

func expectedRows(batches int) int {
	deleted := 0
	if batches > 2 {
		deleted = (batches - 2) * 10
	}
	return 100 + batches*20 - deleted
}
func open(port int, database string) *sql.DB {
	c := mysql.NewConfig()
	c.User = "root"
	c.Passwd = "root"
	c.Net = "tcp"
	c.Addr = fmt.Sprintf("127.0.0.1:%d", port)
	c.DBName = database
	c.Timeout = 5 * time.Second
	c.ReadTimeout = 20 * time.Second
	c.WriteTimeout = 20 * time.Second
	db, e := sql.Open("mysql", c.FormatDSN())
	must(e)
	db.SetMaxOpenConns(3)
	must(db.Ping())
	return db
}
func database(side string) string { return "nbsoak_" + *run + "_" + side }
func table(side string) string    { return side + "_rows" }
func prepare() {
	if *mode != "server" && *mode != "edge" {
		panic("invalid mode")
	}
	cfg, e := appconfig.LoadFile("C:/ProgramData/NodeBridge/config.yaml")
	must(e)
	db := open(3306, "")
	defer db.Close()
	_, e = db.Exec("CREATE DATABASE `" + database(*mode) + "`")
	must(e)
	db.Close()
	db = open(3306, database(*mode))
	defer db.Close()
	must(mysqlconn.RunMigrations(context.Background(), db, filepath.Join(*migrations, *mode)))
	_, e = db.Exec("CREATE TABLE " + table(*mode) + " (id BIGINT UNSIGNED PRIMARY KEY, amount DECIMAL(30,10) NOT NULL, raw_bytes VARBINARY(256), note VARCHAR(240), payload MEDIUMTEXT, stamp DATETIME(6), nullable_value VARCHAR(40), last_event_id VARCHAR(128) NOT NULL DEFAULT '', updated_by_node VARCHAR(64) NOT NULL DEFAULT '') ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin")
	must(e)
	if *mode == "server" {
		for i := 1; i <= 100; i++ {
			_, e = db.Exec("INSERT INTO server_rows (id,amount,raw_bytes,note,payload,stamp) VALUES (?,12345678901234567890.1234567890,?, 'initial snapshot', REPEAT('snapshot',1024), '2026-09-13 12:34:56.123456')", i, []byte{0, 128, 255})
			must(e)
		}
	}
	brokerURL, e := url.Parse(cfg.RabbitMQ.ServerURL)
	must(e)
	brokerURL.Path = "/nbsoak-" + *run
	brokerURL.RawPath = ""
	cfg.Node.ID = "soak-" + *run + "-" + *mode
	cfg.MySQL.Database = database(*mode)
	cfg.MySQL.Username = "root"
	cfg.MySQL.Password = "root"
	cfg.RabbitMQ = appconfig.RabbitMQConfig{Mode: "external", LocalURL: brokerURL.String(), ServerURL: brokerURL.String()}
	cfg.CDC.Mode = "external"
	cfg.CDC.Install = false
	cfg.CDC.CanalAddr = "127.0.0.1:11121"
	cfg.CDC.Destination = "soak"
	cfg.CDC.Username = ""
	cfg.CDC.Password = ""
	cfg.CDC.ReaderName = cfg.Node.ID
	cfg.CDC.Filter = database(*mode) + `\.(.*)`
	cfg.CDC.BatchSize = 200
	cfg.Sync.UploadBatchSize = 50
	cfg.Sync.DispatchBatchSize = 50
	cfg.Sync.FlushIntervalMillis = 200
	cfg.Sync.RetryIntervalSeconds = 2
	cfg.LogWeb.Enable = false
	cfg.MCP.Enable = false
	must(appconfig.SaveFile(filepath.Join(*root, "config.yaml"), *cfg))
	rule := rules.SyncRule{ID: "soak-" + *run, Enable: false, DatabaseName: database("edge"), TableName: table("edge"), TargetDatabaseName: database("server"), TargetTableName: table("server"), PrimaryKeys: []string{"id"}, SourceNodeIDs: []string{"soak-" + *run + "-edge"}, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteHard}
	rule.InitialAlignment.Policy = rules.AlignmentManual
	bytes, e := yaml.Marshal(rules.RuleSet{Rules: []rules.SyncRule{rule}})
	must(e)
	must(os.WriteFile(filepath.Join(*root, "rules.yaml"), bytes, 0600))
	var uuid string
	must(db.QueryRow("SELECT @@server_uuid").Scan(&uuid))
	write("prepared.json", map[string]any{"database": database(*mode), "uuid": uuid, "node": cfg.Node.ID})
}
func digest(db *sql.DB, side string) (string, int, error) {
	rows, e := db.Query("SELECT id,CAST(amount AS CHAR),HEX(raw_bytes),note,payload,CAST(stamp AS CHAR),nullable_value FROM " + table(side) + " ORDER BY id")
	if e != nil {
		return "", 0, e
	}
	defer rows.Close()
	h := sha256.New()
	n := 0
	for rows.Next() {
		v := make([]sql.NullString, 7)
		args := make([]any, 7)
		for i := range v {
			args[i] = &v[i]
		}
		if e = rows.Scan(args...); e != nil {
			return "", n, e
		}
		b, _ := json.Marshal(v)
		h.Write(b)
		n++
	}
	return hex.EncodeToString(h.Sum(nil)), n, rows.Err()
}
func verify(dbs []*sql.DB, expected int) bool {
	a, n, e := digest(dbs[0], "server")
	must(e)
	b, m, e := digest(dbs[1], "edge")
	must(e)
	ok := a == b && n == m && n == expected
	write("verification.json", map[string]any{"at": time.Now(), "equal": ok, "expected_rows": expected, "server_rows": n, "edge_rows": m, "server_sha256": a, "edge_sha256": b})
	return ok
}
func workload() {
	dbs := []*sql.DB{open(3306, database("server")), open(*remotePort, database("edge"))}
	defer dbs[0].Close()
	defer dbs[1].Close()
	if *action == "verify" {
		var progress struct {
			Batches int `json:"batches"`
		}
		if b, e := os.ReadFile(filepath.Join(*root, "progress.json")); e == nil {
			must(json.Unmarshal(b, &progress))
		}
		if !verify(dbs, expectedRows(progress.Batches)) {
			panic("data differs")
		}
		return
	}
	start := time.Now()
	end := start.Add(*duration)
	batches := 0
	operations := 0
	for time.Now().Before(end) {
		if _, e := os.Stat(filepath.Join(*root, "stop.requested")); e == nil {
			panic("operator stop requested")
		}
		for i, side := range []string{"server", "edge"} {
			tx, e := dbs[i].Begin()
			must(e)
			for row := 0; row < 10; row++ {
				id := int64(1000 + batches*20 + i*10 + row)
				_, e = tx.Exec("INSERT INTO "+table(side)+" (id,amount,raw_bytes,note,payload,stamp,nullable_value) VALUES (?,12345678901234567890.1234567890,?,?,REPEAT('soak',256),NOW(6),NULL)", id, []byte{0, 128, 255, byte(row)}, fmt.Sprintf("%s-%d", side, batches))
				if e != nil {
					_ = tx.Rollback()
					must(e)
				}
				if batches > 0 {
					prev := id - 20
					_, e = tx.Exec("UPDATE "+table(side)+" SET amount=amount+0.0000000001,note=? WHERE id=?", fmt.Sprintf("updated-%s-%d", side, batches), prev)
					if e != nil {
						_ = tx.Rollback()
						must(e)
					}
				}
				if batches > 1 && row%2 == 0 {
					_, e = tx.Exec("DELETE FROM "+table(side)+" WHERE id=?", id-40)
					if e != nil {
						_ = tx.Rollback()
						must(e)
					}
				}
			}
			must(tx.Commit())
			operations += 10
			if batches > 0 {
				operations += 10
			}
			if batches > 1 {
				operations += 5
			}
		}
		batches++
		write("progress.json", map[string]any{"status": "running", "started_at": start, "planned_end_at": end, "heartbeat_at": time.Now(), "batches": batches, "committed_operations": operations, "pid": os.Getpid()})
		time.Sleep(5 * time.Second)
	}
	write("progress.json", map[string]any{"status": "draining", "started_at": start, "heartbeat_at": time.Now(), "batches": batches, "committed_operations": operations})
	deadline := time.Now().Add(10 * time.Minute)
	ok := false
	for time.Now().Before(deadline) {
		if verify(dbs, expectedRows(batches)) {
			ok = true
			break
		}
		time.Sleep(10 * time.Second)
	}
	write("result.json", map[string]any{"passed": ok, "started_at": start, "finished_at": time.Now(), "batches": batches, "committed_operations": operations})
	if !ok {
		panic("final consistency timeout")
	}
}
func main() {
	flag.Parse()
	if !regexp.MustCompile(`^[a-z0-9]{8,20}$`).MatchString(*run) || *root == "" {
		panic("owned run and root required")
	}
	must(os.MkdirAll(*root, 0700))
	if *action == "reverse-test-roles" {
		if *mode != "server" && *mode != "edge" {
			panic("invalid physical side")
		}
		cfg, e := appconfig.LoadFile(filepath.Join(*root, "config.yaml"))
		must(e)
		if cfg.MySQL.Database != database(*mode) {
			panic("owned database required")
		}
		db := open(3306, database(*mode))
		defer db.Close()
		var jobs int
		must(db.QueryRow("SELECT COUNT(*) FROM sync_alignment_job").Scan(&jobs))
		if jobs != 0 {
			panic("cannot change roles with existing alignment jobs")
		}
		for _, name := range []string{"config.yaml", "rules.yaml"} {
			b, e := os.ReadFile(filepath.Join(*root, name))
			must(e)
			p := filepath.Join(*root, name+".original-roles")
			if _, e = os.Stat(p); !os.IsNotExist(e) {
				panic("original role backup already exists")
			}
			must(os.WriteFile(p, b, 0600))
		}
		cfg.Mode = "edge"
		if *mode == "edge" {
			cfg.Mode = "server"
		}
		must(mysqlconn.RunMigrations(context.Background(), db, filepath.Join(*migrations, cfg.Mode)))
		set, e := rules.LoadFile(filepath.Join(*root, "rules.yaml"))
		must(e)
		r := &set.Rules[0]
		r.DatabaseName = database("server")
		r.TableName = table("server")
		r.TargetDatabaseName = database("edge")
		r.TargetTableName = table("edge")
		r.SourceNodeIDs = []string{"soak-" + *run + "-server"}
		b, e := yaml.Marshal(set)
		must(e)
		must(os.WriteFile(filepath.Join(*root, "rules.yaml"), b, 0600))
		must(appconfig.SaveFile(filepath.Join(*root, "config.yaml"), *cfg))
		return
	}
	if *action == "topology" {
		cfg, e := appconfig.LoadFile(filepath.Join(*root, "config.yaml"))
		must(e)
		u, e := url.Parse(cfg.RabbitMQ.ServerURL)
		must(e)
		if u.Path != "/nbsoak-"+*run {
			panic("owned broker required")
		}
		conn, e := rabbitmq.Dial(cfg.RabbitMQ.ServerURL)
		must(e)
		defer conn.Close()
		must(rabbitmq.InitializeTopology(conn.Channel, rabbitmq.EdgeTopology()))
		set, e := rules.LoadFile(filepath.Join(*root, "rules.yaml"))
		must(e)
		must(rabbitmq.InitializeTopology(conn.Channel, rabbitmq.ServerTopology(set.Rules[0].SourceNodeIDs)))
		return
	}
	if *action == "normalize-schema" {
		if *mode != "server" && *mode != "edge" {
			panic("invalid side")
		}
		db := open(3306, database(*mode))
		defer db.Close()
		_, e := db.Exec("ALTER TABLE " + table(*mode) + " CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_bin")
		must(e)
		return
	}
	if *action == "inspect" {
		cfg, e := appconfig.LoadFile("C:/ProgramData/NodeBridge/config.yaml")
		must(e)
		u, e := url.Parse(cfg.RabbitMQ.ServerURL)
		must(e)
		db := open(3306, "")
		defer db.Close()
		var uuid, format, image string
		must(db.QueryRow("SELECT @@server_uuid,@@binlog_format,@@binlog_row_image").Scan(&uuid, &format, &image))
		b, e := json.Marshal(map[string]any{"broker_host": u.Host, "broker_user": u.User.Username(), "mysql_uuid": uuid, "binlog_format": format, "row_image": image})
		must(e)
		fmt.Println(string(b))
		return
	}
	if *action == "prepare" {
		prepare()
	} else {
		workload()
	}
}
