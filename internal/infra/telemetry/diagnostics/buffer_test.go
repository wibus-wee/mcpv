package diagnostics

import (
	"context"
	"testing"
	"time"
)

func TestRingBufferSnapshotOrder(t *testing.T) {
	buffer := NewRingBuffer[int](3)
	buffer.Add(1)
	buffer.Add(2)
	buffer.Add(3)
	buffer.Add(4)

	snapshot := buffer.Snapshot()
	if len(snapshot) != 3 {
		t.Fatalf("expected 3 items, got %d", len(snapshot))
	}
	if snapshot[0] != 2 || snapshot[1] != 3 || snapshot[2] != 4 {
		t.Fatalf("unexpected snapshot order: %+v", snapshot)
	}
}

func TestAsyncBufferDropped(t *testing.T) {
	buffer := NewAsyncBuffer[int](1, 1)
	buffer.Add(1)
	buffer.Add(2)
	if buffer.Dropped() == 0 {
		t.Fatalf("expected dropped entries")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	buffer.Start(ctx)
	buffer.Add(3)
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(buffer.Snapshot()) > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected snapshot entries")
}
