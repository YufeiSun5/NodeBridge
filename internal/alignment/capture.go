package alignment

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/capture"
	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/cdc/canal"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

// SnapshotCaptureClient deliberately has no Ack operation. Probe reads must be
// rolled back on close so normal capture can replay every observed business row.
type SnapshotCaptureClient interface {
	Connect(context.Context) error
	Subscribe(context.Context, string) error
	Fetch(context.Context, int) ([]canal.RowChange, cdc.Offset, error)
	Close(context.Context) error
}

type SnapshotBoundary struct {
	OriginNodeID    string `json:"origin_node_id"`
	DatabaseName    string `json:"database_name"`
	TableName       string `json:"table_name"`
	MySQLServerUUID string `json:"mysql_server_uuid"`
	BinlogFile      string `json:"binlog_file"`
	BinlogPos       uint32 `json:"binlog_pos"`
	Token           string `json:"token"`
}

var snapshotBinlogName = regexp.MustCompile(`^(.+)\.([0-9]+)$`)
var snapshotMySQLUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (b SnapshotBoundary) Validate() error {
	if b.OriginNodeID == "" || len(b.OriginNodeID) > 128 || b.BinlogPos == 0 || !snapshotMySQLUUID.MatchString(b.MySQLServerUUID) || len(b.Token) != 64 {
		return errors.New("alignment_capture_boundary_invalid")
	}
	if err := validateTable(b.DatabaseName, b.TableName); err != nil {
		return err
	}
	if _, err := hex.DecodeString(b.Token); err != nil {
		return errors.New("alignment_capture_boundary_invalid")
	}
	_, _, err := snapshotBinlogParts(b.BinlogFile)
	return err
}

func snapshotBinlogParts(file string) (string, uint64, error) {
	parts := snapshotBinlogName.FindStringSubmatch(file)
	if len(parts) != 3 {
		return "", 0, errors.New("alignment_binlog_name_invalid")
	}
	sequence, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil || sequence == 0 {
		return "", 0, errors.New("alignment_binlog_name_invalid")
	}
	return parts[1], sequence, nil
}

// BeforePosition is an exclusive boundary comparison. The caller must first
// verify producer UUID, origin and table; positions from another stream cannot
// be compared merely because their filenames look alike.
func (b SnapshotBoundary) BeforePosition(file string, position uint32) (bool, error) {
	if err := b.Validate(); err != nil {
		return false, err
	}
	if position == 0 {
		return false, errors.New("alignment_binlog_position_required")
	}
	prefix, sequence, err := snapshotBinlogParts(file)
	if err != nil {
		return false, err
	}
	boundaryPrefix, boundarySequence, err := snapshotBinlogParts(b.BinlogFile)
	if err != nil {
		return false, err
	}
	if prefix != boundaryPrefix {
		return false, errors.New("alignment_binlog_lineage_changed")
	}
	return sequence < boundarySequence || sequence == boundarySequence && position < b.BinlogPos, nil
}

// SnapshotCapture is single-consumer and owns its exclusive Canal client.
// It observes markers only; it does not persist cutover state or enable rules.
type SnapshotCapture struct {
	planID  string
	client  SnapshotCaptureClient
	control string
	base    SnapshotBoundary
	markers map[string]SnapshotBoundary
	batches int
	closed  bool
}

func StartSnapshotCapture(ctx context.Context, db *sql.DB, client SnapshotCaptureClient, destination string, plan Plan, rule rules.SyncRule, nodeID string, confirm bool) (*SnapshotCapture, error) {
	if err := validateStreamPlan(plan, rule, confirm); err != nil {
		return nil, err
	}
	return startSnapshotCapture(ctx, db, client, destination, plan, nodeID)
}

// StartSnapshotRecoveryCapture permits an expired plan only for inspecting an
// existing target commit. It cannot authorize another copy or replace its marker.
func StartSnapshotRecoveryCapture(ctx context.Context, db *sql.DB, client SnapshotCaptureClient, destination string, plan Plan, rule rules.SyncRule, nodeID string, confirm bool) (*SnapshotCapture, error) {
	if err := plan.Validate(rule, plan.CreatedAt, confirm); err != nil {
		return nil, err
	}
	if rule.Enable || nodeID != plan.Target.NodeID {
		return nil, errors.New("alignment_recovery_endpoint_invalid")
	}
	job, err := ReadSnapshotJob(ctx, db, plan, nodeID)
	if err != nil {
		return nil, err
	}
	if job.Phase != JobTargetCommitted || job.CaptureMarker == "" {
		return nil, errors.New("alignment_recovery_capture_not_committed")
	}
	return startSnapshotCapture(ctx, db, client, destination, plan, nodeID)
}

