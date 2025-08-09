package forwarder

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/your-username/iot-edge-gateway/internal/buffer"
	"github.com/your-username/iot-edge-gateway/internal/kafka"
)

type Forwarder struct {
	store    *buffer.Store
	producer kafka.ProducerClient
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	retries  int
	timeout  time.Duration
}

// New creates a forwarder that polls the buffer and forwards messages to Kafka.
// interval: how often to poll the buffer
// retries: number of retries per message on transient failures
// timeout: per-message produce timeout
func New(store *buffer.Store, producer kafka.ProducerClient, interval time.Duration, retries int, timeout time.Duration) *Forwarder {
	if retries < 0 {
		retries = 3
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Forwarder{
		store:    store,
		producer: producer,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
		retries:  retries,
		timeout:  timeout,
	}
}

func (f *Forwarder) Start() {
	go f.loop()
}

func (f *Forwarder) Stop() {
	if f.cancel != nil {
		f.cancel()
	}
}

func (f *Forwarder) loop() {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.flushOnce()
		}
	}
}

func (f *Forwarder) flushOnce() {
	if f.store == nil || f.producer == nil {
		return
	}

	msgs, err := f.store.FetchUnsent(100)
	if err != nil {
		fmt.Printf("forwarder: fetch unsent error: %v\n", err)
		return
	}
	if len(msgs) == 0 {
		return
	}

	var sentIDs []int64
	for _, m := range msgs {
		ok := f.sendWithRetry(m.Payload)
		if ok {
			sentIDs = append(sentIDs, m.ID)
		} else {
			// If a message fails after retries, stop processing further to avoid reordering,
			// and leave remaining messages for the next run. This is conservative.
			fmt.Printf("forwarder: message id=%d failed after retries; will retry later\n", m.ID)
			break
		}
	}

	if len(sentIDs) > 0 {
		if err := f.store.MarkSent(sentIDs); err != nil {
			fmt.Printf("forwarder: failed to mark messages as sent: %v\n", err)
			// We don't attempt rollback; on next run, fetch will include same messages (but they may be re-sent).
		}
	}
}

// FlushOnce exposes flushOnce for testing.
func (f *Forwarder) FlushOnce() {
	f.flushOnce()
}

func (f *Forwarder) sendWithRetry(payload []byte) bool {
	var lastErr error
	for attempt := 0; attempt <= f.retries; attempt++ {
		// exponential backoff sleep for attempts > 0
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 500 * time.Millisecond
			time.Sleep(backoff)
		}
		err := f.producer.Produce(payload, f.timeout)
		if err == nil {
			return true
		}
		lastErr = err
		fmt.Printf("forwarder: produce attempt=%d failed: %v\n", attempt+1, err)
	}
	fmt.Printf("forwarder: all retries failed; last error: %v\n", lastErr)
	return false
}
