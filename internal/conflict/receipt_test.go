package conflict

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSeenReceiptRequiresOriginalRowAndVersion(t *testing.T) {
	for _, change := range []string{"same", "row", "identity", "payload", "time", "corrupt"} {
		t.Run(change, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			incoming := version("node", "event", 1000000, false)
			stored := incoming
			if change == "payload" {
				stored.PayloadHash = "b" + stored.PayloadHash[1:]
			}
			if change == "time" {
				stored.Time = stored.Time.Add(1000)
			}
			encoded, _ := json.Marshal(stored)
			if change == "corrupt" {
				encoded = []byte("broken")
			}
			identity := eventIdentity(incoming)
			if change == "identity" {
				identity = []byte("another identity")
			}
			hash := sha256.Sum256([]byte("row"))
			storedHash := hash
			if change == "row" {
				storedHash[0] ^= 1
			}
			mock.ExpectQuery("SELECT row_hash,event_identity,version_json").WillReturnRows(sqlmock.NewRows([]string{"row_hash", "event_identity", "version_json"}).AddRow(storedHash[:], identity, encoded))
			seen, err := checkSeen(context.Background(), tx, hash[:], incoming)
			if change == "same" && (err != nil || !seen) || change != "same" && (err == nil || seen) {
				t.Fatalf("seen=%t error=%v", seen, err)
			}
			mock.ExpectRollback()
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
