//go:build windows

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/uiapi"
	"github.com/go-sql-driver/mysql"
	"github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

type ownedMCPClient struct {
	t       *testing.T
	input   io.WriteCloser
	output  *bufio.Scanner
	seq     int
	process *exec.Cmd
}

func startOwnedMCP(t *testing.T, ctx context.Context, executable, directory string) *ownedMCPClient {
	t.Helper()
	cmd := exec.CommandContext(ctx, executable, "mcp-stdio", "-config", filepath.Join(directory, "config.yaml"), "-rules", filepath.Join(directory, "rules.yaml"))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(directory, "mcp-stderr.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	client := &ownedMCPClient{t: t, input: input, output: bufio.NewScanner(output), process: cmd}
	client.output.Buffer(make([]byte, 4096), 16<<20)
	t.Cleanup(func() {
		_ = input.Close()
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
		_ = log.Close()
	})
	_, err = client.rpc("initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "owned-multi-fixture", "version": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func (c *ownedMCPClient) rpc(method string, params any) (json.RawMessage, error) {
	c.seq++
	request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.seq, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if _, err := c.input.Write(append(request, '\n')); err != nil {
		return nil, err
	}
	if !c.output.Scan() {
		return nil, fmt.Errorf("MCP output ended: %w", c.output.Err())
	}
	var reply struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(c.output.Bytes(), &reply); err != nil {
		return nil, err
	}
	if reply.ID != c.seq || len(reply.Error) > 0 && string(reply.Error) != "null" {
		return nil, fmt.Errorf("MCP protocol error: %s", c.output.Bytes())
	}
	return reply.Result, nil
}

func (c *ownedMCPClient) tool(name string, arguments any) (json.RawMessage, error) {
	data, err := c.rpc("tools/call", map[string]any{"name": name, "arguments": arguments})
	if err != nil {
		return nil, err
	}
	var result struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &result); err != nil || len(result.Content) != 1 {
		return nil, fmt.Errorf("invalid MCP tool result %s", data)
	}
	if result.IsError {
		return nil, errors.New(result.Content[0].Text)
	}
	return json.RawMessage(result.Content[0].Text), nil
}

func TestOwnedMultiNodePipeline(t *testing.T) {
	if os.Getenv("NODEBRIDGE_OWNED_MULTI_FIXTURE") != "1" {
		t.Skip("run scripts/test-multi-node-sync.ps1")
	}
	root, err := filepath.Abs(os.Getenv("NODEBRIDGE_MULTI_FIXTURE_ROOT"))
	allowed, _ := filepath.Abs("../../.cache/multi-node-sync")
	if err != nil || !strings.HasPrefix(strings.ToLower(root), strings.ToLower(allowed)+string(os.PathSeparator)) {
		t.Fatal("owned multi-node fixture path required")
	}
	var endpoints []struct {
		Port     int    `json:"mysql_port"`
		Canal    string `json:"canal_addr"`
		LocalURL string `json:"local_url"`
	}
	if json.Unmarshal([]byte(os.Getenv("NODEBRIDGE_MULTI_ENDPOINTS")), &endpoints) != nil || len(endpoints) != 3 {
		t.Fatal("three independent endpoints required")
	}
	soakSeconds, _ := strconv.Atoi(os.Getenv("NODEBRIDGE_MULTI_SOAK_SECONDS"))
	if soakSeconds < 0 || soakSeconds > 25200 {
		t.Fatal("invalid soak duration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(soakSeconds)*time.Second+12*time.Minute)
	t.Cleanup(cancel)
	nodes := []string{"owned-multi-edge1", "owned-multi-server", "owned-multi-edge2"}
	modes := []string{"edge", "server", "edge"}
	databases := []string{"nb_multi_edge1", "nb_multi_server", "nb_multi_edge2"}
	tables := []string{"edge_rows", "central_rows", "branch_rows"}
	keys := []string{"edge_id", "central_id", "branch_id"}
	values := []string{"edge_value", "central_value", "branch_value"}
	sharedRule := os.Getenv("NODEBRIDGE_MULTI_SHARED_RULE") == "1"
	if sharedRule {
		for i := range nodes {
			databases[i], tables[i], keys[i], values[i] = "nb_multi_shared", "shared_rows", "row_id", "row_value"
		}
	}
	dbs := make([]*sql.DB, 3)
	commandTime := time.Now().Unix()
	execute := func(index int, query string, args ...any) {
		t.Helper()
		commandTime++
		connection, err := dbs[index].Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		defer func() { _ = connection.Raw(func(any) error { return driver.ErrBadConn }) }()
		if _, err := connection.ExecContext(ctx, "SET TIMESTAMP=?", commandTime); err != nil {
			t.Fatal(err)
		}
		if _, err := connection.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	uuids := map[string]bool{}
	for i, endpoint := range endpoints {
		canalHost, _, err := net.SplitHostPort(endpoint.Canal)
		broker, brokerErr := url.Parse(endpoint.LocalURL)
		if err != nil || canalHost != "127.0.0.1" || brokerErr != nil || broker.Hostname() != "127.0.0.1" || endpoint.Port < 1024 {
			t.Fatal("owned loopback endpoints required")
		}
		cfg := mysql.NewConfig()
		cfg.User, cfg.Passwd, cfg.Net, cfg.Addr = "root", "owned_multi_only", "tcp", fmt.Sprintf("127.0.0.1:%d", endpoint.Port)
		cfg.ParseTime = true
		cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 3*time.Second, 15*time.Second, 15*time.Second
		admin, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+databases[i]+"`"); err != nil {
			t.Fatal(err)
		}
		_ = admin.Close()
		cfg.DBName = databases[i]
		db, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		dbs[i] = db
		t.Cleanup(func() { _ = db.Close() })
		var uuid string
		if err := db.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&uuid); err != nil || uuids[uuid] {
			t.Fatal("MySQL instances are not independent", uuid, err)
		}
		uuids[uuid] = true
		if err := mysqlconn.RunMigrations(ctx, db, "../../migrations/"+modes[i]); err != nil {
			t.Fatal(err)
		}
		execute(i, "CREATE TABLE "+tables[i]+" ("+keys[i]+" BIGINT UNSIGNED PRIMARY KEY,"+values[i]+" DECIMAL(30,10) NOT NULL,payload LONGBLOB,note VARCHAR(64)) ENGINE=InnoDB")
		connection, err := amqp091.Dial(endpoint.LocalURL)
		if err != nil {
			t.Fatal(err)
		}
		channel, err := connection.Channel()
		if err != nil {
			t.Fatal(err)
		}
		topology := rabbitmq.EdgeTopology()
		if i == 1 {
			topology = rabbitmq.ServerTopology([]string{nodes[0], nodes[2]})
		}
		if err := rabbitmq.InitializeTopology(channel, topology); err != nil {
			t.Fatal(err)
		}
		_ = channel.Close()
		_ = connection.Close()
	}
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		snapshot := map[string]any{}
		for i, db := range dbs {
			queries := map[string]string{
				"business":  "SELECT " + keys[i] + ",note FROM " + tables[i] + " ORDER BY " + keys[i] + " LIMIT 30",
				"versions":  "SELECT * FROM sync_row_version LIMIT 30",
				"conflicts": "SELECT * FROM sync_conflict_event LIMIT 30",
				"jobs":      "SELECT job_id,phase,endpoint_role,capture_boundary FROM sync_alignment_job",
				"groups":    "SELECT intent_id,phase FROM sync_alignment_topology",
			}
			node := map[string]any{}
			for label, query := range queries {
				readCtx, done := context.WithTimeout(context.Background(), 2*time.Second)
				rows, err := db.QueryContext(readCtx, query)
				if err != nil {
					node[label] = err.Error()
					done()
					continue
				}
				columns, _ := rows.Columns()
				var records []map[string]string
				for rows.Next() {
					values, scan := make([][]byte, len(columns)), make([]any, len(columns))
					for j := range scan {
						scan[j] = &values[j]
					}
					if rows.Scan(scan...) != nil {
						break
					}
					record := map[string]string{}
					for j, column := range columns {
						record[column] = string(values[j])
					}
					records = append(records, record)
				}
				_ = rows.Close()
				done()
				node[label] = records
			}
			snapshot[nodes[i]] = node
		}
		data, _ := json.MarshalIndent(snapshot, "", "  ")
		_ = os.WriteFile(filepath.Join(root, "failure-state.json"), data, 0600)
	})
	large := os.Getenv("NODEBRIDGE_MULTI_LARGE") == "1"
	source := map[string]int{"edge1": 0, "server": 1, "edge2": 2, "empty": -1}[os.Getenv("NODEBRIDGE_MULTI_SOURCE")]
	seedRows := 0
	if source >= 0 {
		seedRows = 1
		payload := []byte{0, 128, 255}
		if large {
			seedRows = 400
			payload = make([]byte, 512*1024)
			for i := range payload {
				payload[i] = byte(i % 251)
			}
		}
		transaction, err := dbs[source].BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		statement, err := transaction.PrepareContext(ctx, "INSERT INTO "+tables[source]+" ("+keys[source]+","+values[source]+",payload,note) VALUES (?, '12345678901234567890.1234567890',?,'snapshot')")
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < seedRows; i++ {
			if _, err := statement.ExecContext(ctx, 10000+i, payload); err != nil {
				_ = transaction.Rollback()
				t.Fatal(err)
			}
		}
		_ = statement.Close()
		if err := transaction.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	memberRules := map[int]rules.SyncRule{}
	for _, i := range []int{0, 2} {
		memberRules[i] = rules.SyncRule{ID: "pair-" + nodes[i], DatabaseName: databases[i], TableName: tables[i], TargetDatabaseName: databases[1], TargetTableName: tables[1], PrimaryKeys: []string{keys[i]}, TargetPrimaryKeys: []string{keys[1]}, SourceNodeIDs: []string{nodes[i]}, Direction: rules.DirectionBidirectional, ConflictPolicy: rules.ConflictLastWriteWin, DeleteMode: rules.DeleteHard, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}, ColumnMappings: []rules.ColumnMapping{{SourceColumn: values[i], TargetColumn: values[1]}}}
	}
	if sharedRule {
		shared := memberRules[0]
		shared.ID, shared.SourceNodeIDs = "shared-multi-rule", []string{nodes[0], nodes[2]}
		memberRules[0], memberRules[2] = shared, shared
	}
	clients := make([]*ownedMCPClient, 3)
	batchSize := 16
	if raw := os.Getenv("NODEBRIDGE_MULTI_BATCH_SIZE"); raw != "" {
		batchSize, err = strconv.Atoi(raw)
		if err != nil || batchSize < 1 || batchSize > 16 {
			t.Fatal("invalid fixture batch size")
		}
	}
	directories := make([]string, 3)
	for i, endpoint := range endpoints {
		dir := filepath.Join(root, nodes[i])
		directories[i] = dir
		cfg := appconfig.Config{Mode: modes[i], Node: appconfig.NodeConfig{ID: nodes[i]}, MySQL: appconfig.MySQLConfig{Host: "127.0.0.1", Port: endpoint.Port, Username: "root", Password: "owned_multi_only", Database: databases[i]}, RabbitMQ: appconfig.RabbitMQConfig{Mode: "external", LocalURL: endpoint.LocalURL, ServerURL: endpoints[1].LocalURL}, CDC: appconfig.CDCConfig{Type: "canal", Mode: "external", CanalAddr: endpoint.Canal, Destination: "example", ReaderName: "owned-multi-reader", Filter: databases[i] + `\.` + tables[i], BatchSize: 16}, Sync: appconfig.SyncConfig{UploadBatchSize: 16, DispatchBatchSize: 16, FlushIntervalMillis: 50, RetryIntervalSeconds: 1}, MCP: appconfig.MCPServerConfig{Enable: true}}
		cfg.CDC.BatchSize, cfg.Sync.UploadBatchSize, cfg.Sync.DispatchBatchSize = batchSize, batchSize, batchSize
		if err := appconfig.SaveFile(filepath.Join(dir, "config.yaml"), cfg); err != nil {
			t.Fatal(err)
		}
		set := rules.RuleSet{Rules: []rules.SyncRule{memberRules[i]}}
		if i == 1 {
			set.Rules = []rules.SyncRule{memberRules[0], memberRules[2]}
			if sharedRule {
				set.Rules = set.Rules[:1]
			}
		}
		data, _ := yaml.Marshal(set)
		if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), data, 0600); err != nil {
			t.Fatal(err)
		}
		clients[i] = startOwnedMCP(t, ctx, filepath.Join(root, "SyncAgent.exe"), dir)
		catalog, err := clients[i].rpc("tools/list", map[string]any{})
		if err != nil || !strings.Contains(string(catalog), "nodebridge_start_initial_alignment") {
			t.Fatal("ordinary MCP lacks initial alignment", err, string(catalog))
		}
	}
	requests := []uiapi.InitialAlignmentRequest{
		{RuleID: memberRules[0].ID, PeerNodeID: nodes[1], Confirm: true},
		{RuleID: memberRules[0].ID, PeerNodeID: nodes[2] + "," + nodes[0], Confirm: true},
		{RuleID: memberRules[2].ID, PeerNodeID: nodes[1], Confirm: true},
	}
	status := func(i int) uiapi.InitialAlignmentStatus {
		t.Helper()
		data, err := clients[i].tool("nodebridge_initial_alignment_status", map[string]any{})
		var result uiapi.InitialAlignmentStatus
		if err != nil || json.Unmarshal(data, &result) != nil {
			t.Fatal("MCP status", err, string(data))
		}
		return result
	}
	if _, err := clients[0].tool("nodebridge_start_initial_alignment", requests[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := clients[0].tool("nodebridge_interrupt_initial_alignment", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(8 * time.Second); status(0).Running; {
		if time.Now().After(deadline) {
			t.Fatal("MCP interruption did not finish")
		}
		time.Sleep(50 * time.Millisecond)
	}
	interCopy := os.Getenv("NODEBRIDGE_MULTI_INTER_COPY") == "1"
	interCopyDone := false
	partialRetry := os.Getenv("NODEBRIDGE_MULTI_PARTIAL_RETRY") == "1"
	partialProof := ""
	if interCopy {
		if source != 1 {
			t.Fatal("inter-copy fixture requires a central initial source")
		}
	}
	if interCopy || partialRetry {
		execute(1, "CREATE TRIGGER pause_multi_cutover BEFORE UPDATE ON sync_alignment_job FOR EACH ROW BEGIN IF NEW.scope_hash <> OLD.scope_hash THEN DO SLEEP(3); END IF; END")
	}
	alignAll := func() {
		t.Helper()
		for _, i := range []int{0, 2, 1} {
			if _, err := clients[i].tool("nodebridge_start_initial_alignment", requests[i]); err != nil {
				t.Fatal(nodes[i], err)
			}
		}
		deadline := time.Now().Add(6 * time.Minute)
		for {
			if partialRetry && partialProof == "" {
				var active int
				if err := dbs[1].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_alignment_cutover WHERE phase='ACTIVE'").Scan(&active); err != nil {
					t.Fatal(err)
				}
				if active == 1 {
					if err := dbs[1].QueryRowContext(ctx, "SELECT proof_id FROM sync_alignment_cutover WHERE phase='ACTIVE' LIMIT 1").Scan(&partialProof); err != nil {
						t.Fatal(err)
					}
					for _, client := range clients {
						if _, err := client.tool("nodebridge_interrupt_initial_alignment", map[string]any{}); err != nil {
							t.Fatal(err)
						}
					}
					stopDeadline := time.Now().Add(10 * time.Second)
					for {
						running := false
						for i := range clients {
							running = status(i).Running || running
						}
						if !running {
							return
						}
						if time.Now().After(stopDeadline) {
							t.Fatal("partial group interruption did not finish")
						}
						time.Sleep(50 * time.Millisecond)
					}
				}
			}
			if interCopy && !interCopyDone {
				var active int
				if err := dbs[1].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_alignment_cutover WHERE phase='ACTIVE'").Scan(&active); err != nil {
					t.Fatal(err)
				}
				if active == 1 {
					execute(0, "INSERT INTO "+tables[0]+" ("+keys[0]+","+values[0]+",note) VALUES (800,1,'inter-copy-old'),(801,1,'inter-copy-delete-old')")
					execute(1, "INSERT INTO "+tables[1]+" ("+keys[1]+","+values[1]+",note) VALUES (800,2,'inter-copy-winner'),(801,2,'inter-copy-delete-new')")
					execute(1, "DELETE FROM "+tables[1]+" WHERE "+keys[1]+"=801")
					interCopyDone = true
				}
			}
			complete := true
			states := make([]uiapi.InitialAlignmentStatus, 3)
			for i := range clients {
				states[i] = status(i)
				if states[i].Stage != "completed" || states[i].Running {
					complete = false
				}
			}
			for i, state := range states {
				if !state.Running && state.Stage != "completed" {
					t.Fatalf("%s group alignment failed: %+v; all=%+v", nodes[i], state, states)
				}
			}
			if complete {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("group alignment timed out: %+v", states)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	alignAll()
	if partialRetry {
		if partialProof == "" {
			t.Fatal("partial copy interruption was not exercised")
		}
		for i, db := range dbs {
			var pending int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_alignment_topology WHERE phase<>'ACTIVE'").Scan(&pending); err != nil || pending != 1 {
				t.Fatal("partial group incorrectly active", i, pending, err)
			}
		}
		execute(1, "DROP TRIGGER pause_multi_cutover")
		alignAll()
		var retained int
		if err := dbs[1].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_alignment_cutover WHERE proof_id=? AND phase='ACTIVE'", partialProof).Scan(&retained); err != nil || retained != 1 {
			t.Fatal("committed first copy was replaced during retry", retained, err)
		}
		t.Log("PASS: MCP interruption after first durable copy kept the group pending; retry retained the original committed proof")
	}
	if interCopy {
		if !interCopyDone {
			t.Fatal("inter-copy write window was not exercised")
		}
		execute(1, "DROP TRIGGER pause_multi_cutover")
	}
	baselineDigest := ""
	for i, db := range dbs {
		rows, err := db.QueryContext(ctx, "SELECT "+keys[i]+",SHA2(payload,256),OCTET_LENGTH(payload),"+values[i]+",note FROM "+tables[i]+" WHERE "+keys[i]+">=10000 ORDER BY "+keys[i])
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.New()
		count, rawBytes := 0, 0
		for rows.Next() {
			var key, n int
			var digest, decimal, note string
			if err := rows.Scan(&key, &digest, &n, &decimal, &note); err != nil {
				t.Fatal(err)
			}
			_, _ = fmt.Fprintf(h, "%d|%s|%d|%s|%s\n", key, digest, n, decimal, note)
			count++
			rawBytes += n
		}
		rowErr := rows.Err()
		_ = rows.Close()
		if rowErr != nil || count != seedRows || large && rawBytes != 200*1024*1024 {
			t.Fatal("snapshot size/count mismatch", nodes[i], count, rawBytes, rowErr)
		}
		digest := hex.EncodeToString(h.Sum(nil))
		if i == 0 {
			baselineDigest = digest
		} else if digest != baselineDigest {
			t.Fatal("baseline checksum differs", nodes[i])
		}
		var active int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_alignment_topology WHERE phase='ACTIVE'").Scan(&active); err != nil || active != 1 {
			t.Fatal("group not durably active", nodes[i], active, err)
		}
		observations, err := readPairManifest(filepath.Join(directories[i], "rules.yaml.pairs.json"))
		if err != nil || len(observations) != 2 {
			t.Fatal("member did not receive complete observed topology", nodes[i], err)
		}
	}
	t.Logf("PASS: ordinary MCP start/status/interrupt, three independent schemas, full topology activation, %d baseline rows per node; 200 MiB mode=%t; payload digest=%s", seedRows, large, baselineDigest)
	for i := range dbs {
		execute(i, "INSERT INTO "+tables[i]+" ("+keys[i]+","+values[i]+",payload,note) VALUES (?,3,X'0080FF',?)", 100+i, "before-start-"+nodes[i])
	}
	alignAll()
	for i := range clients {
		for _, enabled := range []bool{false, true} {
			set, revision, err := rules.LoadFileWithRevision(filepath.Join(directories[i], "rules.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			for j := range set.Rules {
				set.Rules[j].Enable = enabled
			}
			data, err := clients[i].tool("nodebridge_save_sync_rules", map[string]any{"rules": set.Rules, "expected_revision": revision})
			if err != nil || !strings.Contains(string(data), `"saved"`) {
				t.Fatal("multi-member rule save", nodes[i], enabled, err, string(data))
			}
		}
	}
	start := func(i int) func() {
		t.Helper()
		dir := directories[i]
		stopFile := filepath.Join(dir, "stop.request")
		log, err := os.Create(filepath.Join(dir, fmt.Sprintf("agent-%d.log", time.Now().UnixNano())))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, filepath.Join(root, "SyncAgent.exe"), "run", "-config", filepath.Join(dir, "config.yaml"), "-rules", filepath.Join(dir, "rules.yaml"), "-stop-file", stopFile, "-edges", nodes[0]+","+nodes[2])
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		var once sync.Once
		stop := func() {
			once.Do(func() {
				_ = os.WriteFile(stopFile, []byte("owned multi-node stop"), 0600)
				select {
				case err := <-done:
					if err != nil {
						t.Errorf("%s Agent failed: %v", nodes[i], err)
					}
				case <-time.After(8 * time.Second):
					_ = cmd.Process.Kill()
					<-done
					t.Errorf("%s Agent required forced cleanup", nodes[i])
				}
				_ = log.Close()
			})
		}
		t.Cleanup(stop)
		return stop
	}
	stops := []func(){start(0), start(1), start(2)}
	read := func(i, key int, note string) bool {
		var count int
		return dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tables[i]+" WHERE "+keys[i]+"=? AND note=?", key, note).Scan(&count) == nil && count == 1
	}
	absent := func(i, key int) bool {
		var count int
		return dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tables[i]+" WHERE "+keys[i]+"=?", key).Scan(&count) == nil && count == 0
	}
	wait := func(label string, predicate func() bool) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if predicate() {
				return
			}
			time.Sleep(80 * time.Millisecond)
		}
		t.Fatal("multi-node timeout: " + label)
	}
	all := func(key int, note string) bool { return read(0, key, note) && read(1, key, note) && read(2, key, note) }
	if interCopy {
		wait("inter-copy source version survives later snapshot", func() bool { return all(800, "inter-copy-winner") })
		wait("inter-copy tombstone survives later snapshot", func() bool { return absent(0, 801) && absent(1, 801) && absent(2, 801) })
		t.Log("PASS: writes between sequential snapshots preserve winner versions and deletion tombstones on every member")
	}
	for i := range dbs {
		wait("pre-start write survives committed retry "+nodes[i], func() bool { return all(100+i, "before-start-"+nodes[i]) })
	}
	for i := range dbs {
		key, note := 200+i, "insert-"+nodes[i]
		execute(i, "INSERT INTO "+tables[i]+" ("+keys[i]+","+values[i]+",payload,note) VALUES (?,'12345678901234567890.1234567890',X'0080FF',?)", key, note)
		wait("insert fanout "+nodes[i], func() bool { return all(key, note) })
		for j := range dbs {
			var decimal, raw string
			if err := dbs[j].QueryRowContext(ctx, "SELECT "+values[j]+",HEX(payload) FROM "+tables[j]+" WHERE "+keys[j]+"=?", key).Scan(&decimal, &raw); err != nil || decimal != "12345678901234567890.1234567890" || raw != "0080FF" {
				t.Fatal("three-way precision or binary mapping lost", j, decimal, raw, err)
			}
		}
		writer := (i + 1) % 3
		execute(writer, "UPDATE "+tables[writer]+" SET note=? WHERE "+keys[writer]+"=?", "update-"+nodes[writer], key)
		wait("retained-marker update fanout", func() bool { return all(key, "update-"+nodes[writer]) })
		writer = (i + 2) % 3
		execute(writer, "DELETE FROM "+tables[writer]+" WHERE "+keys[writer]+"=?", key)
		wait("delete fanout", func() bool { return absent(0, key) && absent(1, key) && absent(2, key) })
	}
	for round := 0; round < 2; round++ {
		key := 400 + round
		execute(1, "INSERT INTO "+tables[1]+" ("+keys[1]+","+values[1]+",note) VALUES (?,1,'conflict-base')", key)
		wait("conflict baseline", func() bool { return all(key, "conflict-base") })
		stops[2]()
		execute(0, "UPDATE "+tables[0]+" SET note='earlier-edge1' WHERE "+keys[0]+"=?", key)
		wait("earlier reaches server", func() bool { return read(1, key, "earlier-edge1") })
		execute(2, "UPDATE "+tables[2]+" SET note='later-offline-edge2' WHERE "+keys[2]+"=?", key)
		stops[2] = start(2)
		wait("offline edge2 wins across whole star", func() bool { return all(key, "later-offline-edge2") })
		stops[0]()
		execute(1, "UPDATE "+tables[1]+" SET note='central-before-delete' WHERE "+keys[1]+"=?", key)
		wait("central update reaches online edge2", func() bool { return read(2, key, "central-before-delete") })
		execute(2, "DELETE FROM "+tables[2]+" WHERE "+keys[2]+"=?", key)
		wait("delete reaches center", func() bool { return absent(1, key) })
		stops[0] = start(0)
		wait("offline edge1 delete convergence", func() bool { return absent(0, key) && absent(1, key) && absent(2, key) })
	}
	execute(2, "CREATE TRIGGER reject_multi_receipt BEFORE INSERT ON sync_apply_log FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned multi receipt failure'")
	execute(0, "INSERT INTO "+tables[0]+" ("+keys[0]+","+values[0]+",note) VALUES (900,1,'retry-fanout')")
	wait("actual receipt failure observed", func() bool {
		data, _ := os.ReadFile(filepath.Join(directories[2], "logs", "sync-runtime.jsonl"))
		return strings.Contains(string(data), "owned multi receipt failure")
	})
	if !absent(2, 900) {
		t.Fatal("business row escaped failed receipt transaction")
	}
	execute(2, "DROP TRIGGER reject_multi_receipt")
	wait("receipt retry fanout", func() bool { return all(900, "retry-fanout") })
	for i := range dbs {
		wait("repair queue drained "+nodes[i], func() bool {
			var count int
			return dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_conflict_state WHERE repair_required=1").Scan(&count) == nil && count == 0
		})
	}
	if soakSeconds > 0 {
		runOwnedMultiSoak(t, ctx, root, time.Duration(soakSeconds)*time.Second, dbs, tables, keys, values, execute, start, stops)
	}
	if os.Getenv("NODEBRIDGE_MULTI_RECONNECT") == "1" {
		runOwnedRabbitReconnect(t, ctx, root, dbs, tables, keys, values, execute, all, absent)
	}
	counts := make([]int, 3)
	for i := range dbs {
		if err := dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log").Scan(&counts[i]); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(1500 * time.Millisecond)
	for i := range dbs {
		var count int
		if err := dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_apply_log").Scan(&count); err != nil || count != counts[i] {
			t.Fatal("unexpected continuing replay receipts", nodes[i], count, counts[i], err)
		}
		stops[i]()
	}
	for i := range dbs {
		execute(i, "UPDATE sync_alignment_topology SET phase='READY'")
		dir := directories[i]
		blocked, done := context.WithTimeout(ctx, 8*time.Second)
		cmd := exec.CommandContext(blocked, filepath.Join(root, "SyncAgent.exe"), "run", "-config", filepath.Join(dir, "config.yaml"), "-rules", filepath.Join(dir, "rules.yaml"))
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		output, err := cmd.CombinedOutput()
		done()
		if err == nil || !strings.Contains(string(output), "alignment_topology_pending") {
			t.Fatal("incomplete topology did not block Agent", nodes[i], err, string(output))
		}
	}
	t.Log("PASS: three production Agents, distinct local queues, all-origin lossless CRUD/relay, offline newer update and delete, restart, receipt rollback/retry, stable receipt counts, repair drain and incomplete-topology startup gate. Short functional test, not a soak test.")
}
