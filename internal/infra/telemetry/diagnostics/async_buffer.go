package diagnostics

import (
	"context"
	"sync/atomic"
)

type AsyncBuffer[T any] struct {
	buffer  *RingBuffer[T]
	ch      chan T
	dropped uint64
	started atomic.Bool
}

// NewAsyncBuffer constructs an async buffer backed by a ring buffer.
func NewAsyncBuffer[T any](capacity, queue int) *AsyncBuffer[T] {
	if capacity < 1 {
		capacity = 1
	}
	if queue < 1 {
		queue = capacity
	}
	return &AsyncBuffer[T]{
		buffer: NewRingBuffer[T](capacity),
		ch:     make(chan T, queue),
	}
}

// Start begins draining the channel into the ring buffer.
func (b *AsyncBuffer[T]) Start(ctx context.Context) {
	if b == nil || b.started.Swap(true) {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case value := <-b.ch:
				b.buffer.Add(value)
			}
		}
	}()
}

// Add enqueues a value without blocking.
func (b *AsyncBuffer[T]) Add(value T) {
	if b == nil {
		return
	}
	select {
	case b.ch <- value:
	default:
		atomic.AddUint64(&b.dropped, 1)
	}
}

// Snapshot returns the buffered values.
func (b *AsyncBuffer[T]) Snapshot() []T {
	if b == nil {
		return nil
	}
	return b.buffer.Snapshot()
}

// Dropped returns the number of dropped values.
func (b *AsyncBuffer[T]) Dropped() uint64 {
	if b == nil {
		return 0
	}
	return atomic.LoadUint64(&b.dropped)
}
