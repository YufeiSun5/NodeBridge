package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (l *lab) quarantine() error {
	url := os.Getenv("NODEBRIDGE_WIDE_AMQP")
	if url == "" {
		return fmt.Errorf("NODEBRIDGE_WIDE_AMQP required")
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
	queue := l.c.Prefix + ".quarantine"
	if _, err = ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}
	if err = ch.Confirm(false); err != nil {
		return err
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	records := []map[string]any{}
	defer func() {
		_ = atomicJSON(filepath.Join(l.root, "quarantine.json"), map[string]any{"at": time.Now(), "archive_queue": queue, "messages": records, "count": len(records)})
	}()
	for _, source := range []string{"edge-001.downlink.q", "server.cdc.ingress.q"} {
		for n := 0; n < 100000; n++ {
			msg, ok, err := ch.Get(source, false)
			if err != nil {
				return err
			}
			if !ok {
				break
			}
			var identity struct {
				EventID string `json:"event_id"`
				Table   string `json:"table_name"`
			}
			if err = json.Unmarshal(msg.Body, &identity); err != nil {
				return err
			}
			if identity.EventID == "" || !strings.HasPrefix(identity.Table, l.c.Prefix+"_") {
				if err = msg.Nack(false, true); err != nil {
					return err
				}
				return fmt.Errorf("foreign event at queue head; left untouched: %s", identity.EventID)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err = ch.PublishWithContext(ctx, "", queue, true, false, amqp.Publishing{ContentType: msg.ContentType, DeliveryMode: amqp.Persistent, Body: msg.Body, MessageId: msg.MessageId, Headers: amqp.Table{"lab_source_queue": source, "lab_run_id": l.c.RunID}})
			cancel()
			if err != nil {
				return err
			}
			select {
			case confirm := <-confirms:
				if !confirm.Ack {
					return fmt.Errorf("quarantine confirm NACK")
				}
			case <-time.After(10 * time.Second):
				return fmt.Errorf("quarantine confirm timeout")
			}
			if err = msg.Ack(false); err != nil {
				return err
			}
			records = append(records, map[string]any{"source_queue": source, "event_id": identity.EventID, "table": identity.Table, "bytes": len(msg.Body)})
		}
	}
	fmt.Printf("quarantined %d test messages to %s\n", len(records), queue)
	return nil
}
