package canal

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	"github.com/withlin/canal-go/protocol/packet"
	"google.golang.org/protobuf/proto"
)

func TestSocketConnectorHandshakeDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	client, err := NewWithlinClient(Config{ReaderName: "deadline", Address: listener.Addr().String(), Destination: "deadline"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = client.Connect(ctx)
	if err == nil || time.Since(started) > 2*time.Second || client.connector != nil {
		t.Fatalf("handshake was not interrupted: %v", err)
	}
	select {
	case conn := <-accepted:
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		var b [1]byte
		if _, err := conn.Read(b[:]); !errors.Is(err, io.EOF) {
			t.Fatalf("client socket not closed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("connection not accepted")
	}
}

func TestSocketConnectorCanceledReadAndWrite(t *testing.T) {
	for _, operation := range []string{"fetch", "ack", "close"} {
		t.Run(operation, func(t *testing.T) {
			local, remote := net.Pipe()
			defer local.Close()
			defer remote.Close()
			entered := make(chan struct{})
			connector := &socketConnector{conn: &writeSignalConn{Conn: local, entered: entered}, config: Config{Destination: "test"}}
			client := &WithlinClient{connector: connector}
			ctx, cancel := context.WithCancel(context.Background())
			finished := make(chan error, 1)
			go func() {
				var err error
				switch operation {
				case "fetch":
					_, _, err = client.Fetch(ctx, 1)
				case "ack":
					err = client.Ack(ctx, cdc.Offset{BatchID: 1})
				case "close":
					err = client.Close(ctx)
				}
				finished <- err
			}()
			if operation == "fetch" {
				if _, err := readTestPacket(remote); err != nil {
					t.Fatal(err)
				}
			}
			<-entered
			cancel()
			select {
			case err := <-finished:
				if !errors.Is(err, context.Canceled) || connector.conn != nil {
					t.Fatalf("cancel failed: %v, socket=%v", err, connector.conn)
				}
			case <-time.After(time.Second):
				t.Fatal("network operation ignored cancellation")
			}
		})
	}
}

type writeSignalConn struct {
	net.Conn
	entered chan struct{}
	once    sync.Once
}

func (c *writeSignalConn) Write(data []byte) (int, error) {
	c.once.Do(func() { close(c.entered) })
	return c.Conn.Write(data)
}

func readTestPacket(conn net.Conn) (*packet.Packet, error) {
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return nil, err
	}
	data := make([]byte, binary.BigEndian.Uint32(header[:]))
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}
	p := &packet.Packet{}
	return p, proto.Unmarshal(data, p)
}

func TestSocketConnectorFetchFrames(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind packet.PacketType
		body proto.Message
		want string
	}{
		{"messages", packet.PacketType_MESSAGES, &packet.Messages{BatchId: 12}, ""},
		{"server_error", packet.PacketType_ACK, &packet.Ack{ErrorCodePresent: &packet.Ack_ErrorCode{ErrorCode: 1}, ErrorMessage: "batchId:12 is not exist"}, "batchId:12"},
		{"wrong_type", packet.PacketType_HANDSHAKE, &packet.Handshake{}, "unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local, remote := net.Pipe()
			defer local.Close()
			defer remote.Close()
			connector := &socketConnector{conn: local, config: Config{Destination: "test"}}
			body, _ := proto.Marshal(tc.body)
			data, _ := proto.Marshal(&packet.Packet{Type: tc.kind, Body: body})
			frame := make([]byte, 4+len(data))
			binary.BigEndian.PutUint32(frame[:4], uint32(len(data)))
			copy(frame[4:], data)
			finished := make(chan error, 1)
			go func() {
				request, err := readTestPacket(remote)
				if err == nil && request.GetType() != packet.PacketType_GET {
					err = errors.New("expected GET")
				}
				if err == nil {
					for _, b := range frame {
						if _, err = remote.Write([]byte{b}); err != nil {
							break
						}
					}
				}
				finished <- err
			}()
			message, err := connector.GetWithOutAck(1, nil, nil)
			if tc.want == "" {
				if err != nil || message == nil || message.Id != 12 {
					t.Fatalf("invalid result: %+v %v", message, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) || connector.conn != nil {
				t.Fatalf("expected %q and closed connection: %v", tc.want, err)
			}
			if err := <-finished; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSocketConnectorRejectsOversizedFrame(t *testing.T) {
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	connector := &socketConnector{conn: local}
	finished := make(chan error, 1)
	go func() {
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], maxCanalPacket+1)
		_, err := remote.Write(header[:])
		finished <- err
	}()
	if err := connector.network(func() error { _, err := connector.read(); return err }); err == nil || connector.conn != nil {
		t.Fatalf("oversized packet accepted: %v", err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
