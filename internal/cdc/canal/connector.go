package canal

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	withlinclient "github.com/withlin/canal-go/client"
	"github.com/withlin/canal-go/protocol"
	"github.com/withlin/canal-go/protocol/packet"
	"google.golang.org/protobuf/proto"
)

const canalNetworkTimeout = 10 * time.Second
const maxCanalPacket = 64 << 20

// socketConnector keeps the upstream wire types and decoder, but owns the socket
// so cancellation interrupts actual I/O. Like WithlinClient it is single-consumer.
type socketConnector struct {
	config Config
	conn   net.Conn
	ctx    context.Context
}

func (c *socketConnector) withContext(ctx context.Context, call func() error) error {
	ctx, cancel := context.WithTimeout(ctx, canalNetworkTimeout)
	defer cancel()
	c.ctx = ctx
	defer func() { c.ctx = nil }()
	err := call()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func runConnector(ctx context.Context, connector WithlinConnector, call func() error) error {
	if c, ok := connector.(*socketConnector); ok {
		return c.withContext(ctx, call)
	}
	return call()
}

func (c *socketConnector) network(call func() error) error {
	if c.conn == nil {
		return errors.New("canal connector unavailable")
	}
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, canalNetworkTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	conn := c.conn
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
		close(done)
	})
	err := call()
	if !stop() {
		<-done
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		_ = conn.Close()
		c.conn = nil
	} else {
		_ = conn.SetDeadline(time.Time{})
	}
	return err
}

func (c *socketConnector) Connect() error {
	if c.conn != nil {
		return nil
	}
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	conn, err := (&net.Dialer{Timeout: canalNetworkTimeout}).DialContext(ctx, "tcp", c.config.Address)
	if err != nil {
		return err
	}
	c.conn = conn
	return c.network(func() error {
		data, err := c.read()
		if err != nil {
			return err
		}
		p := &packet.Packet{}
		if err := proto.Unmarshal(data, p); err != nil {
			return err
		}
		if p.GetVersion() != 1 || p.GetType() != packet.PacketType_HANDSHAKE {
			return errors.New("canal handshake invalid")
		}
		handshake := &packet.Handshake{}
		if err := proto.Unmarshal(p.GetBody(), handshake); err != nil {
			return err
		}
		password := []byte(c.config.Password)
		auth := &packet.ClientAuth{
			Username:               c.config.Username,
			Password:               []byte(withlinclient.ByteSliceToHexString(withlinclient.Scramble411(&password, &handshake.Seeds))),
			NetReadTimeoutPresent:  &packet.ClientAuth_NetReadTimeout{NetReadTimeout: 3600000},
			NetWriteTimeoutPresent: &packet.ClientAuth_NetWriteTimeout{NetWriteTimeout: 3600000},
		}
		if err := c.write(packet.PacketType_CLIENTAUTHENTICATION, auth); err != nil {
			return err
		}
		if err := c.readAck(); err != nil {
			return err
		}
		return c.rollback()
	})
}

func (c *socketConnector) Subscribe(filter string) error {
	return c.network(func() error {
		if err := c.write(packet.PacketType_SUBSCRIPTION, &packet.Sub{Destination: c.config.Destination, ClientId: "1001", Filter: filter}); err != nil {
			return err
		}
		return c.readAck()
	})
}

func (c *socketConnector) GetWithOutAck(batchSize int32, timeout *int64, units *int32) (*protocol.Message, error) {
	var message *protocol.Message
	err := c.network(func() error {
		request := &packet.Get{Destination: c.config.Destination, ClientId: "1001", FetchSize: batchSize, AutoAckPresent: &packet.Get_AutoAck{AutoAck: false}}
		if timeout != nil {
			request.TimeoutPresent = &packet.Get_Timeout{Timeout: *timeout}
		}
		if units != nil {
			request.UnitPresent = &packet.Get_Unit{Unit: *units}
		}
		if err := c.write(packet.PacketType_GET, request); err != nil {
			return err
		}
		data, err := c.read()
		if err != nil {
			return err
		}
		p := &packet.Packet{}
		if err := proto.Unmarshal(data, p); err != nil {
			return err
		}
		// The upstream decoder panics for ACK errors and unsupported compression.
		if p.GetType() == packet.PacketType_ACK {
			ack := &packet.Ack{}
			if err := proto.Unmarshal(p.GetBody(), ack); err != nil {
				return err
			}
			return fmt.Errorf("canal fetch error %d: %s", ack.GetErrorCode(), ack.GetErrorMessage())
		}
		if p.GetType() != packet.PacketType_MESSAGES || (p.GetCompression() != packet.Compression_NONE && p.GetCompression() != packet.Compression_COMPRESSIONCOMPATIBLEPROTO2) {
			return errors.New("canal message type or compression unsupported")
		}
		message, err = protocol.Decode(data, false)
		return err
	})
	return message, err
}

func (c *socketConnector) Ack(batchID int64) error {
	return c.network(func() error {
		return c.write(packet.PacketType_CLIENTACK, &packet.ClientAck{Destination: c.config.Destination, ClientId: "1001", BatchId: batchID})
	})
}

func (c *socketConnector) rollback() error {
	return c.write(packet.PacketType_CLIENTROLLBACK, &packet.ClientRollback{Destination: c.config.Destination, ClientId: "1001", BatchId: 0})
}

func (c *socketConnector) DisConnection() error {
	if c.conn == nil {
		return nil
	}
	conn := c.conn
	err := c.network(c.rollback)
	closeErr := conn.Close()
	c.conn = nil
	if err != nil {
		return err
	}
	return closeErr
}

func (c *socketConnector) read() ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(c.conn, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > maxCanalPacket {
		return nil, errors.New("canal packet size invalid")
	}
	body := make([]byte, int(size))
	_, err := io.ReadFull(c.conn, body)
	return body, err
}

func (c *socketConnector) write(kind packet.PacketType, body proto.Message) error {
	data, err := proto.Marshal(body)
	if err != nil {
		return err
	}
	data, err = proto.Marshal(&packet.Packet{Type: kind, Body: data})
	if err != nil {
		return err
	}
	if len(data) > maxCanalPacket {
		return errors.New("canal packet size invalid")
	}
	frame := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(data)))
	copy(frame[4:], data)
	for len(frame) > 0 {
		n, err := c.conn.Write(frame)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}

func (c *socketConnector) readAck() error {
	data, err := c.read()
	if err != nil {
		return err
	}
	p := &packet.Packet{}
	if err := proto.Unmarshal(data, p); err != nil {
		return err
	}
	if p.GetType() != packet.PacketType_ACK {
		return errors.New("canal expected ACK packet")
	}
	ack := &packet.Ack{}
	if err := proto.Unmarshal(p.GetBody(), ack); err != nil {
		return err
	}
	if ack.GetErrorCode() != 0 {
		return fmt.Errorf("canal error %d: %s", ack.GetErrorCode(), ack.GetErrorMessage())
	}
	return nil
}
