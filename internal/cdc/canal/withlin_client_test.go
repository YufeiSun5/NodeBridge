package canal

import (
	"context"
	"errors"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	withlinprotocol "github.com/withlin/canal-go/protocol"
	withlinentry "github.com/withlin/canal-go/protocol/entry"
	"google.golang.org/protobuf/proto"
)

func TestConvertWithlinMessage(t *testing.T) {
	msg := &withlinprotocol.Message{
		Id: 99,
		Entries: []withlinentry.Entry{
			withlinEntry(t, withlinentry.EventType_UPDATE),
		},
	}

	rows, offset, err := ConvertWithlinMessage(msg)
	if err != nil {
		t.Fatalf("ConvertWithlinMessage returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	row := rows[0]
	if row.Operation != cdc.OperationUpdate || row.DatabaseName != "scada_edge" || row.TableName != "device_config" {
		t.Fatalf("unexpected row %+v", row)
	}
	if row.PrimaryKey["id"] != "1" || row.After["value"] != "ON" || row.Before["value"] != "OFF" {
		t.Fatalf("unexpected maps pk=%+v before=%+v after=%+v", row.PrimaryKey, row.Before, row.After)
	}
	if offset.BatchID != 99 || offset.BinlogFile != "mysql-bin.000001" || offset.BinlogPos != 128 || offset.GTID != "gtid-001" {
		t.Fatalf("unexpected offset %+v", offset)
	}
}

func TestConvertWithlinMessageKeepsOffsetForNonRowBatch(t *testing.T) {
	msg := &withlinprotocol.Message{
		Id: 101,
		Entries: []withlinentry.Entry{
			{
				Header: &withlinentry.Header{
					LogfileName:   "mysql-bin.000002",
					LogfileOffset: 2048,
					Gtid:          "gtid-002",
				},
				EntryTypePresent: &withlinentry.Entry_EntryType{EntryType: withlinentry.EntryType_TRANSACTIONBEGIN},
			},
		},
	}

	rows, offset, err := ConvertWithlinMessage(msg)
	if err != nil {
		t.Fatalf("ConvertWithlinMessage returned error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no rows, got %+v", rows)
	}
	if offset.BatchID != 101 || offset.BinlogFile != "mysql-bin.000002" || offset.BinlogPos != 2048 || offset.GTID != "gtid-002" {
		t.Fatalf("unexpected offset %+v", offset)
	}
}

func TestConvertWithlinMessageIncludesSupportedColumnDDL(t *testing.T) {
	msg := &withlinprotocol.Message{Id: 102, Entries: []withlinentry.Entry{withlinDDLEntry(t, "ALTER TABLE device_config ADD COLUMN governed_note VARCHAR(64) NULL")}}
	rows, offset, err := ConvertWithlinMessage(msg)
	if err != nil {
		t.Fatalf("ConvertWithlinMessage returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Operation != cdc.OperationAddColumn || rows[0].SchemaChange == nil || rows[0].SchemaChange.Column.Name != "governed_note" {
		t.Fatalf("unexpected DDL rows %+v", rows)
	}
	if offset.BatchID != 102 {
		t.Fatalf("unexpected offset %+v", offset)
	}
}

func TestConvertWithlinMessageStillSkipsCreateTable(t *testing.T) {
	msg := &withlinprotocol.Message{Id: 103, Entries: []withlinentry.Entry{withlinDDLEntry(t, "CREATE TABLE ignored (id BIGINT)")}}
	rows, _, err := ConvertWithlinMessage(msg)
	if err != nil || len(rows) != 0 {
		t.Fatalf("CREATE TABLE must remain unsupported, rows=%+v err=%v", rows, err)
	}
}

func TestWithlinClientAckSkipsNonPositiveBatchID(t *testing.T) {
	connector := &fakeWithlinConnector{}
	client, err := NewWithlinClientWithConnector(Config{
		ReaderName:  "edge-001",
		Address:     "127.0.0.1:11111",
		Destination: "edge-001",
	}, connector)
	if err != nil {
		t.Fatalf("NewWithlinClientWithConnector returned error: %v", err)
	}
	if err := client.Ack(context.Background(), cdc.Offset{BatchID: -1}); err != nil {
		t.Fatalf("Ack returned error: %v", err)
	}
	if connector.acked != 0 {
		t.Fatalf("unexpected acked batch %d", connector.acked)
	}
}

func TestWithlinClientLifecycleFetchAndAck(t *testing.T) {
	connector := &fakeWithlinConnector{
		msg: &withlinprotocol.Message{
			Id:      100,
			Entries: []withlinentry.Entry{withlinEntry(t, withlinentry.EventType_INSERT)},
		},
	}
	client, err := NewWithlinClientWithConnector(Config{
		ReaderName:  "edge-001",
		Address:     "127.0.0.1:11111",
		Destination: "edge-001",
		Filter:      "scada_edge\\..*",
	}, connector)
	if err != nil {
		t.Fatalf("NewWithlinClientWithConnector returned error: %v", err)
	}
	if client.TimeoutMS != 100 || client.UnitMS != 2 {
		t.Fatalf("expected 100ms low-latency fetch, got timeout=%d unit=%d", client.TimeoutMS, client.UnitMS)
	}

	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	if err := client.Subscribe(context.Background(), "edge-001"); err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	rows, offset, err := client.Fetch(context.Background(), 10)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(rows) != 1 || offset.BatchID != 100 {
		t.Fatalf("unexpected rows=%+v offset=%+v", rows, offset)
	}
	if connector.timeout != 100 || connector.unit != 2 {
		t.Fatalf("expected connector 100ms timeout, got timeout=%d unit=%d", connector.timeout, connector.unit)
	}
	if err := client.Ack(context.Background(), offset); err != nil {
		t.Fatalf("Ack returned error: %v", err)
	}
	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if !connector.connected || connector.filter != "scada_edge\\..*" || connector.acked != 100 || !connector.closed {
		t.Fatalf("unexpected connector state %+v", connector)
	}
}

func TestSplitAddress(t *testing.T) {
	host, port, err := splitAddress("127.0.0.1:11111")
	if err != nil {
		t.Fatalf("splitAddress returned error: %v", err)
	}
	if host != "127.0.0.1" || port != 11111 {
		t.Fatalf("unexpected host=%s port=%d", host, port)
	}
	if _, _, err := splitAddress("127.0.0.1"); err == nil {
		t.Fatal("expected invalid address error")
	}
}

func TestIsBatchNotExistError(t *testing.T) {
	if !IsBatchNotExistError(errors.New("batchId:4 is not exist")) {
		t.Fatal("expected batch missing error to match")
	}
	if IsBatchNotExistError(errors.New("permission denied")) {
		t.Fatal("unexpected match")
	}
}

type fakeWithlinConnector struct {
	msg       *withlinprotocol.Message
	connected bool
	filter    string
	acked     int64
	closed    bool
	timeout   int64
	unit      int32
}

func TestWithlinClientRecreatesClosedConnector(t *testing.T) {
	var created []*fakeWithlinConnector
	c := &WithlinClient{connectorFactory: func() WithlinConnector {
		connector := &fakeWithlinConnector{}
		created = append(created, connector)
		return connector
	}}
	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || !created[0].closed || c.connector != nil {
		t.Fatal("closed connector retained")
	}
	if err := c.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || !created[1].connected || created[1].closed {
		t.Fatal("reconnect reused closed socket")
	}
	if err := c.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func (c *fakeWithlinConnector) Connect() error {
	c.connected = true
	return nil
}

func (c *fakeWithlinConnector) DisConnection() error {
	c.closed = true
	return nil
}

func (c *fakeWithlinConnector) Subscribe(filter string) error {
	c.filter = filter
	return nil
}

func (c *fakeWithlinConnector) GetWithOutAck(batchSize int32, timeOut *int64, units *int32) (*withlinprotocol.Message, error) {
	c.timeout = *timeOut
	c.unit = *units
	return c.msg, nil
}

func (c *fakeWithlinConnector) Ack(batchId int64) error {
	c.acked = batchId
	return nil
}

func withlinEntry(t *testing.T, eventType withlinentry.EventType) withlinentry.Entry {
	t.Helper()
	rowChange := &withlinentry.RowChange{
		EventTypePresent: &withlinentry.RowChange_EventType{EventType: eventType},
		RowDatas: []*withlinentry.RowData{
			{
				BeforeColumns: []*withlinentry.Column{
					{Name: "id", IsKey: true, Value: "1"},
					{Name: "name", Value: "Pump A"},
					{Name: "value", Value: "OFF"},
				},
				AfterColumns: []*withlinentry.Column{
					{Name: "id", IsKey: true, Value: "1"},
					{Name: "name", Value: "Pump A"},
					{Name: "value", Value: "ON"},
				},
			},
		},
	}
	body, err := proto.Marshal(rowChange)
	if err != nil {
		t.Fatalf("marshal row change: %v", err)
	}
	return withlinentry.Entry{
		Header: &withlinentry.Header{
			LogfileName:   "mysql-bin.000001",
			LogfileOffset: 128,
			ExecuteTime:   1779350400000,
			SchemaName:    "scada_edge",
			TableName:     "device_config",
			Gtid:          "gtid-001",
		},
		EntryTypePresent: &withlinentry.Entry_EntryType{EntryType: withlinentry.EntryType_ROWDATA},
		StoreValue:       body,
	}
}

func withlinDDLEntry(t *testing.T, sql string) withlinentry.Entry {
	t.Helper()
	rowChange := &withlinentry.RowChange{
		EventTypePresent: &withlinentry.RowChange_EventType{EventType: withlinentry.EventType_ALTER},
		IsDdlPresent:     &withlinentry.RowChange_IsDdl{IsDdl: true},
		Sql:              sql,
	}
	body, err := proto.Marshal(rowChange)
	if err != nil {
		t.Fatalf("marshal DDL row change: %v", err)
	}
	return withlinentry.Entry{
		Header:           &withlinentry.Header{LogfileName: "mysql-bin.000010", LogfileOffset: 512, ExecuteTime: 1779350400000, SchemaName: "scada_edge", TableName: "device_config"},
		EntryTypePresent: &withlinentry.Entry_EntryType{EntryType: withlinentry.EntryType_ROWDATA}, StoreValue: body,
	}
}