func startSnapshotCapture(ctx context.Context, db *sql.DB, client SnapshotCaptureClient, destination string, plan Plan, nodeID string) (*SnapshotCapture, error) {
	if db == nil || client == nil || destination == "" || len(nodeID) > 128 {
		return nil, errors.New("alignment_capture_dependencies_required")
	}
	_, _, role, err := jobIdentity(plan, nodeID)
	if err != nil {
		return nil, err
	}
	local := plan.Source
	if role == "TARGET" {
		local = plan.Target
	}
	p := &SnapshotCapture{planID: plan.ID, client: client, markers: make(map[string]SnapshotBoundary), base: SnapshotBoundary{OriginNodeID: nodeID, DatabaseName: local.Schema.Database, TableName: local.Schema.Table}}
	if err := db.QueryRowContext(ctx, "SELECT DATABASE(),@@server_uuid").Scan(&p.control, &p.base.MySQLServerUUID); err != nil {
		return nil, err
	}
	if err := mapper.ValidateIdentifier(p.control); err != nil {
		return nil, err
	}
	if !snapshotMySQLUUID.MatchString(p.base.MySQLServerUUID) {
		return nil, errors.New("alignment_source_uuid_invalid")
	}
	ok := false
	defer func() {
		if !ok {
			_ = p.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	if err := client.Subscribe(ctx, destination); err != nil {
		return nil, err
	}
	// An asynchronous, first-ever Canal startup may begin after the first pulse.
	// Keep issuing fresh pulses until one written after subscription is observed.
	issued := make(map[string]bool)
	nextPulse := time.Time{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("alignment_capture_not_ready: %w", err)
		}
		if !time.Now().Before(nextPulse) {
			token, err := p.WriteMarker(ctx, db)
			if err != nil {
				return nil, err
			}
			issued[token] = true
			nextPulse = time.Now().Add(250 * time.Millisecond)
		}
		if err := p.fetch(ctx); err != nil {
			return nil, err
		}
		for token := range issued {
			if _, found := p.markers[token]; found {
				ok = true
				return p, nil
			}
		}
	}
}

type markerWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// WriteMarker may participate in the target copy transaction. An uncommitted or
// rolled-back marker must never satisfy WaitMarker. Writers must be this local DB.
func (p *SnapshotCapture) WriteMarker(ctx context.Context, writer markerWriter) (string, error) {
	if p.closed || writer == nil {
		return "", errors.New("alignment_capture_closed")
	}
	var control, serverUUID string
	if err := writer.QueryRowContext(ctx, "SELECT DATABASE(),@@server_uuid").Scan(&control, &serverUUID); err != nil {
		return "", err
	}
	if control != p.control || serverUUID != p.base.MySQLServerUUID {
		return "", errors.New("alignment_capture_writer_mismatch")
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(random[:])
	query := "INSERT INTO `" + p.control + "`.`" + capture.Table + "` (node_id,token) VALUES (?,?) ON DUPLICATE KEY UPDATE token=?"
	if _, err := writer.ExecContext(ctx, query, p.base.OriginNodeID, token, token); err != nil {
		return "", err
	}
	return token, nil
}

func (p *SnapshotCapture) WaitMarker(ctx context.Context, token string) (SnapshotBoundary, error) {
	if p.closed || len(token) != 64 {
		return SnapshotBoundary{}, errors.New("alignment_capture_marker_invalid")
	}
	if _, err := hex.DecodeString(token); err != nil {
		return SnapshotBoundary{}, errors.New("alignment_capture_marker_invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return SnapshotBoundary{}, fmt.Errorf("alignment_capture_marker_unobserved: %w", err)
		}
		if boundary, ok := p.markers[token]; ok {
			return boundary, nil
		}
		if err := p.fetch(ctx); err != nil {
			return SnapshotBoundary{}, err
		}
	}
}

func (p *SnapshotCapture) fetch(ctx context.Context) error {
	rows, offset, err := p.client.Fetch(ctx, 256)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(rows) != 0 && !offset.HasCanalBatch() {
		return errors.New("alignment_capture_batch_missing")
	}
	if offset.HasCanalBatch() {
		p.batches++
		if p.batches > 1024 {
			return errors.New("alignment_capture_backlog_limit")
		}
	}
	for _, row := range rows {
		if row.DatabaseName != p.control || row.TableName != capture.Table || (row.Operation != cdc.OperationInsert && row.Operation != cdc.OperationUpdate) {
			continue
		}
		node, _ := row.After["node_id"].(string)
		token, _ := row.After["token"].(string)
		if node != p.base.OriginNodeID {
			continue
		}
		boundary := p.base
		boundary.Token, boundary.BinlogFile, boundary.BinlogPos = token, row.BinlogFile, row.BinlogPos
		if err := boundary.Validate(); err != nil {
			return err
		}
		if previous, exists := p.markers[token]; exists && previous != boundary {
			return errors.New("alignment_capture_marker_reused")
		}
		p.markers[token] = boundary
		if len(p.markers) > 4096 {
			return errors.New("alignment_capture_marker_limit")
		}
	}
	return nil
}

func (p *SnapshotCapture) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return p.client.Close(context.Background())
}
