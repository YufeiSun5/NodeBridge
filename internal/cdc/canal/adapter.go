package canal

import (
	"context"
	"fmt"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
)

type Config struct {
	ReaderName  string
	Address     string
	Destination string
	BatchSize   int
}

func (c Config) Validate() error {
	if c.ReaderName == "" {
		return fmt.Errorf("reader name is required")
	}
	if c.Address == "" {
		return fmt.Errorf("canal address is required")
	}
	if c.Destination == "" {
		return fmt.Errorf("canal destination is required")
	}
	if c.BatchSize < 0 {
		return fmt.Errorf("canal batch size cannot be negative")
	}
	return nil
}

type RowChange struct {
	DatabaseName string
	TableName    string
	Operation    cdc.Operation
	PrimaryKey   map[string]any
	Before       map[string]any
	After        map[string]any
	BinlogFile   string
	BinlogPos    uint32
	EventTime    time.Time
}

func ConvertRowChange(row RowChange) (cdc.ChangeEvent, error) {
	if row.DatabaseName == "" || row.TableName == "" {
		return cdc.ChangeEvent{}, fmt.Errorf("database and table are required")
	}
	switch row.Operation {
	case cdc.OperationInsert, cdc.OperationUpdate, cdc.OperationDelete:
	default:
		return cdc.ChangeEvent{}, fmt.Errorf("unsupported canal operation %q", row.Operation)
	}
	return cdc.ChangeEvent{
		DatabaseName: row.DatabaseName,
		TableName:    row.TableName,
		Operation:    row.Operation,
		PrimaryKey:   clone(row.PrimaryKey),
		Before:       clone(row.Before),
		After:        clone(row.After),
		BinlogFile:   row.BinlogFile,
		BinlogPos:    row.BinlogPos,
		EventTime:    row.EventTime,
	}, nil
}

type Client interface {
	Connect(ctx context.Context) error
	Subscribe(ctx context.Context, destination string) error
	Fetch(ctx context.Context, batchSize int) ([]RowChange, cdc.Offset, error)
	Ack(ctx context.Context, offset cdc.Offset) error
	Close(ctx context.Context) error
}

type Adapter struct {
	Config Config
	Client Client
	Store  cdc.OffsetStore
	events chan cdc.ChangeEvent
	errors chan error
}

func NewAdapter(config Config, client Client, store cdc.OffsetStore) (*Adapter, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("canal client is required")
	}
	if store == nil {
		store = cdc.NewMemoryOffsetStore()
	}
	return &Adapter{
		Config: config,
		Client: client,
		Store:  store,
		events: make(chan cdc.ChangeEvent, defaultBatchSize(config.BatchSize)),
		errors: make(chan error, 1),
	}, nil
}

func (a *Adapter) Start(ctx context.Context) error {
	if err := a.Client.Connect(ctx); err != nil {
		return err
	}
	return a.Client.Subscribe(ctx, a.Config.Destination)
}

func (a *Adapter) Stop(ctx context.Context) error {
	return a.Client.Close(ctx)
}

func (a *Adapter) Events() <-chan cdc.ChangeEvent {
	return a.events
}

func (a *Adapter) Errors() <-chan error {
	return a.errors
}

func (a *Adapter) SaveOffset(ctx context.Context) error {
	return nil
}

func (a *Adapter) LoadOffset(ctx context.Context) error {
	_, _, err := a.Store.Load(ctx, a.Config.ReaderName)
	return err
}

func (a *Adapter) FetchOnce(ctx context.Context) (int, error) {
	rows, offset, err := a.Client.Fetch(ctx, defaultBatchSize(a.Config.BatchSize))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		change, err := ConvertRowChange(row)
		if err != nil {
			return count, err
		}
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		case a.events <- change:
			count++
		}
	}
	if count > 0 {
		offset.ReaderName = a.Config.ReaderName
		if err := a.Store.Save(ctx, offset); err != nil {
			return count, err
		}
		if err := a.Client.Ack(ctx, offset); err != nil {
			return count, err
		}
	}
	return count, nil
}

func defaultBatchSize(value int) int {
	if value > 0 {
		return value
	}
	return 1000
}

func clone(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
