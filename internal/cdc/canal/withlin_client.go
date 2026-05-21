package canal

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	withlinclient "github.com/withlin/canal-go/client"
	withlinprotocol "github.com/withlin/canal-go/protocol"
	withlinentry "github.com/withlin/canal-go/protocol/entry"
	"google.golang.org/protobuf/proto"
)

type WithlinConnector interface {
	Connect() error
	DisConnection() error
	Subscribe(filter string) error
	GetWithOutAck(batchSize int32, timeOut *int64, units *int32) (*withlinprotocol.Message, error)
	Ack(batchId int64) error
}

type WithlinClient struct {
	Config    Config
	TimeoutMS int64
	UnitMS    int32
	connector WithlinConnector
}

func NewWithlinClient(config Config) (*WithlinClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	host, port, err := splitAddress(config.Address)
	if err != nil {
		return nil, err
	}
	connector := withlinclient.NewSimpleCanalConnector(host, port, config.Username, config.Password, config.Destination, 60000, 60000)
	return &WithlinClient{
		Config:    config,
		TimeoutMS: 1000,
		UnitMS:    2,
		connector: connector,
	}, nil
}

func NewWithlinClientWithConnector(config Config, connector WithlinConnector) (*WithlinClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if connector == nil {
		return nil, fmt.Errorf("withlin connector is required")
	}
	return &WithlinClient{Config: config, TimeoutMS: 1000, UnitMS: 2, connector: connector}, nil
}

func (c *WithlinClient) Connect(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.connector.Connect()
}

func (c *WithlinClient) Subscribe(ctx context.Context, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	filter := c.Config.Filter
	if filter == "" {
		filter = ".*\\..*"
	}
	return c.connector.Subscribe(filter)
}

func (c *WithlinClient) Fetch(ctx context.Context, batchSize int) ([]RowChange, cdc.Offset, error) {
	if err := ctx.Err(); err != nil {
		return nil, cdc.Offset{}, err
	}
	timeout := c.TimeoutMS
	unit := c.UnitMS
	msg, err := c.connector.GetWithOutAck(int32(defaultBatchSize(batchSize)), &timeout, &unit)
	if err != nil {
		return nil, cdc.Offset{}, fmt.Errorf("fetch canal message: %w", err)
	}
	rows, offset, err := ConvertWithlinMessage(msg)
	if err != nil {
		return nil, cdc.Offset{}, err
	}
	return rows, offset, nil
}

func (c *WithlinClient) Ack(ctx context.Context, offset cdc.Offset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if offset.BatchID == 0 {
		return nil
	}
	return c.connector.Ack(offset.BatchID)
}

func (c *WithlinClient) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.connector.DisConnection()
}

func ConvertWithlinMessage(msg *withlinprotocol.Message) ([]RowChange, cdc.Offset, error) {
	if msg == nil {
		return nil, cdc.Offset{}, nil
	}
	rows := make([]RowChange, 0, len(msg.Entries))
	offset := cdc.Offset{BatchID: msg.Id, UpdatedAt: time.Now()}
	for i := range msg.Entries {
		item := &msg.Entries[i]
		if item.GetEntryType() != withlinentry.EntryType_ROWDATA {
			continue
		}
		header := item.GetHeader()
		if header == nil {
			continue
		}
		rowChange := &withlinentry.RowChange{}
		if err := proto.Unmarshal(item.GetStoreValue(), rowChange); err != nil {
			return nil, cdc.Offset{}, fmt.Errorf("parse canal row change: %w", err)
		}
		if rowChange.GetIsDdl() {
			continue
		}
		operation, err := mapWithlinOperation(rowChange.GetEventType())
		if err != nil {
			continue
		}
		for _, rowData := range rowChange.GetRowDatas() {
			rows = append(rows, RowChange{
				DatabaseName: header.GetSchemaName(),
				TableName:    header.GetTableName(),
				Operation:    operation,
				PrimaryKey:   primaryKeyFor(operation, rowData),
				Before:       columnsToMap(rowData.GetBeforeColumns()),
				After:        columnsToMap(rowData.GetAfterColumns()),
				BinlogFile:   header.GetLogfileName(),
				BinlogPos:    uint32(header.GetLogfileOffset()),
				EventTime:    time.UnixMilli(header.GetExecuteTime()),
			})
		}
		offset.BinlogFile = header.GetLogfileName()
		offset.BinlogPos = uint32(header.GetLogfileOffset())
		offset.GTID = header.GetGtid()
	}
	return rows, offset, nil
}

func mapWithlinOperation(eventType withlinentry.EventType) (cdc.Operation, error) {
	switch eventType {
	case withlinentry.EventType_INSERT:
		return cdc.OperationInsert, nil
	case withlinentry.EventType_UPDATE:
		return cdc.OperationUpdate, nil
	case withlinentry.EventType_DELETE:
		return cdc.OperationDelete, nil
	default:
		return "", fmt.Errorf("unsupported canal event type %s", eventType.String())
	}
}

func primaryKeyFor(operation cdc.Operation, rowData *withlinentry.RowData) map[string]any {
	columns := rowData.GetAfterColumns()
	if operation == cdc.OperationDelete {
		columns = rowData.GetBeforeColumns()
	}
	result := make(map[string]any)
	for _, column := range columns {
		if column.GetIsKey() {
			result[column.GetName()] = columnValue(column)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func columnsToMap(columns []*withlinentry.Column) map[string]any {
	if len(columns) == 0 {
		return nil
	}
	result := make(map[string]any, len(columns))
	for _, column := range columns {
		result[column.GetName()] = columnValue(column)
	}
	return result
}

func columnValue(column *withlinentry.Column) any {
	if column.GetIsNull() {
		return nil
	}
	return column.GetValue()
}

func splitAddress(address string) (string, int, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("parse canal address %q: %w", address, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, fmt.Errorf("parse canal port %q: %w", portText, err)
	}
	return host, port, nil
}
