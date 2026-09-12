// Package capture provides an in-process CDC barrier, not a durable checkpoint.
package capture

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
)

const Table = "sync_capture_fence"

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

type Fence struct {
	db       *sql.DB
	database string
	node     string
	mu       sync.Mutex
	pending  map[string]chan struct{}
}

func NewFence(db *sql.DB, database, node string) (*Fence, error) {
	if db == nil || !identifier.MatchString(database) || node == "" || len(node) > 128 {
		return nil, errors.New("capture_fence_dependencies_required")
	}
	return &Fence{db: db, database: database, node: node, pending: make(map[string]chan struct{})}, nil
}

// Wait writes on a separate connection while the caller holds business row/gap
// locks, before taking version locks. The CDC pipeline must durably register all
// local versions before confirming batches. No persisted offset can replace this
// fresh pulse. The subscription must include the fence table and business tables.
func (f *Fence) Wait(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	token := hex.EncodeToString(random[:])
	done := make(chan struct{})
	f.mu.Lock()
	f.pending[token] = done
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		delete(f.pending, token)
		f.mu.Unlock()
	}()
	// One row per node bounds storage. Each concurrent pulse is a distinct binlog write.
	query := "INSERT INTO `" + f.database + "`.`" + Table + "` (node_id,token) VALUES (?,?) ON DUPLICATE KEY UPDATE token=?"
	if _, err := f.db.ExecContext(ctx, query, f.node, token, token); err != nil {
		return fmt.Errorf("write capture fence: %w", err)
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait capture fence: %w", ctx.Err())
	case <-done:
		return ctx.Err()
	}
}

func (f *Fence) isPulse(change cdc.ChangeEvent) bool {
	return change.DatabaseName == f.database && change.TableName == Table
}

func (f *Fence) observe(changes []cdc.ChangeEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, change := range changes {
		if !f.isPulse(change) || (change.Operation != cdc.OperationInsert && change.Operation != cdc.OperationUpdate) {
			continue
		}
		node, _ := change.After["node_id"].(string)
		token, _ := change.After["token"].(string)
		if node != f.node {
			continue
		}
		if done, ok := f.pending[token]; ok {
			close(done)
			delete(f.pending, token)
		}
	}
}
