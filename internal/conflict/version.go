package conflict

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	SourceBinlog  = "source_binlog"
	SourceCommand = "source_command"
	Apply         = "APPLY"
	Duplicate     = "DUPLICATE"
	Superseded    = "SUPERSEDED"
)

// Version is immutable across routing, retries, restarts and target writes.
type Version struct {
	Time         time.Time `json:"time"`
	TimeSource   string    `json:"time_source"`
	OriginNodeID string    `json:"origin_node_id"`
	EventID      string    `json:"event_id"`
	PayloadHash  string    `json:"payload_hash"`
	Deleted      bool      `json:"deleted"`
	BinlogFile   string    `json:"binlog_file,omitempty"`
	BinlogPos    uint32    `json:"binlog_pos,omitempty"`
}

func (v Version) Validate() error {
	if !v.Time.After(time.Unix(0, 0)) || v.Time.Year() > 9999 {
		return errors.New("conflict_time_required: a valid source event time is required")
	}
	if v.Time.Nanosecond()%1000 != 0 {
		return errors.New("conflict_time_precision: maximum supported precision is microseconds")
	}
	if v.TimeSource != SourceBinlog && v.TimeSource != SourceCommand {
		return errors.New("conflict_time_source: processing time is not a source event time")
	}
	if v.TimeSource == SourceBinlog {
		if _, _, err := binlogSequence(v.BinlogFile); err != nil || v.BinlogPos == 0 {
			return errors.New("conflict_source_position_required: invalid binlog position")
		}
	}
	if strings.TrimSpace(v.OriginNodeID) == "" || strings.TrimSpace(v.EventID) == "" || len(v.PayloadHash) != 64 {
		return errors.New("conflict_identity_required: origin, event id and SHA256 payload hash are required")
	}
	for _, c := range v.PayloadHash {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return errors.New("conflict_payload_hash: expected lowercase SHA256")
		}
	}
	return nil
}

// Resolve keeps tombstones in the same order as writes. Arrival time is irrelevant.
func Resolve(current *Version, incoming Version) (string, error) {
	if err := incoming.Validate(); err != nil {
		return "", err
	}
	if current == nil {
		return Apply, nil
	}
	if err := current.Validate(); err != nil {
		return "", fmt.Errorf("invalid stored version: %w", err)
	}
	if current.TimeSource != incoming.TimeSource {
		return "", errors.New("conflict_clock_mismatch: cannot mix command and binlog clocks")
	}
	if current.OriginNodeID == incoming.OriginNodeID && current.EventID == incoming.EventID {
		if !current.Time.Equal(incoming.Time) || current.PayloadHash != incoming.PayloadHash || current.Deleted != incoming.Deleted || current.BinlogFile != incoming.BinlogFile || current.BinlogPos != incoming.BinlogPos {
			return "", errors.New("conflict_event_identity_reused: event content or time changed")
		}
		return Duplicate, nil
	}
	if incoming.Time.After(current.Time) {
		return Apply, nil
	}
	if incoming.Time.Before(current.Time) {
		return Superseded, nil
	}
	if incoming.OriginNodeID == current.OriginNodeID && incoming.TimeSource == SourceBinlog {
		prefix, sequence, _ := binlogSequence(incoming.BinlogFile)
		oldPrefix, oldSequence, _ := binlogSequence(current.BinlogFile)
		if prefix != oldPrefix {
			return "", errors.New("conflict_binlog_lineage_changed")
		}
		if sequence != oldSequence {
			if sequence > oldSequence {
				return Apply, nil
			}
			return Superseded, nil
		}
		if incoming.BinlogPos != current.BinlogPos {
			if incoming.BinlogPos > current.BinlogPos {
				return Apply, nil
			}
			return Superseded, nil
		}
	}
	// A fixed tie-break converges even when the source clock has second resolution.
	if incoming.OriginNodeID > current.OriginNodeID || incoming.OriginNodeID == current.OriginNodeID && incoming.EventID > current.EventID {
		return Apply, nil
	}
	return Superseded, nil
}

func binlogSequence(file string) (string, uint64, error) {
	index := strings.LastIndexByte(file, '.')
	if index < 1 || index == len(file)-1 {
		return "", 0, errors.New("invalid binlog file")
	}
	sequence, err := strconv.ParseUint(file[index+1:], 10, 64)
	return file[:index], sequence, err
}
