package aggregator

import "context"

type RefreshGate struct {
	ch chan struct{}
}

func NewRefreshGate() *RefreshGate {
	return &RefreshGate{ch: make(chan struct{}, 1)}
}

func (g *RefreshGate) Acquire(ctx context.Context) error {
	if g == nil {
		return nil
	}
	select {
	case g.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *RefreshGate) Release() {
	if g == nil {
		return
	}
	select {
	case <-g.ch:
	default:
	}
}
