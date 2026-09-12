package rabbitmq_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/rabbitmq/amqp091-go"
)

func TestPublisherPublishAck(t *testing.T) {
	channel := newFakePublishChannel()
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher returned error: %v", err)
	}

	err = publisher.Publish(context.Background(), rabbitmq.PublishRequest{
		Exchange:   "events.x",
		RoutingKey: "events",
		Body:       []byte(`{"ok":true}`),
	})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if !channel.confirmEnabled {
		t.Fatal("expected confirm mode")
	}
	if channel.published.DeliveryMode != amqp091.Persistent {
		t.Fatalf("expected persistent message, got %d", channel.published.DeliveryMode)
	}
}

func TestPublisherPublishMultipleConfirms(t *testing.T) {
	channel := newFakePublishChannel()
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher returned error: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := publisher.Publish(context.Background(), rabbitmq.PublishRequest{Exchange: "events.x", RoutingKey: "events"}); err != nil {
			t.Fatalf("Publish %d returned error: %v", i, err)
		}
	}
	if channel.notifyCount != 1 {
		t.Fatalf("expected one confirm channel registration, got %d", channel.notifyCount)
	}
}

func TestPublisherPublishBatchWaitsForAllConfirms(t *testing.T) {
	channel := newFakePublishChannel()
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher returned error: %v", err)
	}

	err = publisher.PublishBatch(context.Background(), []rabbitmq.PublishRequest{
		{Exchange: "events.x", RoutingKey: "events.1", Body: []byte(`{"n":1}`)},
		{Exchange: "events.x", RoutingKey: "events.2", Body: []byte(`{"n":2}`)},
		{Exchange: "events.x", RoutingKey: "events.3", Body: []byte(`{"n":3}`)},
	})
	if err != nil {
		t.Fatalf("PublishBatch returned error: %v", err)
	}
	if channel.publishCount != 3 {
		t.Fatalf("expected three publishes, got %d", channel.publishCount)
	}
	if channel.notifyCount != 1 {
		t.Fatalf("expected one confirm channel registration, got %d", channel.notifyCount)
	}
}

func TestPublisherPublishNack(t *testing.T) {
	channel := newFakePublishChannel()
	channel.ack = false
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher returned error: %v", err)
	}

	err = publisher.Publish(context.Background(), rabbitmq.PublishRequest{Exchange: "events.x", RoutingKey: "events"})
	if err == nil {
		t.Fatal("expected nack error")
	}
}

func TestPublisherPublishBatchNack(t *testing.T) {
	channel := newFakePublishChannel()
	channel.ack = false
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher returned error: %v", err)
	}

	err = publisher.PublishBatch(context.Background(), []rabbitmq.PublishRequest{
		{Exchange: "events.x", RoutingKey: "events.1"},
		{Exchange: "events.x", RoutingKey: "events.2"},
	})
	if err == nil {
		t.Fatal("expected nack error")
	}
	if channel.publishCount != 2 {
		t.Fatalf("expected both publishes before confirm failure, got %d", channel.publishCount)
	}
}

func TestPublisherPublishError(t *testing.T) {
	channel := newFakePublishChannel()
	channel.publishErr = errors.New("publish failed")
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher returned error: %v", err)
	}

	err = publisher.Publish(context.Background(), rabbitmq.PublishRequest{Exchange: "events.x", RoutingKey: "events"})
	if err == nil {
		t.Fatal("expected publish error")
	}
}

func TestPublisherPublishBatchBoundedWindows(t *testing.T) {
	for _, size := range []int{65, 128, 1000} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			channel := newFakePublishChannel()
			publisher := mustNewPublisher(t, channel)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := publisher.PublishBatch(ctx, makePublishRequests(size)); err != nil {
				t.Fatalf("PublishBatch: %v", err)
			}
			if cap(channel.confirmations) > 64 || channel.maxBuffered > 64 {
				t.Fatalf("unbounded confirms: capacity=%d buffered=%d", cap(channel.confirmations), channel.maxBuffered)
			}
			if channel.publishCount != size || len(channel.confirmations) != 0 {
				t.Fatalf("published=%d, undrained=%d", channel.publishCount, len(channel.confirmations))
			}
			for i, key := range channel.routingKeys {
				if key != fmt.Sprint(i) {
					t.Fatalf("publish order at %d: %s", i, key)
				}
			}
		})
	}
}

