package alignment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
)

const EpochHeader = "alignment_epoch"
const SourceUUIDHeader = "source_mysql_uuid"

type CutoverFilter struct {
	NodeID string
	DB     *sql.DB
	Proofs []CutoverProof
}

func NewCutoverFilter(node string, db *sql.DB, proofs []CutoverProof) (*CutoverFilter, error) {
	f := &CutoverFilter{NodeID: node, DB: db, Proofs: append([]CutoverProof(nil), proofs...)}
	seen := make(map[string]bool)
	groups := make(map[string][]CutoverProof)
	for _, proof := range proofs {
		if err := proof.Validate(); err != nil {
			return nil, err
		}
		if seen[proof.ID] {
			return nil, errors.New("alignment_overlapping_cutovers")
		}
		seen[proof.ID] = true
		pair := proof.ObservedPair()
		key := pair.ServerNode + ":" + pair.Server.Database + "." + pair.Server.Table
		groups[key] = append(groups[key], proof)
	}
	localScopes := map[string]bool{}
	for _, group := range groups {
		local, err := group[0].Local(node)
		if len(group) > 1 {
			topology, topologyErr := BuildTopologyProof(group)
			if topologyErr != nil {
				return nil, topologyErr
			}
			local, err = topology.Local(node)
		}
		if err != nil {
			return nil, err
		}
		key := local.DatabaseName + "." + local.TableName
		if localScopes[key] {
			return nil, errors.New("alignment_overlapping_cutovers")
		}
		localScopes[key] = true
	}
	return f, nil
}

func (f *CutoverFilter) find(origin, database, table string) (*CutoverProof, SnapshotBoundary) {
	var first *CutoverProof
	var earliest SnapshotBoundary
	for i := range f.Proofs {
		proof := &f.Proofs[i]
		for _, boundary := range []SnapshotBoundary{proof.Source, proof.Target} {
			if boundary.OriginNodeID == origin && boundary.DatabaseName == database && boundary.TableName == table {
				before, _ := earliest.BeforePosition(boundary.BinlogFile, boundary.BinlogPos)
				if first == nil || before || (boundary.BinlogFile == earliest.BinlogFile && boundary.BinlogPos == earliest.BinlogPos && proof.ID < first.ID) {
					first, earliest = proof, boundary
				}
			}
		}
	}
	return first, earliest
}

func (f *CutoverFilter) SkipChange(change cdc.ChangeEvent) (bool, error) {
	proof, boundary := f.find(f.NodeID, change.DatabaseName, change.TableName)
	if proof == nil {
		return false, nil
	}
	return boundary.BeforePosition(change.BinlogFile, change.BinlogPos)
}

func (f *CutoverFilter) Stamp(evt event.SyncEvent) (event.SyncEvent, error) {
	proof, boundary := f.find(evt.OriginNodeID, evt.DatabaseName, evt.TableName)
	if proof == nil {
		return evt, nil
	}
	if evt.OriginNodeID != f.NodeID {
		return event.SyncEvent{}, errors.New("alignment_cannot_stamp_remote_event")
	}
	before, err := boundary.BeforePosition(evt.BinlogFile, evt.BinlogPos)
	if err != nil || before {
		return event.SyncEvent{}, errors.Join(errors.New("alignment_capture_before_boundary"), err)
	}
	// A first alignment creates a new event namespace without deleting old receipts.
	digest := sha256.Sum256([]byte(proof.ID + ":" + evt.EventID))
	evt.EventID = hex.EncodeToString(digest[:])
	evt.TraceID = evt.EventID
	headers := make(map[string]string, len(evt.Headers)+2)
	for key, value := range evt.Headers {
		headers[key] = value
	}
	headers[EpochHeader], headers[SourceUUIDHeader] = proof.ID, boundary.MySQLServerUUID
	evt.Headers = headers
	return evt, nil
}

// Superseded only classifies known members of a durably aligned topology.
// Missing epochs are legacy inputs captured before the coordinated first copy;
// unknown nonempty epochs fail instead of guessing whether they are older.
func (f *CutoverFilter) Superseded(evt event.SyncEvent) (*CutoverProof, bool, error) {
	proof, boundary := f.find(evt.OriginNodeID, evt.DatabaseName, evt.TableName)
	if proof == nil {
		return nil, false, nil
	}
	pair := proof.ObservedPair()
	if evt.EventID == "" || len(evt.EventID) > 128 || (evt.TargetNodeID != "" && evt.TargetNodeID != f.NodeID) || (evt.SourceNodeID != evt.OriginNodeID && evt.SourceNodeID != pair.ServerNode) {
		return nil, false, errors.New("alignment_incoming_identity_invalid")
	}
	epoch := evt.Headers[EpochHeader]
	if epoch == "" {
		return proof, true, nil
	}
	if epoch != proof.ID || evt.Headers[SourceUUIDHeader] != boundary.MySQLServerUUID {
		return nil, false, errors.New("alignment_incoming_lineage_changed")
	}
	// Later snapshots include row values, not conflict versions. Every member
	// must replay post-epoch events to reconstruct the same winners and tombstones.
	before, err := boundary.BeforePosition(evt.BinlogFile, evt.BinlogPos)
	if err != nil {
		return nil, false, err
	}
	return proof, before, nil
}

