package capture

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/YufeiSun5/NodeBridge/internal/cdc"
)

type tokenArgument struct{ tokens chan string }

func (a tokenArgument) Match(value driver.Value) bool {
	token, ok := value.(string)
	if !ok || len(token) != 64 {
		return false
	}
	a.tokens <- token
	return true
}

func pulse(token string) cdc.ChangeEvent {
	return cdc.ChangeEvent{DatabaseName: "db", TableName: Table, Operation: cdc.OperationUpdate, After: map[string]any{"node_id": "node", "token": token}}
}

func TestFenceWaitRequiresExactObservedPulse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	f, err := NewFence(db, "db", "node")
	if err != nil {
		t.Fatal(err)
	}
	tokens := make(chan string, 1)
	mock.ExpectExec("INSERT INTO `db`.`sync_capture_fence`").WithArgs("node", tokenArgument{tokens}, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- f.Wait(ctx) }()
	var token string
	select {
	case token = <-tokens:
	case <-ctx.Done():
		t.Fatal("pulse was not written")
	}
	wrong := []cdc.ChangeEvent{pulse("old-token"), pulse(token), pulse(token), pulse(token), pulse(token)}
	wrong[1].DatabaseName = "another"
	wrong[2].TableName = "another"
	wrong[3].After["node_id"] = "another"
	wrong[4].Operation = cdc.OperationDelete
	f.observe(wrong)
	select {
	case err := <-result:
		t.Fatalf("wrong pulse released barrier: %v", err)
	default:
	}
	f.observe([]cdc.ChangeEvent{pulse(token), pulse(token)})
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if len(f.pending) != 0 {
		t.Fatal("completed wait leaked")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFenceFailureAndCancellationReleasePendingWait(t *testing.T) {
	for _, failSQL := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		f, err := NewFence(db, "db", "node")
		if err != nil {
			t.Fatal(err)
		}
		tokens := make(chan string, 1)
		query := mock.ExpectExec("INSERT INTO").WithArgs("node", tokenArgument{tokens}, sqlmock.AnyArg())
		if failSQL {
			query.WillReturnError(errors.New("database down"))
		} else {
			query.WillReturnResult(sqlmock.NewResult(0, 1))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		result := make(chan error, 1)
		go func() { result <- f.Wait(ctx) }()
		select {
		case <-tokens:
		case <-ctx.Done():
			t.Fatal("no pulse")
		}
		if !failSQL {
			cancel()
		}
		if err := <-result; err == nil || (!failSQL && !errors.Is(err, context.Canceled)) {
			t.Fatalf("unexpected result %v", err)
		}
		cancel()
		if len(f.pending) != 0 {
			t.Fatal("failed wait leaked")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestFenceValidatesDependencies(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{"", "db`;DROP", strings.Repeat("x", 65)} {
		if _, err := NewFence(db, name, "node"); err == nil {
			t.Fatal("accepted invalid database")
		}
	}
	if _, err := NewFence(nil, "db", "node"); err == nil {
		t.Fatal("accepted nil database")
	}
}