func TestPublisherPublishBatchLateNackDrainsWindow(t *testing.T) {
	for _, nackAt := range []int{1, 63, 64, 100, 128} {
		t.Run(fmt.Sprint(nackAt), func(t *testing.T) {
			channel := newFakePublishChannel()
			channel.onPublish = func(ctx context.Context, attempt int) error {
				channel.confirmations <- amqp091.Confirmation{DeliveryTag: uint64(attempt), Ack: attempt != nackAt}
				return nil
			}
			publisher := mustNewPublisher(t, channel)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := publisher.PublishBatch(ctx, makePublishRequests(1000))
			if err == nil || !strings.Contains(err.Error(), "not acknowledged") {
				t.Fatalf("expected nack, got %v", err)
			}
			if want := ((nackAt-1)/64 + 1) * 64; channel.publishCount != want {
				t.Fatalf("published past failed window: got %d, want %d", channel.publishCount, want)
			}
			if len(channel.confirmations) != 0 {
				t.Fatalf("stale confirms after nack: %d", len(channel.confirmations))
			}
			channel.onPublish = nil
			channel.ack = false
			if err := publisher.Publish(ctx, rabbitmq.PublishRequest{}); err == nil {
				t.Fatal("retry consumed a stale ACK instead of its NACK")
			}
			channel.ack = true
			if err := publisher.Publish(ctx, rabbitmq.PublishRequest{}); err != nil {
				t.Fatalf("reuse after drained nacks: %v", err)
			}
		})
	}
}

func TestPublisherCancellationRecoversPendingConfirms(t *testing.T) {
	for _, size := range []int{1, 3, 5} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			channel := newFakePublishChannel()
			publisher := mustNewPublisher(t, channel)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sent := min(size, 3)
			channel.onPublish = func(ctx context.Context, attempt int) error {
				if attempt == sent {
					cancel()
				}
				return nil
			}
			var err error
			if size == 1 {
				err = publisher.Publish(ctx, rabbitmq.PublishRequest{})
			} else {
				err = publisher.PublishBatch(ctx, makePublishRequests(size))
			}
			if !errors.Is(err, context.Canceled) || channel.publishCount != sent {
				t.Fatalf("cancelled publish: sent=%d err=%v", channel.publishCount, err)
			}
			for tag := 1; tag < sent; tag++ {
				channel.confirmations <- amqp091.Confirmation{DeliveryTag: uint64(tag), Ack: true}
			}
			channel.onPublish = nil
			channel.ack = false
			// Leave one old confirmation outstanding: no new request may be sent.
			retryCtx, retryCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer retryCancel()
			if err := publisher.Publish(retryCtx, rabbitmq.PublishRequest{}); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected recovery timeout, got %v", err)
			}
			if channel.publishCalls != sent {
				t.Fatalf("published before recovery finished: calls=%d", channel.publishCalls)
			}
			channel.confirmations <- amqp091.Confirmation{DeliveryTag: uint64(sent), Ack: true}
			recoveredCtx, recoveredCancel := context.WithTimeout(context.Background(), time.Second)
			defer recoveredCancel()
			if err := publisher.Publish(recoveredCtx, rabbitmq.PublishRequest{}); err == nil || !strings.Contains(err.Error(), "not acknowledged") {
				t.Fatalf("expected new request's nack after recovery, got %v", err)
			}
			channel.ack = true
			if err := publisher.PublishBatch(recoveredCtx, makePublishRequests(1000)); err != nil {
				t.Fatalf("large batch after recovery: %v", err)
			}
		})
	}
}

func TestPublisherNackDrainInterruptedThenRetry(t *testing.T) {
	channel := newFakePublishChannel()
	publisher := mustNewPublisher(t, channel)
	channel.onPublish = func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			channel.confirmations <- amqp091.Confirmation{DeliveryTag: 1, Ack: false}
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := publisher.PublishBatch(ctx, makePublishRequests(3))
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "not acknowledged") {
		t.Fatalf("expected nack and interrupted drain, got %v", err)
	}
	channel.confirmations <- amqp091.Confirmation{DeliveryTag: 2, Ack: false}
	channel.confirmations <- amqp091.Confirmation{DeliveryTag: 3, Ack: true}
	channel.onPublish = nil
	retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
	defer retryCancel()
	if err := publisher.Publish(retryCtx, rabbitmq.PublishRequest{}); err != nil {
		t.Fatalf("old nack must not fail new publish: %v", err)
	}
	if channel.publishCount != 4 || len(channel.confirmations) != 0 {
		t.Fatalf("published=%d, pending=%d", channel.publishCount, len(channel.confirmations))
	}
}

