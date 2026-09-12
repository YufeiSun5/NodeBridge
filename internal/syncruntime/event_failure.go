package syncruntime

import (
	"encoding/json"
	"errors"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

type EventFailure struct {
	Cause          error
	EventID        string
	RuleID         string
	SourceDatabase string
	SourceTable    string
	TargetDatabase string
	TargetTable    string
	BinlogFile     string
	BinlogPos      uint32
	GTID           string
}

func (e *EventFailure) Error() string { return e.Cause.Error() }
func (e *EventFailure) Unwrap() error { return e.Cause }

func describeEventFailure(err error, evt event.SyncEvent, rule *rules.SyncRule, override string) error {
	var previous *EventFailure
	if errors.As(err, &previous) {
		return err
	}
	detail := &EventFailure{Cause: err, EventID: evt.EventID, SourceDatabase: evt.DatabaseName, SourceTable: evt.TableName, TargetDatabase: evt.DatabaseName, TargetTable: evt.TableName, BinlogFile: evt.BinlogFile, BinlogPos: evt.BinlogPos, GTID: evt.GTID}
	if rule != nil {
		detail.RuleID = rule.ID
		if rule.TargetDatabaseName != "" {
			detail.TargetDatabase = rule.TargetDatabaseName
		}
		if rule.TargetTableName != "" {
			detail.TargetTable = rule.TargetTableName
		}
	}
	if override != "" && rule != nil {
		detail.TargetDatabase = rule.DownlinkTargetDatabase(override)
	}
	return detail
}

func messageFailure(err error, id string, messages []rabbitmq.IncomingMessage, set *rules.RuleSet, override string) error {
	for _, message := range messages {
		var evt event.SyncEvent
		if json.Unmarshal(cleanJSONBody(message.Body()), &evt) != nil {
			continue
		}
		if evt.EventID == id {
			return describeEventFailure(err, evt, findRuleForEvent(set, evt), override)
		}
	}
	return err
}
