package cdc

import (
	"context"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/dbgovernance"
)

type Operation string

const (
	OperationInsert     Operation = "INSERT"
	OperationUpdate     Operation = "UPDATE"
	OperationDelete     Operation = "DELETE"
	OperationAddColumn  Operation = "ADD_COLUMN"
	OperationDropColumn Operation = "DROP_COLUMN"
)

type ChangeEvent struct {
	DatabaseName string                     `json:"database_name"`
	TableName    string                     `json:"table_name"`
	Operation    Operation                  `json:"operation"`
	PrimaryKey   map[string]any             `json:"primary_key"`
	Before       map[string]any             `json:"before,omitempty"`
	After        map[string]any             `json:"after,omitempty"`
	SchemaChange *dbgovernance.SchemaChange `json:"schema_change,omitempty"`
	BinlogFile   string                     `json:"binlog_file,omitempty"`
	BinlogPos    uint32                     `json:"binlog_pos,omitempty"`
	EventTime    time.Time                  `json:"event_time"`
}

type Reader interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Events() <-chan ChangeEvent
	Errors() <-chan error
	SaveOffset(ctx context.Context) error
	LoadOffset(ctx context.Context) error
}
