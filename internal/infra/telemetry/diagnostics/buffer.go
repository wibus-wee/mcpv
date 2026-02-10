package diagnostics

import "sync"

// RingBuffer stores the most recent values in a fixed-size ring.
type RingBuffer[T any] struct {
	mu    sync.Mutex
	items []T
	size  int
	next  int
}

// NewRingBuffer constructs a ring buffer with the provided capacity.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	if capacity < 1 {
		capacity = 1
	}
	return &RingBuffer[T]{
		items: make([]T, capacity),
	}
}

// Add inserts a value into the ring buffer.
func (b *RingBuffer[T]) Add(value T) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.items[b.next] = value
	b.next = (b.next + 1) % len(b.items)
	if b.size < len(b.items) {
		b.size++
	}
	b.mu.Unlock()
}

// Snapshot returns the buffered values in insertion order.
func (b *RingBuffer[T]) Snapshot() []T {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.size == 0 {
		return nil
	}
	out := make([]T, 0, b.size)
	if b.size < len(b.items) {
		out = append(out, b.items[:b.size]...)
		return out
	}
	out = append(out, b.items[b.next:]...)
	out = append(out, b.items[:b.next]...)
	return out
}
