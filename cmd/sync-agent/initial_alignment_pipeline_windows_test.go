//go:build windows

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/rabbitmq/amqp091-go"
)

func prepareOwnedInitialAlignment(t *testing.T, ctx context.Context, root string, dbs [2]*sql.DB, ch *amqp091.Channel, rule rules.SyncRule) {
	t.Helper()
	side := os.Getenv("NODEBRIDGE_CDC_ALIGNMENT_SOURCE")
	tables, keys, values := []string{"source_rows", "target_rows"}, []string{"edge_id", "server_id"}, []string{"edge_value", "server_value"}
	modes, nodes, databases := []string{"edge", "server"}, []string{"owned-bidi-edge", "owned-bidi-server"}, []string{"nb_cdc_source", "nb_cdc_target"}
	if side != "empty" {
		i := 0
		if side == "server" {
			i = 1
		}
		if _, err := dbs[i].ExecContext(ctx, "INSERT INTO "+tables[i]+" ("+keys[i]+","+values[i]+",raw_bytes,note) VALUES (7,'12345678901234567890.1234567890',X'0080FF','snapshot')"); err != nil {
			t.Fatal(err)
		}
	}
	publisher, err := rabbitmq.NewPublisher(ch)
	if err != nil {
		t.Fatal(err)
	}
	for i := range nodes {
		old := event.SyncEvent{EventID: "legacy-before-alignment-" + modes[i], EventType: event.TypeInsert, OriginNodeID: nodes[i], SourceNodeID: nodes[i], DatabaseName: databases[i], TableName: tables[i], PrimaryKey: map[string]any{keys[i]: "7"}, After: map[string]any{keys[i]: "7", values[i]: "0", "note": "old-must-not-win"}, BinlogFile: "mysql-bin.000001", BinlogPos: 4, EventTime: time.Now(), CreatedAt: time.Now()}
		body, err := rabbitmq.EncodeJSON(old)
		if err != nil {
			t.Fatal(err)
		}
		queue := "edge.upload.cdc.q"
		if i == 1 {
			queue = nodes[0] + ".downlink.q"
		}
		if err := publisher.Publish(ctx, rabbitmq.PublishRequest{RoutingKey: queue, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	type aligned struct {
		output []byte
		err    error
	}
	results := []chan aligned{make(chan aligned, 1), make(chan aligned, 1)}
	for i := range modes {
		dir := filepath.Join(root, modes[i])
		cmd := exec.CommandContext(ctx, filepath.Join(root, "SyncAgent.exe"), "initial-alignment", "-config", filepath.Join(dir, "config.yaml"), "-rules", filepath.Join(dir, "rules.yaml"), "-rule", rule.ID, "-peer", nodes[1-i], "-confirm")
		cmd.Dir, _ = filepath.Abs("../..")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		go func(index int) { output, err := cmd.CombinedOutput(); results[index] <- aligned{output, err} }(i)
	}
	var outcomes [2]aligned
	for i := range modes {
		outcomes[i] = <-results[i]
		_ = os.WriteFile(filepath.Join(root, modes[i], "initial-alignment.log"), outcomes[i].output, 0600)
	}
	for i, result := range outcomes {
		if result.err != nil {
			t.Fatalf("%s first alignment failed: %v %s", modes[i], result.err, result.output)
		}
		set, err := rules.LoadFile(filepath.Join(root, modes[i], "rules.yaml"))
		if err != nil || !set.Rules[0].Enable {
			t.Fatal("aligned rule not enabled", err)
		}
		if _, err := readPairManifest(filepath.Join(root, modes[i], "rules.yaml.pairs.json")); err != nil {
			t.Fatal("pair manifest not generated", err)
		}
		verifyOwnedAlignedRuleSave(t, ctx, root, modes[i])
		var count int
		wanted := 1
		if side == "empty" {
			wanted = 0
		}
		if err := dbs[i].QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tables[i]+" WHERE "+keys[i]+"=7").Scan(&count); err != nil || count != wanted {
			t.Fatal("wrong baseline", count, err)
		}
	}
	if _, err := dbs[0].ExecContext(ctx, "INSERT INTO source_rows (edge_id,edge_value,note) VALUES (8,8,'before-agent-start')"); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS: actual initial-alignment CLI generated pair manifests and enabled rules without remote MySQL credentials or manual manifests")
}

func verifyOwnedAlignedRuleSave(t *testing.T, ctx context.Context, root, mode string) {
	t.Helper()
	dir := filepath.Join(root, mode)
	for _, enabled := range []bool{true, false, true} {
		set, revision, err := rules.LoadFileWithRevision(filepath.Join(dir, "rules.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		set.Rules[0].Enable = enabled
		request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "nodebridge_save_sync_rules", "arguments": map[string]any{"rules": set.Rules, "expected_revision": revision}}}
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, filepath.Join(root, "SyncAgent.exe"), "mcp-stdio", "-config", filepath.Join(dir, "config.yaml"), "-rules", filepath.Join(dir, "rules.yaml"), "-lab-full-access")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Stdin = bytes.NewReader(append(data, '\n'))
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s aligned rule save: %v %s", mode, err, output)
		}
		var response struct {
			Error  any `json:"error"`
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if json.Unmarshal(output, &response) != nil || response.Error != nil || response.Result.IsError || len(response.Result.Content) != 1 {
			t.Fatalf("%s rule-save response %s", mode, output)
		}
		var result struct {
			OK     bool   `json:"ok"`
			Status string `json:"status"`
		}
		if json.Unmarshal([]byte(response.Result.Content[0].Text), &result) != nil || !result.OK || result.Status != "saved" {
			t.Fatalf("%s aligned rule save rejected %s", mode, output)
		}
		persisted, err := rules.LoadFile(filepath.Join(dir, "rules.yaml"))
		if err != nil || persisted.Rules[0].Enable != enabled {
			t.Fatal("aligned enable not persisted", err)
		}
	}
}
