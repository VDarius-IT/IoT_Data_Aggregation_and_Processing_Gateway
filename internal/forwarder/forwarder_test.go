package forwarder

import (
	"testing"
	"time"
	"path/filepath"

	"github.com/your-username/iot-edge-gateway/internal/buffer"
)

// mock producer implements kafka.ProducerClient
type mockProducer struct {
	fail  bool
	calls int
}

func (m *mockProducer) Produce(payload []byte, timeout time.Duration) error {
	m.calls++
	if m.fail {
		return errMock
	}
	return nil
}

func (m *mockProducer) Close() {}

var errMock = &mockErr{"mock"}

type mockErr struct{ s string }
func (e *mockErr) Error() string { return e.s }

func TestForwarderFlushOnceSuccess(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "buffer.db")
	store, err := buffer.Init(dbPath)
	if err != nil {
		t.Fatalf("buffer init: %v", err)
	}
	defer store.Close()

	_, err = store.Enqueue([]byte("m1"))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	_, err = store.Enqueue([]byte("m2"))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	mock := &mockProducer{fail:false}
	f := New(store, mock, 1*time.Second, 1, 1*time.Second)
	// call FlushOnce to process pending messages
	f.FlushOnce()

	// verify producer was called at least twice
	if mock.calls < 2 {
		t.Fatalf("expected >=2 produce calls, got %d", mock.calls)
	}

	// ensure buffer has no unsent messages
	msgs, err := store.FetchUnsent(10)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 unsent messages after successful forward, got %d", len(msgs))
	}
}

func TestForwarderFlushOnceRetryFail(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "buffer2.db")
	store, err := buffer.Init(dbPath)
	if err != nil {
		t.Fatalf("buffer init: %v", err)
	}
	defer store.Close()

	_, err = store.Enqueue([]byte("m1"))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	_, err = store.Enqueue([]byte("m2"))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// producer always fails
	mock := &mockProducer{fail:true}
	f := New(store, mock, 1*time.Second, 1, 100*time.Millisecond)
	// attempt to flush; since producer fails, messages should remain unsent
	f.FlushOnce()

	// verify producer was called at least once
	if mock.calls == 0 {
		t.Fatalf("expected produce to be attempted at least once")
	}

	// ensure buffer still has unsent messages
	msgs, err := store.FetchUnsent(10)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatalf("expected unsent messages to remain after failed forward")
	}
}
