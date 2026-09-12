package eventstatus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentlog"
	"github.com/YufeiSun5/NodeBridge/internal/conflict"
)

type Request struct {
	EventID string `json:"event_id,omitempty"`
	RuleID  string `json:"rule_id,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

type Observation struct {
	EventID        string `json:"event_id"`
	RuleID         string `json:"rule_id,omitempty"`
	State          string `json:"state"`
	ApplyStatus    string `json:"apply_status"`
	AppliedAt      string `json:"applied_at,omitempty"`
	Worker         string `json:"worker,omitempty"`
	SourceDatabase string `json:"source_database,omitempty"`
	SourceTable    string `json:"source_table,omitempty"`
	TargetDatabase string `json:"target_database,omitempty"`
	TargetTable    string `json:"target_table,omitempty"`
	BinlogFile     string `json:"binlog_file,omitempty"`
	BinlogPos      uint32 `json:"binlog_pos,omitempty"`
	GTID           string `json:"gtid,omitempty"`
	LastError      string `json:"last_error,omitempty"`
	MySQLErrorCode int    `json:"mysql_error_code,omitempty"`
	LastErrorAt    string `json:"last_error_at,omitempty"`
	NextRetryAt    string `json:"next_retry_at,omitempty"`
	Errors         int    `json:"consecutive_errors,omitempty"`
}

type Response struct {
	Items    []Observation `json:"items"`
	Scope    string        `json:"scope"`
	Warnings []string      `json:"warnings"`
	Complete bool          `json:"complete"`
}

type Service struct {
	DB         *sql.DB
	LogPath    string
	RunningPID int
	Redact     func(string) string
}

type logRecord struct {
	Observation
	Time         string `json:"time"`
	PID          int    `json:"pid"`
	Message      string `json:"msg"`
	Error        string `json:"error"`
	RetryAt      string `json:"retry_at"`
	RetryAfterMS int64  `json:"retry_after_ms"`
}

func (s Service) Query(ctx context.Context, req Request) (Response, error) {
	result := Response{Items: []Observation{}, Warnings: []string{}, Scope: "bounded_runtime_errors_and_apply_receipts", Complete: false}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit < 1 || req.Limit > 50 {
		return result, fmt.Errorf("limit must be 1..50")
	}
	if len(req.EventID) > 128 || len(req.RuleID) > 128 {
		return result, fmt.Errorf("event_id and rule_id must be at most 128 bytes")
	}
	latest := map[string]logRecord{}
	for backup := 0; backup <= 4; backup++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		path := s.LogPath
		if backup > 0 {
			path = fmt.Sprintf("%s.%d", path, backup)
		}
		lines, err := agentlog.TailFiltered(path, 500, func(line string) bool {
			var record logRecord
			if json.Unmarshal([]byte(line), &record) != nil || record.EventID == "" {
				return false
			}
			if req.EventID != "" && record.EventID != req.EventID {
				return false
			}
			if req.RuleID != "" && record.RuleID != req.RuleID {
				return false
			}
			return record.Error != "" || record.Message == "worker step recovered"
		})
		if err != nil {
			if backup == 0 {
				result.Warnings = append(result.Warnings, "runtime_log_unavailable")
			}
			continue
		}
		for _, line := range lines {
			var record logRecord
			if json.Unmarshal([]byte(line), &record) != nil {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, record.Time)
			if err != nil {
				continue
			}
			previous, exists := latest[record.EventID]
			prevTime, _ := time.Parse(time.RFC3339Nano, previous.Time)
			if !exists || at.After(prevTime) {
				latest[record.EventID] = record
			}
		}
	}
	if req.EventID != "" {
		if _, ok := latest[req.EventID]; !ok {
			latest[req.EventID] = logRecord{Observation: Observation{EventID: req.EventID, RuleID: req.RuleID}}
		}
	}
	records := make([]logRecord, 0, len(latest))
	for _, record := range latest {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339Nano, records[i].Time)
		b, _ := time.Parse(time.RFC3339Nano, records[j].Time)
		return a.After(b)
	})
	if len(records) > req.Limit {
		records = records[:req.Limit]
	}
	for _, record := range records {
		item := record.Observation
		item.State = "unknown"
		item.ApplyStatus = "unknown"
		if record.Error != "" {
			item.LastError = record.Error
			item.LastErrorAt = record.Time
			item.State = "retry_pending"
			item.NextRetryAt = record.RetryAt
			if item.NextRetryAt == "" {
				at, _ := time.Parse(time.RFC3339Nano, record.Time)
				item.NextRetryAt = at.Add(time.Duration(record.RetryAfterMS) * time.Millisecond).Format(time.RFC3339Nano)
			}
			if s.RunningPID > 0 && record.PID == s.RunningPID {
				item.State = "retrying"
			}
		}
		if s.DB != nil {
			var applied time.Time
			var origin string
			err := s.DB.QueryRowContext(ctx, "SELECT database_name,table_name,COALESCE(target_database_name,''),COALESCE(target_table_name,''),applied_at,origin_node_id FROM sync_apply_log WHERE event_id=?", item.EventID).Scan(&item.SourceDatabase, &item.SourceTable, &item.TargetDatabase, &item.TargetTable, &applied, &origin)
			switch {
			case err == nil:
				item.ApplyStatus = "applied"
				decision, _, decisionErr := conflict.LookupDecision(ctx, s.DB, origin, item.EventID)
				if decisionErr != nil {
					item.ApplyStatus = "recorded_outcome_unknown"
					result.Warnings = append(result.Warnings, "conflict_outcome_query_failed: "+decisionErr.Error())
				} else if decision == conflict.Superseded {
					item.ApplyStatus = "superseded"
				}
				item.AppliedAt = applied.Format(time.RFC3339Nano)
				failedAt, _ := time.Parse(time.RFC3339Nano, item.LastErrorAt)
				if record.Error == "" || !applied.Before(failedAt) {
					item.State = item.ApplyStatus
					item.NextRetryAt = ""
				}
			case errors.Is(err, sql.ErrNoRows):
				item.ApplyStatus = "not_recorded"
			default:
				result.Warnings = append(result.Warnings, "apply_receipt_query_failed: "+err.Error())
			}
		} else {
			result.Warnings = append(result.Warnings, "apply_receipt_connection_unavailable")
		}
		if record.Message == "worker step recovered" && (item.ApplyStatus == "unknown" || item.ApplyStatus == "not_recorded") {
			item.State = "recovered_without_apply_receipt"
			item.NextRetryAt = ""
		}
		if s.Redact != nil {
			item.LastError = s.Redact(item.LastError)
		}
		result.Items = append(result.Items, item)
	}
	if s.Redact != nil {
		for i := range result.Warnings {
			result.Warnings[i] = s.Redact(result.Warnings[i])
		}
	}
	result.Warnings = append(result.Warnings, "Empty results do not prove queue health or data consistency; log history is bounded and receipts do not verify current business rows.")
	return result, nil
}

func IsRetry(item Observation) bool { return strings.HasPrefix(item.State, "retry") }