func TestPublisherCancellationAfterCompletedWindow(t *testing.T) {
	channel := newFakePublishChannel()
	publisher := mustNewPublisher(t, channel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	channel.onPublish = func(ctx context.Context, attempt int) error {
		if attempt <= 64 {
			channel.confirmations <- amqp091.Confirmation{DeliveryTag: uint64(attempt), Ack: true}
		}
		if attempt == 67 {
			cancel()
		}
		return nil
	}
	if err := publisher.PublishBatch(ctx, makePublishRequests(1000)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation in second window, got %v", err)
	}
	if channel.publishCount != 67 {
		t.Fatalf("published=%d, want 67", channel.publishCount)
	}
	for tag := 65; tag <= 67; tag++ {
		channel.confirmations <- amqp091.Confirmation{DeliveryTag: uint64(tag), Ack: tag != 66}
	}
	channel.onPublish = nil
	retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
	defer retryCancel()
	if err := publisher.Publish(retryCtx, rabbitmq.PublishRequest{}); err != nil {
		t.Fatalf("recover only current window, ignoring its old nack: %v", err)
	}
	if channel.publishCount != 68 || len(channel.confirmations) != 0 {
		t.Fatalf("published=%d, pending=%d", channel.publishCount, len(channel.confirmations))
	}
}

func TestPublisherPartialBatchValidationFailureCanRecover(t *testing.T) {
	channel := newFakePublishChannel()
	publisher := mustNewPublisher(t, channel)
	reqs := makePublishRequests(4)
	reqs[2].Headers = amqp091.Table{"invalid": make(chan int)}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := publisher.PublishBatch(ctx, reqs); err == nil {
		t.Fatal("expected header validation error")
	}
	if channel.publishCount != 2 {
		t.Fatalf("published=%d, want 2", channel.publishCount)
	}
	channel.ack = false
	if err := publisher.Publish(ctx, rabbitmq.PublishRequest{}); err == nil {
		t.Fatal("retry consumed a stale ACK instead of its NACK")
	}
	channel.ack = true
	if err := publisher.Publish(ctx, rabbitmq.PublishRequest{}); err != nil {
		t.Fatalf("reusable after local validation failure: %v", err)
	}
}

func TestPublisherPartialBatchSendCancellationCanRecover(t *testing.T) {
	channel := newFakePublishChannel()
	publisher := mustNewPublisher(t, channel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	channel.onPublish = func(ctx context.Context, attempt int) error {
		if attempt == 3 {
			cancel()
			return ctx.Err()
		}
		channel.confirmations <- amqp091.Confirmation{DeliveryTag: uint64(attempt), Ack: true}
		return nil
	}
	if err := publisher.PublishBatch(ctx, makePublishRequests(5)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation from send, got %v", err)
	}
	channel.onPublish = nil
	channel.ack = false
	retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
	defer retryCancel()
	if err := publisher.Publish(retryCtx, rabbitmq.PublishRequest{}); err == nil || !strings.Contains(err.Error(), "not acknowledged") {
		t.Fatalf("expected new nack, not old ack or permanent failure, got %v", err)
	}
	if channel.publishCalls != 4 || channel.publishCount != 3 || len(channel.confirmations) != 0 {
		t.Fatalf("attempts=%d, published=%d, pending=%d", channel.publishCalls, channel.publishCount, len(channel.confirmations))
	}
}

func TestPublisherAmbiguousSendFailurePreventsReuse(t *testing.T) {
	for _, failAt := range []int{1, 3} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			channel := newFakePublishChannel()
			publisher := mustNewPublisher(t, channel)
			sendErr := errors.New("partial socket write")
			channel.onPublish = func(ctx context.Context, attempt int) error {
				// Even a failed write may have reached the broker.
				channel.confirmations <- amqp091.Confirmation{DeliveryTag: uint64(attempt), Ack: true}
				if attempt == failAt {
					return sendErr
				}
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := publisher.PublishBatch(ctx, makePublishRequests(5)); !errors.Is(err, sendErr) {
				t.Fatalf("expected send error, got %v", err)
			}
			channel.onPublish = nil
			if err := publisher.Publish(ctx, rabbitmq.PublishRequest{}); !errors.Is(err, sendErr) || !strings.Contains(err.Error(), "recreate channel") {
				t.Fatalf("expected unusable publisher, got %v", err)
			}
			if err := publisher.PublishBatch(ctx, makePublishRequests(3)); !errors.Is(err, sendErr) {
				t.Fatalf("batch reused uncertain confirm stream: %v", err)
			}
			if channel.publishCalls != failAt {
				t.Fatalf("retried ambiguous send on same channel: %d calls", channel.publishCalls)
			}
		})
	}
}

