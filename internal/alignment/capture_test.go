package alignment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/capture"
	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/YufeiSun5/NodeBridge/internal/cdc/canal"
)

func boundaryFixture() SnapshotBoundary {
	return SnapshotBoundary{OriginNodeID: "local", DatabaseName: "business", TableName: "items", MySQLServerUUID: "12345678-1234-1234-1234-123456789abc", BinlogFile: "mysql-bin.000010", BinlogPos: 200, Token: strings.Repeat("a", 64)}
}

func TestSnapshotBoundaryIsExclusiveAndChecksLineage(t *testing.T) {
	b := boundaryFixture()
	for _, tc := range []struct {
		file   string
		pos    uint32
		before bool
	}{
		{"mysql-bin.000009", 999, true}, {"mysql-bin.000010", 199, true}, {"mysql-bin.000010", 200, false}, {"mysql-bin.000010", 201, false}, {"mysql-bin.000011", 4, false},
	} {
		got, err := b.BeforePosition(tc.file, tc.pos)
		if err != nil || got != tc.before {
			t.Fatalf("%+v: %v %v", tc, got, err)
		}
	}
	for _, file := range []string{"other.000010", "mysql-bin", "mysql-bin.000000", "mysql-bin.18446744073709551616"} {
		if _, err := b.BeforePosition(file, 4); err == nil {
			t.Fatal("incomparable position accepted", file)
		}
	}
	if _, err := b.BeforePosition(b.BinlogFile, 0); err == nil {
		t.Fatal("missing position accepted")
	}
}

type probeClient struct {
	rows          []canal.RowChange
	offset        cdc.Offset
	err           error
	closed, acked int
}

func (*probeClient) Connect(context.Context) error           { return nil }
func (*probeClient) Subscribe(context.Context, string) error { return nil }
func (p *probeClient) Fetch(context.Context, int) ([]canal.RowChange, cdc.Offset, error) {
	return p.rows, p.offset, p.err
}
func (p *probeClient) Close(context.Context) error           { p.closed++; return nil }
func (p *probeClient) Ack(context.Context, cdc.Offset) error { p.acked++; return nil }

func TestSnapshotProbeNeverAcknowledgesReads(t *testing.T) {
	b := boundaryFixture()
	client := &probeClient{offset: cdc.Offset{BatchID: 1}, rows: []canal.RowChange{
		{DatabaseName: "business", TableName: "items", Operation: cdc.OperationInsert, After: map[string]any{"id": "1"}},
		{DatabaseName: "control", TableName: capture.Table, Operation: cdc.OperationUpdate, BinlogFile: b.BinlogFile, BinlogPos: b.BinlogPos, After: map[string]any{"node_id": b.OriginNodeID, "token": b.Token}},
	}}
	p := &SnapshotCapture{client: client, control: "control", base: b, markers: map[string]SnapshotBoundary{}}
	if err := p.fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := p.WaitMarker(context.Background(), b.Token)
	if err != nil || got != b {
		t.Fatal(got, err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if client.acked != 0 || client.closed != 1 {
		t.Fatal("probe acknowledged or closed twice", client)
	}
	if _, err := p.WaitMarker(context.Background(), b.Token); err == nil {
		t.Fatal("closed probe used")
	}
}

func TestSnapshotProbeRejectsMissingBatchAndReusedMarker(t *testing.T) {
	b := boundaryFixture()
	row := canal.RowChange{DatabaseName: "control", TableName: capture.Table, Operation: cdc.OperationInsert, BinlogFile: b.BinlogFile, BinlogPos: b.BinlogPos, After: map[string]any{"node_id": b.OriginNodeID, "token": b.Token}}
	client := &probeClient{rows: []canal.RowChange{row}}
	p := &SnapshotCapture{client: client, control: "control", base: b, markers: map[string]SnapshotBoundary{}}
	if err := p.fetch(context.Background()); err == nil {
		t.Fatal("missing batch accepted")
	}
	client.offset.BatchID = 1
	if err := p.fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	client.rows[0].BinlogPos++
	if err := p.fetch(context.Background()); err == nil {
		t.Fatal("same token at another position accepted")
	}
	client.err = errors.New("fetch failed")
	if err := p.fetch(context.Background()); err == nil {
		t.Fatal("fetch failure ignored")
	}
	if client.acked != 0 {
		t.Fatal("failed probe acknowledged input")
	}
}

type canceledProbeClient struct {
	probeClient
	cancel context.CancelFunc
}

func (p *canceledProbeClient) Fetch(ctx context.Context, size int) ([]canal.RowChange, cdc.Offset, error) {
	p.cancel()
	return p.probeClient.Fetch(ctx, size)
}

func TestSnapshotProbeRejectsMarkerReturnedAfterCancellation(t *testing.T) {
	b := boundaryFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &canceledProbeClient{cancel: cancel, probeClient: probeClient{offset: cdc.Offset{BatchID: 1}, rows: []canal.RowChange{
		{DatabaseName: "control", TableName: capture.Table, Operation: cdc.OperationUpdate, BinlogFile: b.BinlogFile, BinlogPos: b.BinlogPos, After: map[string]any{"node_id": b.OriginNodeID, "token": b.Token}},
	}}}
	p := &SnapshotCapture{client: client, control: "control", base: b, markers: map[string]SnapshotBoundary{}}
	if err := p.fetch(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("late marker accepted", err)
	}
	if len(p.markers) != 0 || client.acked != 0 {
		t.Fatal("canceled probe changed capture progress")
	}
}
