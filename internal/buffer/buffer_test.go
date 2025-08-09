package buffer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnqueueFetchMarkSent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "buffer.db")
	store, err := Init(dbPath)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer store.Close()

	payload := []byte("hello-world")
	id, err := store.Enqueue(payload)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected non-zero id")
	}

	msgs, err := store.FetchUnsent(10)
	if err != nil {
		t.Fatalf("FetchUnsent failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if string(msgs[0].Payload) != string(payload) {
		t.Fatalf("payload mismatch")
	}

	// Mark sent
	if err := store.MarkSent([]int64{msgs[0].ID}); err != nil {
		t.Fatalf("MarkSent failed: %v", err)
	}

	msgs2, err := store.FetchUnsent(10)
	if err != nil {
		t.Fatalf("FetchUnsent failed: %v", err)
	}
	if len(msgs2) != 0 {
		t.Fatalf("expected 0 unsent messages after MarkSent, got %d", len(msgs2))
	}

	// ensure DB file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("db file not found")
	}
}
