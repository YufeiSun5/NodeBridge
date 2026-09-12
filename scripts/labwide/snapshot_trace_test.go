package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSnapshotQueryTraceAndCancellation(t *testing.T) {
	for _, slow := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "timeout"}[slow], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			expect := mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(9))
			if slow {
				expect.WillDelayFor(time.Second)
			}
			budget := time.Second
			if slow {
				budget = 50 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()
			var out bytes.Buffer
			var n int
			err = snapshotQuery(ctx, &out, db, "edge-001", "source", "SELECT COUNT(*) FROM private_table", nil, &n)
			if (err != nil) != slow {
				t.Fatalf("slow=%v err=%v", slow, err)
			}
			if !slow && n != 9 {
				t.Fatalf("count=%d", n)
			}
			if slow && !strings.Contains(err.Error(), "edge-001 source") {
				t.Fatal(err)
			}
			dec := json.NewDecoder(&out)
			var begin, end snapshotTiming
			if err := dec.Decode(&begin); err != nil {
				t.Fatal(err)
			}
			if err := dec.Decode(&end); err != nil {
				t.Fatal(err)
			}
			if begin.Phase != "start" || end.Phase != "end" || end.Failed != slow || end.ElapsedMS >= 500 {
				t.Fatalf("begin=%+v end=%+v", begin, end)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSnapshotStepStartWrittenBeforeBlocking(t *testing.T) {
	var out bytes.Buffer
	err := snapshotStep(&out, "server-001", "connect", func() error {
		if !strings.Contains(out.String(), `"phase":"start"`) {
			t.Fatal("missing live trace")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
