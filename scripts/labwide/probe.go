package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

// The probe owns its queue and tables; no shared consumer is paused or purged.
func (l *lab) probe() error {
	agent := os.Getenv("NODEBRIDGE_WIDE_AGENT")
	config := os.Getenv("NODEBRIDGE_WIDE_AGENT_CONFIG")
	url := os.Getenv("NODEBRIDGE_WIDE_AMQP")
	if agent == "" || config == "" || url == "" {
		return fmt.Errorf("probe requires NODEBRIDGE_WIDE_AGENT, NODEBRIDGE_WIDE_AGENT_CONFIG and NODEBRIDGE_WIDE_AMQP")
	}
	appendTable := table{l.c.Prefix + "_probe_append", nil}
	stateTable := table{l.c.Prefix + "_probe_state", nil}
	guardTable := table{l.c.Prefix + "_probe_guard", nil}
	set := rules.RuleSet{}
	for _, t := range []table{appendTable, stateTable, guardTable} {
		if _, err := l.db[1].Exec(t.ddl(l.s)); err != nil {
			return err
		}
		mode := rules.SyncModeAppendOnly
		if t.Name == stateTable.Name {
			mode = rules.SyncModeOrderedCRUD
		}
		set.Rules = append(set.Rules, rules.SyncRule{ID: t.Name, DatabaseName: "scada_edge", TableName: t.Name, TargetDatabaseName: "scada_center", TargetTableName: t.Name, SourceNodeIDs: []string{"edge-001"}, Direction: rules.DirectionEdgeToServer, DispatchTarget: rules.DispatchNone, SyncMode: mode, ConflictPolicy: rules.ConflictNone, Enable: true, PrimaryKeys: []string{"id"}})
		if t.Name == guardTable.Name {
			set.Rules[len(set.Rules)-1].ExcludeColumns = []string{"i01"}
		}
	}
	b, err := yaml.Marshal(set)
	if err != nil {
		return err
	}
	rulesPath := filepath.Join(l.root, "probe-rules.yaml")
	if err = os.WriteFile(rulesPath, b, 0600); err != nil {
		return err
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	q, err := ch.QueueDeclare(l.c.Prefix+".probe", false, false, false, false, nil)
	if err != nil {
		return err
	}
	defer ch.QueueDelete(q.Name, false, false, false) // Only this uniquely owned test queue.
	if err = ch.Confirm(false); err != nil {
		return err
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 16))
	evidence := map[string]any{"at": time.Now(), "queue": q.Name, "columns": 50, "batches": []string{}, "passed": false}
	outputs := []string{}
	defer func() {
		evidence["batches"] = outputs
		_ = atomicJSON(filepath.Join(l.root, "idempotency.json"), evidence)
	}()
	mk := func(t table, id string, rev int64, operation string) event.SyncEvent {
		row := makeRow(l.c, l.sides[0], 1, rev)
		after := map[string]any{}
		for i, col := range l.s.Columns {
			after[col.Name] = row[i]
		}
		return event.SyncEvent{EventID: l.c.RunID + "-" + id, EventType: operation, OriginNodeID: "edge-001", SourceNodeID: "edge-001", DatabaseName: "scada_edge", TableName: t.Name, PrimaryKey: map[string]any{"id": int64(1)}, After: after, SchemaVersion: 1, SyncVersion: rev, CreatedAt: time.Now(), EventTime: time.Now(), TraceID: l.c.RunID + "-" + id}
	}
	consume := func(events []event.SyncEvent, expectFailure bool) error {
		for _, evt := range events {
			body, e := json.Marshal(evt)
			if e != nil {
				return e
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			e = ch.PublishWithContext(ctx, "", q.Name, true, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body})
			cancel()
			if e != nil {
				return e
			}
			select {
			case c := <-confirms:
				if !c.Ack {
					return fmt.Errorf("probe publish NACK")
				}
			case <-time.After(10 * time.Second):
				return fmt.Errorf("probe confirm timeout")
			}
		}
		pending, e := ch.QueueDeclarePassive(q.Name, false, false, false, false, nil)
		if e != nil {
			return e
		}
		if pending.Consumers != 0 || pending.Messages != len(events) {
			return fmt.Errorf("cannot establish same-batch isolation: %+v", pending)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, agent, "consume-batch-once", "-config", config, "-rules", rulesPath, "-amqp-url", url, "-queue", q.Name, "-max-batch", fmt.Sprint(len(events)), "-flush-interval-millis", "100")
		out, e := cmd.CombinedOutput()
		outputs = append(outputs, string(out))
		if expectFailure {
			if e == nil || !strings.Contains(string(out), "1062") {
				return fmt.Errorf("expected strict duplicate key rejection, got %s: %w", out, e)
			}
			return nil
		}
		if e != nil {
			return fmt.Errorf("probe consumer: %s: %w", out, e)
		}
		if !strings.Contains(string(out), fmt.Sprintf("count=%d ", len(events))) {
			return fmt.Errorf("batch size not confirmed: %s", out)
		}
		pending, e = ch.QueueDeclarePassive(q.Name, false, false, false, false, nil)
		if e != nil {
			return e
		}
		if pending.Messages != 0 {
			return fmt.Errorf("probe queue did not drain")
		}
		return nil
	}
	insert := mk(appendTable, "I", 1, "INSERT")
	for round := 0; round < 2; round++ {
		if err = consume([]event.SyncEvent{insert, insert, insert, insert}, false); err != nil {
			return err
		}
	}
	stateInsert := mk(stateTable, "S", 1, "INSERT")
	stateInsert.After["i01"] = int64(9007199254740993)
	stateInsert.After["n01"] = json.Number("999999999999.123456")
	u1, u2 := mk(stateTable, "U1", 2, "UPDATE"), mk(stateTable, "U2", 3, "UPDATE")
	if err = consume([]event.SyncEvent{stateInsert}, false); err != nil {
		return err
	}
	boundaryRow, err := l.getRow(1, stateTable, int64(1))
	if err != nil {
		return err
	}
	if string(boundaryRow[12]) != "9007199254740993" {
		evidence["bigint_expected"], evidence["bigint_actual"] = "9007199254740993", string(boundaryRow[12])
		return fmt.Errorf("BIGINT precision lost: expected 9007199254740993, actual %s", boundaryRow[12])
	}
	if string(boundaryRow[24]) != "999999999999.123456" {
		return fmt.Errorf("DECIMAL precision lost: %s", boundaryRow[24])
	}
	if err = consume([]event.SyncEvent{u1, u2, u1}, false); err != nil {
		return err
	}
	if err = consume([]event.SyncEvent{u1}, false); err != nil {
		return err
	}
	actual, err := l.getRow(1, stateTable, int64(1))
	if err != nil {
		return err
	}
	expected := makeRow(l.c, l.sides[0], 1, 3)
	expected[6] = u2.EventID
	if err = compareRows(l.s, rawRow(expected), actual); err != nil {
		return err
	}
	for _, evt := range []event.SyncEvent{insert, stateInsert, u1, u2} {
		var applied, success int
		if err = l.db[1].QueryRow("SELECT (SELECT COUNT(*) FROM sync_apply_log WHERE event_id=?),(SELECT COUNT(*) FROM sync_event_log WHERE event_id=? AND status='SUCCESS')", evt.EventID, evt.EventID).Scan(&applied, &success); err != nil {
			return err
		}
		if applied != 1 || success != 1 {
			return fmt.Errorf("unique logs for %s: %d/%d", evt.EventID, applied, success)
		}
	}
	guardInsert := mk(guardTable, "guard-insert", 1, "INSERT")
	guardInsert.After["i01"] = int64(9007199254740993)
	if err = consume([]event.SyncEvent{guardInsert}, false); err != nil {
		return err
	}
	guardRow, err := l.getRow(1, guardTable, int64(1))
	if err != nil {
		return err
	}
	guardExpected := makeRow(l.c, l.sides[0], 1, 1)
	guardExpected[12] = nil
	guardExpected[6] = guardInsert.EventID
	if err = compareRows(l.s, rawRow(guardExpected), guardRow); err != nil {
		return fmt.Errorf("column exclusion: %w", err)
	}
	for _, operation := range []string{event.TypeAddColumn, event.TypeDropColumn} {
		denied := mk(guardTable, "denied-"+operation, 1, operation)
		columnName := "lab_forbidden"
		if operation == event.TypeDropColumn {
			columnName = "t01"
		}
		denied.After = nil
		denied.PrimaryKey = nil
		denied.SchemaChange = &dbgovernance.SchemaChange{Operation: operation, Column: dbgovernance.ColumnDefinition{Name: columnName, Type: "VARCHAR(64)", Nullable: true}}
		if err = consume([]event.SyncEvent{denied}, false); err != nil {
			return err
		}
		var columns, logs int
		if err = l.db[1].QueryRow("SELECT (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=?),(SELECT COUNT(*) FROM sync_apply_log WHERE event_id=?)", guardTable.Name, denied.EventID).Scan(&columns, &logs); err != nil {
			return err
		}
		if columns != 50 || logs != 0 {
			return fmt.Errorf("unauthorized DDL applied: columns=%d logs=%d", columns, logs)
		}
		still, e := l.getRow(1, guardTable, int64(1))
		if e != nil {
			return e
		}
		if e = compareRows(l.s, guardRow, still); e != nil {
			return e
		}
	}
	evidence["column_exclusion_verified"], evidence["unauthorized_add_drop_verified"] = true, true
	negative := mk(appendTable, "different-id-same-key", 1, "INSERT")
	if err = consume([]event.SyncEvent{negative}, true); err != nil {
		return err
	}
	var count, applied int
	if err = l.db[1].QueryRow("SELECT (SELECT COUNT(*) FROM "+quote(appendTable.Name)+"),(SELECT COUNT(*) FROM sync_apply_log WHERE event_id=?)", negative.EventID).Scan(&count, &applied); err != nil {
		return err
	}
	if count != 1 || applied != 0 {
		return fmt.Errorf("strict conflict mutated business/apply state: %d/%d", count, applied)
	}
	evidence["passed"], evidence["duplicate_deliveries"], evidence["final_revision"], evidence["strict_conflict_verified"] = true, 9, 3, true
	return nil
}
