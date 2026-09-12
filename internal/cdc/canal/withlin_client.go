package canal

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/rowvalue"
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
	Config           Config
	TimeoutMS        int64
	UnitMS           int32
	connector        WithlinConnector
	connectorFactory func() WithlinConnector
}

const (
	defaultFetchTimeoutMillis int64 = 100
	millisecondTimeUnit       int32 = 2
)

func NewWithlinClient(config Config) (*WithlinClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	host, port, err := splitAddress(config.Address)
	if err != nil {
		return nil, err
	}
	factory := func() WithlinConnector {
		return withlinclient.NewSimpleCanalConnector(host, port, config.Username, config.Password, config.Destination, 60000, int32(time.Hour/time.Millisecond))
	}
	return &WithlinClient{
		Config:           config,
		TimeoutMS:        defaultFetchTimeoutMillis,
		UnitMS:           millisecondTimeUnit,
		connector:        factory(),
		connectorFactory: factory,
	}, nil
}

func NewWithlinClientWithConnector(config Config, connector WithlinConnector) (*WithlinClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if connector == nil {
		return nil, fmt.Errorf("withlin connector is required")
	}
	return &WithlinClient{Config: config, TimeoutMS: defaultFetchTimeoutMillis, UnitMS: millisecondTimeUnit, connector: connector}, nil
}

func (c *WithlinClient) Connect(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.connector == nil && c.connectorFactory != nil {
		c.connector = c.connectorFactory()
	}
	if c.connector == nil {
		return fmt.Errorf("canal connector unavailable")
	}
	err := c.connector.Connect()
	if err != nil && c.connectorFactory != nil {
		_ = c.connector.DisConnection()
		c.connector = nil
	}
	return err
}

func (c *WithlinClient) Subscribe(ctx context.Context, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.connector == nil {
		return fmt.Errorf("canal connector unavailable")
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
	if c.connector == nil {
		return nil, cdc.Offset{}, fmt.Errorf("canal connector unavailable")
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
	if !offset.HasCanalBatch() {
		return nil
	}
	if c.connector == nil {
		return fmt.Errorf("canal connector unavailable")
	}
	return c.connector.Ack(offset.BatchID)
}

func IsBatchNotExistError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "batchid:") && strings.Contains(text, "is not exist")
}

func (c *WithlinClient) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.connector == nil {
		return nil
	}
	err := c.connector.DisConnection()
	// The upstream connector retains its closed socket; reconnect needs a new instance.
	if c.connectorFactory != nil {
		c.connector = nil
	}
	return err
}

func ConvertWithlinMessage(msg *withlinprotocol.Message) ([]RowChange, cdc.Offset, error) {
	if msg == nil {
		return nil, cdc.Offset{}, nil
	}
	rows := make([]RowChange, 0, len(msg.Entries))
	offset := cdc.Offset{BatchID: msg.Id, UpdatedAt: time.Now()}
	for i := range msg.Entries {
		item := &msg.Entries[i]
		header := item.GetHeader()
		if header == nil {
			continue
		}
		offset.BinlogFile = header.GetLogfileName()
		offset.BinlogPos = uint32(header.GetLogfileOffset())
		offset.GTID = header.GetGtid()
		if item.GetEntryType() != withlinentry.EntryType_ROWDATA {
			continue
		}
		rowChange := &withlinentry.RowChange{}
		if err := proto.Unmarshal(item.GetStoreValue(), rowChange); err != nil {
			return nil, cdc.Offset{}, fmt.Errorf("parse canal row change: %w", err)
		}
		if rowChange.GetIsDdl() {
			change, recognized, err := ParseAlterColumnSQL(header.GetSchemaName(), header.GetTableName(), rowChange.GetSql())
			if err != nil {
				return nil, cdc.Offset{}, fmt.Errorf("parse Canal column DDL: %w", err)
			}
			if recognized {
				operation := cdc.Operation(change.Operation)
				rows = append(rows, RowChange{
					DatabaseName: header.GetSchemaName(), TableName: header.GetTableName(), Operation: operation,
					SchemaChange: change, BinlogFile: header.GetLogfileName(), BinlogPos: uint32(header.GetLogfileOffset()),
					EventTime: time.UnixMilli(header.GetExecuteTime()),
				})
			}
			continue
		}
		operation, err := mapWithlinOperation(rowChange.GetEventType())
		if err != nil {
			continue
		}
		for _, rowData := range rowChange.GetRowDatas() {
			before, err := columnsToMap(rowData.GetBeforeColumns())
			if err != nil {
				return nil, cdc.Offset{}, err
			}
			after, err := columnsToMap(rowData.GetAfterColumns())
			if err != nil {
				return nil, cdc.Offset{}, err
			}
			primaryKey, err := primaryKeyFor(operation, rowData)
			if err != nil {
				return nil, cdc.Offset{}, err
			}
			rows = append(rows, RowChange{
				DatabaseName: header.GetSchemaName(),
				TableName:    header.GetTableName(),
				Operation:    operation,
				PrimaryKey:   primaryKey,
				Before:       before,
				After:        after,
				BinlogFile:   header.GetLogfileName(),
				BinlogPos:    uint32(header.GetLogfileOffset()),
				EventTime:    time.UnixMilli(header.GetExecuteTime()),
			})
		}
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

func primaryKeyFor(operation cdc.Operation, rowData *withlinentry.RowData) (map[string]any, error) {
	columns := rowData.GetAfterColumns()
	if operation == cdc.OperationDelete {
		columns = rowData.GetBeforeColumns()
	}
	result := make(map[string]any)
	for _, column := range columns {
		if column.GetIsKey() {
			value, err := columnValue(column)
			if err != nil {
				return nil, err
			}
			result[column.GetName()] = value
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func columnsToMap(columns []*withlinentry.Column) (map[string]any, error) {
	if len(columns) == 0 {
		return nil, nil
	}
	result := make(map[string]any, len(columns))
	for _, column := range columns {
		value, err := columnValue(column)
		if err != nil {
			return nil, err
		}
		result[column.GetName()] = value
	}
	return result, nil
}

func columnValue(column *withlinentry.Column) (any, error) {
	if column.GetIsNull() {
		return nil, nil
	}
	switch column.GetSqlType() {
	case -2, -3, -4, 2004: // JDBC BINARY, VARBINARY, LONGVARBINARY, BLOB.
		value, err := rowvalue.FromCanal(column.GetValue())
		if err != nil {
			return nil, fmt.Errorf("canal column %s: %w", column.GetName(), err)
		}
		return value, nil
	default:
		return column.GetValue(), nil
	}
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