func TestPublisherWrappedTransportCancellationPreventsReuse(t *testing.T) {
	channel := newFakePublishChannel()
	publisher := mustNewPublisher(t, channel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sendErr := fmt.Errorf("transport write: %w", context.Canceled)
	channel.onPublish = func(ctx context.Context, attempt int) error {
		cancel()
		return sendErr
	}
	if err := publisher.Publish(ctx, rabbitmq.PublishRequest{}); !errors.Is(err, sendErr) {
		t.Fatalf("expected transport failure, got %v", err)
	}
	if err := publisher.Publish(context.Background(), rabbitmq.PublishRequest{}); !errors.Is(err, sendErr) || !strings.Contains(err.Error(), "recreate channel") {
		t.Fatalf("transport error mistaken for pre-send cancellation: %v", err)
	}
	if channel.publishCalls != 1 {
		t.Fatalf("reused publisher after transport error: %d calls", channel.publishCalls)
	}
}

func TestPublisherConfirmationChannelClosure(t *testing.T) {
	for _, recovering := range []bool{false, true} {
		t.Run(fmt.Sprint(recovering), func(t *testing.T) {
			channel := newFakePublishChannel()
			publisher := mustNewPublisher(t, channel)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			channel.onPublish = func(ctx context.Context, attempt int) error {
				if attempt == 1 {
					channel.confirmations <- amqp091.Confirmation{DeliveryTag: 1, Ack: true}
				}
				if attempt == 3 {
					close(channel.confirmations)
					if recovering {
						cancel()
					}
				}
				return nil
			}
			err := publisher.PublishBatch(ctx, makePublishRequests(3))
			wantErr := error(amqp091.ErrClosed)
			if recovering {
				wantErr = context.Canceled
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("expected %v, got %v", wantErr, err)
			}
			retryCtx, retryCancel := context.WithTimeout(context.Background(), time.Second)
			defer retryCancel()
			for i := 0; i < 2; i++ {
				if err := publisher.Publish(retryCtx, rabbitmq.PublishRequest{}); !errors.Is(err, amqp091.ErrClosed) {
					t.Fatalf("expected closed publisher, got %v", err)
				}
			}
			if channel.publishCalls != 3 {
				t.Fatalf("published on closed confirm stream: %d calls", channel.publishCalls)
			}
		})
	}
}

func TestPublisherCancelledBeforeSend(t *testing.T) {
	channel := newFakePublishChannel()
	publisher := mustNewPublisher(t, channel)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := publisher.Publish(ctx, rabbitmq.PublishRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancelled context, got %v", err)
	}
	if channel.publishCalls != 0 {
		t.Fatal("sent a request with cancelled context")
	}
}

func TestPublisherSerializesConcurrentBatches(t *testing.T) {
	channel := newFakePublishChannel()
	publisher := mustNewPublisher(t, channel)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for batch := 0; batch < 10; batch++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqs := makePublishRequests(100)
			for i := range reqs {
				reqs[i].RoutingKey = fmt.Sprint(batch)
			}
			if err := publisher.PublishBatch(ctx, reqs); err != nil {
				t.Errorf("concurrent batch: %v", err)
			}
		}()
	}
	wg.Wait()
	if channel.publishCount != 1000 || len(channel.confirmations) != 0 {
		t.Fatalf("published=%d, pending=%d", channel.publishCount, len(channel.confirmations))
	}
	for i, key := range channel.routingKeys {
		if key != channel.routingKeys[i/100*100] {
			t.Fatalf("batches interleaved at request %d", i)
		}
	}
}

func mustNewPublisher(t *testing.T, channel *fakePublishChannel) *rabbitmq.Publisher {
	t.Helper()
	publisher, err := rabbitmq.NewPublisher(channel)
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	return publisher
}

func makePublishRequests(size int) []rabbitmq.PublishRequest {
	reqs := make([]rabbitmq.PublishRequest, size)
	for i := range reqs {
		reqs[i] = rabbitmq.PublishRequest{Exchange: "events.x", RoutingKey: fmt.Sprint(i)}
	}
	return reqs
}

type fakePublishChannel struct {
	confirmEnabled bool
	ack            bool
	publishErr     error
	published      amqp091.Publishing
	publishCount   int
	publishCalls   int
	confirmations  chan amqp091.Confirmation
	notifyCount    int
	maxBuffered    int
	routingKeys    []string
	onPublish      func(context.Context, int) error
}

func newFakePublishChannel() *fakePublishChannel {
	return &fakePublishChannel{ack: true, confirmations: make(chan amqp091.Confirmation, 64)}
}

func (c *fakePublishChannel) Confirm(noWait bool) error {
	c.confirmEnabled = true
	return nil
}

func (c *fakePublishChannel) NotifyPublish(confirm chan amqp091.Confirmation) chan amqp091.Confirmation {
	c.notifyCount++
	c.confirmations = confirm
	return confirm
}

func (c *fakePublishChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
	c.publishCalls++
	if c.publishErr != nil {
		return c.publishErr
	}
	if c.onPublish != nil {
		if err := c.onPublish(ctx, c.publishCalls); err != nil {
			return err
		}
	} else {
		select {
		case c.confirmations <- amqp091.Confirmation{DeliveryTag: uint64(c.publishCount + 1), Ack: c.ack}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.publishCount++
	c.published = msg
	c.maxBuffered = max(c.maxBuffered, len(c.confirmations))
	c.routingKeys = append(c.routingKeys, key)
	return nil
}
