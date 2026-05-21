package cdc

import (
	"context"
	"time"
)

type Operation string

const (
	OperationInsert Operation = "INSERT"
	OperationUpdate Operation = "UPDATE"
	OperationDelete Operation = "DELETE"
)

type ChangeEvent struct {
	DatabaseName string
	TableName    string
	Operation    Operation
	PrimaryKey   map[string]any
	Before       map[string]any
	After        map[string]any
	BinlogFile   string
	BinlogPos    uint32
	EventTime    time.Time
}

type Reader interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Events() <-chan ChangeEvent
	Errors() <-chan error
	SaveOffset(ctx context.Context) error
	LoadOffset(ctx context.Context) error
}
