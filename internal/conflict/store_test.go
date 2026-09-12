package conflict

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStoreAtomicWriteVersionAndReceipt(t *testing.T) {
	for _, step := range []string{"success", "write", "version", "receipt", "commit"} {
		t.Run(step, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			key := RowKey{Database: "db", Table: "rows", CanonicalKey: []byte("canonical-key")}
			identity, _ := json.Marshal(key)
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO sync_row_version").WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectQuery("SELECT row_identity,version_json").WillReturnRows(sqlmock.NewRows([]string{"row_identity", "version_json"}).AddRow(identity, nil))
			mock.ExpectQuery("SELECT row_hash,event_identity,version_json").WillReturnRows(sqlmock.NewRows([]string{"row_hash", "event_identity", "version_json"}))
			write := func(context.Context, *sql.Tx) error {
				if step == "write" {
					return errors.New("write failed")
				}
				return nil
			}
			if step != "write" {
				e := mock.ExpectExec("UPDATE sync_row_version")
				if step == "version" {
					e.WillReturnError(errors.New("version failed"))
				} else {
					e.WillReturnResult(sqlmock.NewResult(0, 1))
				}
			}
			if step != "write" && step != "version" {
				mock.ExpectExec("INSERT INTO sync_conflict_event").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			receipt := func(_ context.Context, _ *sql.Tx, d string) error {
				if d != Apply {
					t.Fatalf("unexpected %s", d)
				}
				if step == "receipt" {
					return errors.New("receipt failed")
				}
				return nil
			}
			switch step {
			case "success":
				mock.ExpectCommit()
			case "commit":
				mock.ExpectCommit().WillReturnError(errors.New("lost response"))
			default:
				mock.ExpectRollback()
			}
			got, err := (Store{DB: db}).Apply(context.Background(), key, version("node", "event", 1000000, true), write, receipt)
			if step == "success" && (err != nil || got != Apply) || step != "success" && (err == nil || got != "") {
				t.Fatalf("%s %v", got, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStoreLosingEventReceiptedWithoutBusinessWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	key := RowKey{Database: "db", Table: "rows", CanonicalKey: []byte("key")}
	identity, _ := json.Marshal(key)
	stored, _ := json.Marshal(version("node", "delete", 2000000, true))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO sync_row_version").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT row_identity,version_json").WillReturnRows(sqlmock.NewRows([]string{"row_identity", "version_json"}).AddRow(identity, stored))
	mock.ExpectQuery("SELECT row_hash,event_identity,version_json").WillReturnRows(sqlmock.NewRows([]string{"row_hash", "event_identity", "version_json"}))
	mock.ExpectExec("INSERT INTO sync_conflict_event").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	called := false
	got, err := (Store{DB: db}).Apply(context.Background(), key, version("node", "old", 1000000, false), func(context.Context, *sql.Tx) error { t.Fatal("old write resurrected deleted row"); return nil }, func(_ context.Context, _ *sql.Tx, d string) error {
		called = true
		if d != Superseded {
			t.Fatal(d)
		}
		return nil
	})
	if err != nil || got != Superseded || !called {
		t.Fatalf("%s %v %t", got, err, called)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyInTxLeavesTransactionWithCaller(t *testing.T) {
	for _, failReceipt := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "receipt_failure"}[failReceipt], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			key := RowKey{Database: "db", Table: "rows", CanonicalKey: []byte("key")}
			identity, _ := json.Marshal(key)
			mock.ExpectExec("INSERT INTO sync_row_version").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT row_identity,version_json").WillReturnRows(sqlmock.NewRows([]string{"row_identity", "version_json"}).AddRow(identity, nil))
			mock.ExpectQuery("SELECT row_hash,event_identity,version_json").WillReturnRows(sqlmock.NewRows([]string{"row_hash", "event_identity", "version_json"}))
			mock.ExpectExec("UPDATE sync_row_version").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO sync_conflict_event").WillReturnResult(sqlmock.NewResult(0, 1))
			failure := errors.New("receipt failed")
			got, err := ApplyInTx(context.Background(), tx, key, version("node", "event", 1000000, false), func(_ context.Context, actual *sql.Tx) error {
				if actual != tx {
					t.Fatal("business write used another transaction")
				}
				return nil
			}, func(_ context.Context, actual *sql.Tx, decision string) error {
				if actual != tx || decision != Apply {
					t.Fatal("receipt transaction or decision changed")
				}
				if failReceipt {
					return failure
				}
				return nil
			})
			if failReceipt {
				if !errors.Is(err, failure) || got != "" {
					t.Fatalf("got %q, %v", got, err)
				}
			} else if err != nil || got != Apply {
				t.Fatalf("got %q, %v", got, err)
			}
			// A rollback also after success proves no partial batch was committed.
			mock.ExpectRollback()
			if err := tx.Rollback(); err != nil {
				t.Fatalf("caller lost transaction ownership: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestApplyInTxRejectsMissingTransaction(t *testing.T) {
	_, err := ApplyInTx(context.Background(), nil, RowKey{}, Version{}, nil, nil)
	if err == nil || err.Error() != "conflict_transaction_dependencies_required" {
		t.Fatalf("got %v", err)
	}
}
