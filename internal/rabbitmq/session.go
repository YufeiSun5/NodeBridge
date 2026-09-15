package rabbitmq

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

const sessionOperationTimeout = 15 * time.Second

// Session replaces failed transports; callers retain responsibility for retries.
// Deliveries always acknowledge their original channel, never a replacement.
type Session struct {
	mu        sync.Mutex
	url       string
	conn      *Connection
	publisher *Publisher
	closed    bool
}

func DialSession(ctx context.Context, url string) (*Session, error) {
	s := &Session{url: url}
	if err := s.WithChannel(ctx, func(*amqp091.Channel) error { return nil }); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) connect(ctx context.Context) error {
	if s.conn != nil && !s.conn.Conn.IsClosed() && !s.conn.Channel.IsClosed() {
		return nil
	}
	s.discard()
	conn, err := amqp091.DialConfig(s.url, amqp091.Config{
		Heartbeat: 5 * time.Second,
		Dial: func(network, addr string) (net.Conn, error) {
			conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			deadline := time.Now().Add(5 * time.Second)
			if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
				deadline = limit
			}
			if err := conn.SetDeadline(deadline); err != nil {
				_ = conn.Close()
				return nil, err
			}
			return conn, nil
		},
	})
	if err != nil {
		return fmt.Errorf("rabbitmq reconnect failed: %w", err)
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.CloseDeadline(time.Now())
		return fmt.Errorf("rabbitmq channel recreate failed: %w", err)
	}
	s.conn = &Connection{Conn: conn, Channel: channel}
	return nil
}

// WithChannel keeps a batch on one generation and bounds stalled network I/O.
func (s *Session) WithChannel(ctx context.Context, operation func(*amqp091.Channel) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("rabbitmq session stopped: %w", amqp091.ErrClosed)
	}
	ctx, cancel := context.WithTimeout(ctx, sessionOperationTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.connect(ctx); err != nil {
		return err
	}
	conn := s.conn.Conn
	stopped := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = conn.CloseDeadline(time.Now()); close(stopped) })
	err := operation(s.conn.Channel)
	if !stop() {
		<-stopped
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		s.discard()
		return fmt.Errorf("rabbitmq transport reset; reconnect on retry: %w", err)
	}
	return nil
}

func (s *Session) Publish(ctx context.Context, req PublishRequest) error {
	return s.PublishBatch(ctx, []PublishRequest{req})
}

func (s *Session) PublishBatch(ctx context.Context, reqs []PublishRequest) error {
	return s.WithChannel(ctx, func(channel *amqp091.Channel) error {
		if s.publisher == nil {
			var err error
			s.publisher, err = NewPublisher(channel)
			if err != nil {
				return err
			}
		}
		return s.publisher.PublishBatch(ctx, reqs)
	})
}

func (s *Session) discard() {
	if s.conn != nil {
		_ = s.conn.Conn.CloseDeadline(time.Now())
	}
	s.conn, s.publisher = nil, nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.discard()
	return nil
}