func (f *CutoverFilter) archive(ctx context.Context, body []byte) (bool, error) {
	if len(f.Proofs) == 0 {
		return false, nil
	}
	var evt event.SyncEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return false, err
	}
	proof, superseded, err := f.Superseded(evt)
	if err != nil || !superseded {
		return false, err
	}
	if f.DB == nil {
		return false, errors.New("alignment_event_audit_required")
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(proof.ID))
	_, _ = digest.Write(body)
	_, err = f.DB.ExecContext(ctx, "INSERT INTO sync_alignment_event (message_hash,proof_id,event_id,event_json,reason,recorded_at) VALUES (?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE message_hash=message_hash", hex.EncodeToString(digest.Sum(nil)), proof.ID, evt.EventID, body, "snapshot_superseded")
	return err == nil, err
}

type ChangeSource interface {
	Start(context.Context) error
	Stop(context.Context) error
	FetchChangesOnce(context.Context) ([]cdc.ChangeEvent, cdc.Offset, error)
	Commit(context.Context, cdc.Offset) error
}

type CutoverSource struct {
	Source ChangeSource
	Filter *CutoverFilter
}

func (s *CutoverSource) Start(ctx context.Context) error {
	if s.Source == nil || s.Filter == nil || s.Filter.DB == nil {
		return errors.New("alignment_capture_dependencies_required")
	}
	var uuid string
	if err := s.Filter.DB.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&uuid); err != nil {
		return err
	}
	for _, proof := range s.Filter.Proofs {
		local, err := proof.Local(s.Filter.NodeID)
		if err != nil {
			continue
		}
		if local.MySQLServerUUID != uuid {
			return errors.New("alignment_capture_server_changed")
		}
	}
	return s.Source.Start(ctx)
}

func (s *CutoverSource) Stop(ctx context.Context) error { return s.Source.Stop(ctx) }

func (s *CutoverSource) FetchChangesOnce(ctx context.Context) ([]cdc.ChangeEvent, cdc.Offset, error) {
	changes, offset, err := s.Source.FetchChangesOnce(ctx)
	if err != nil {
		return nil, cdc.Offset{}, err
	}
	kept := make([]cdc.ChangeEvent, 0, len(changes))
	for _, change := range changes {
		skip, err := s.Filter.SkipChange(change)
		if err != nil {
			return nil, cdc.Offset{}, err
		}
		if !skip {
			kept = append(kept, change)
		}
	}
	return kept, offset, nil
}

func (s *CutoverSource) Commit(ctx context.Context, offset cdc.Offset) error {
	return s.Source.Commit(ctx, offset)
}

type ChangeNormalizer interface {
	Normalize(cdc.ChangeEvent) (event.SyncEvent, error)
}

type CutoverNormalizer struct {
	Normalizer ChangeNormalizer
	Filter     *CutoverFilter
}

func (n CutoverNormalizer) Normalize(change cdc.ChangeEvent) (event.SyncEvent, error) {
	evt, err := n.Normalizer.Normalize(change)
	if err != nil {
		return event.SyncEvent{}, err
	}
	return n.Filter.Stamp(evt)
}

type BatchSource interface {
	GetBatch(context.Context, int, time.Duration) ([]rabbitmq.IncomingMessage, error)
}

// CutoverMessages archives the exact old payload durably before ACK. Unrelated
// messages continue unchanged; errors return every still-unprocessed delivery.
type CutoverMessages struct {
	Source BatchSource
	Filter *CutoverFilter
}

func (s CutoverMessages) GetBatch(ctx context.Context, limit int, flush time.Duration) ([]rabbitmq.IncomingMessage, error) {
	messages, err := s.Source.GetBatch(ctx, limit, flush)
	if err != nil {
		return nil, err
	}
	kept := make([]rabbitmq.IncomingMessage, 0, len(messages))
	removed := make(map[int]bool)
	for i, msg := range messages {
		archived, err := s.Filter.archive(ctx, msg.Body())
		if err == nil && archived {
			err = msg.Ack(false)
			removed[i] = err == nil
		}
		if err != nil {
			for j, original := range messages {
				if !removed[j] {
					err = errors.Join(err, original.Nack(false, true))
				}
			}
			return nil, fmt.Errorf("alignment_incoming_filter: %w", err)
		}
		if !archived {
			kept = append(kept, msg)
		}
	}
	return kept, nil
}
