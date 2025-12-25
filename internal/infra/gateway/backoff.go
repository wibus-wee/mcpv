package gateway

import (
	"context"
	"time"
)

type backoff struct {
	base    time.Duration
	max     time.Duration
	current time.Duration
}

func newBackoff(base, max time.Duration) *backoff {
	if base <= 0 {
		base = time.Second
	}
	if max < base {
		max = base
	}
	return &backoff{base: base, max: max, current: base}
}

func (b *backoff) Reset() {
	b.current = b.base
}

func (b *backoff) Sleep(ctx context.Context) {
	timer := time.NewTimer(b.current)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	next := b.current * 2
	if next > b.max {
		next = b.max
	}
	b.current = next
}
