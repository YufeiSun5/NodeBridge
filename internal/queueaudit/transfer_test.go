package queueaudit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type testAck struct {
	ack, nack bool
	fail      bool
}

func (a *testAck) Ack(uint64, bool) error {
	if a.fail {
		return errors.New("ack failed")
	}
	a.ack = true
	return nil
}
func (a *testAck) Nack(_ uint64, _, requeue bool) error { a.nack = requeue; return nil }
func (a *testAck) Reject(uint64, bool) error            { return errors.New("unexpected reject") }

type testGetter struct{ rows []amqp091.Delivery }

func (g *testGetter) Get(string, bool) (amqp091.Delivery, bool, error) {
	if len(g.rows) == 0 {
		return amqp091.Delivery{}, false, nil
	}
	d := g.rows[0]
	g.rows = g.rows[1:]
	return d, true, nil
}

type testJournal struct {
	receipt Receipt
	fail    string
}

func (j *testJournal) Load(string) (Receipt, error) { return j.receipt, nil }
func (j *testJournal) Save(r Receipt) error {
	if r.State == j.fail {
		return errors.New("journal failed")
	}
	j.receipt = r
	return nil
}

func TestTransferOnlyAcknowledgesAfterConfirmedAudit(t *testing.T) {
	for _, failure := range []string{"", "publishing", "publish", "confirmed", "ack", "acked"} {
		t.Run(failure, func(t *testing.T) {
			now := time.Now().UTC()
			ack, unrelated := &testAck{fail: failure == "ack"}, &testAck{}
			delivery := amqp091.Delivery{Body: []byte(`{"event_id":"wanted"}`), DeliveryTag: 2, Acknowledger: ack}
			fingerprint, err := Fingerprint(delivery)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := Seal(Plan{NodeID: "n", SourceQueue: "source", TargetQueue: "quarantine", RuleID: "r", RuleRevision: "rev", EventID: "wanted", Fingerprint: fingerprint, ExpiresAt: now.Add(time.Minute)})
			if err != nil {
				t.Fatal(err)
			}
			journal := &testJournal{receipt: Receipt{Plan: plan, State: "planned"}, fail: failure}
			getter := &testGetter{rows: []amqp091.Delivery{{Body: []byte(`{"event_id":"other"}`), DeliveryTag: 1, Acknowledger: unrelated}, delivery}}
			published := false
			publish := func(context.Context, string, amqp091.Delivery, string) error {
				if ack.ack || journal.receipt.State != "publishing" {
					t.Fatal("unsafe publication ordering")
				}
				if failure == "publish" {
					return errors.New("publish failed")
				}
				published = true
				return nil
			}
			_, err = Transfer(context.Background(), getter, journal, publish, plan, "rev", now)
			if (failure == "") != (err == nil) {
				t.Fatalf("failure=%s err=%v", failure, err)
			}
			wantAck := failure == "" || failure == "acked"
			if ack.ack != wantAck || !unrelated.nack || unrelated.ack {
				t.Fatalf("ack=%+v unrelated=%+v", ack, unrelated)
			}
			if ack.ack && (!published || journal.receipt.State != "confirmed" && journal.receipt.State != "acked") {
				t.Fatal("ACK before durable copy audit")
			}
			if !ack.ack && !ack.nack {
				t.Fatal("original not returned")
			}
		})
	}
}

func TestFileJournalRejectsInvalidPathAndTamper(t *testing.T) {
	j := FileJournal{Directory: t.TempDir()}
	if _, err := j.Load("../escape"); err == nil {
		t.Fatal("accepted path")
	}
	plan, err := Seal(Plan{NodeID: "n", SourceQueue: "s", TargetQueue: "t", RuleID: "r", RuleRevision: "rev", EventID: "e", Fingerprint: string(make([]byte, 64)), ExpiresAt: time.Now().UTC().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Save(Receipt{Plan: plan, State: "planned"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := j.Load(plan.ID)
	if err != nil || loaded.Plan != plan {
		t.Fatalf("%+v %v", loaded, err)
	}
	plan.TargetQueue = "changed"
	if err := j.Save(Receipt{Plan: plan, State: "planned"}); err == nil {
		t.Fatal("accepted tampered plan")
	}
}
