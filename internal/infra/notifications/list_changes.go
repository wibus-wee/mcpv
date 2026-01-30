package notifications

import (
	"context"
	"sync"

	"mcpv/internal/domain"
)

const defaultListChangeBuffer = 4

type ListChangeHub struct {
	mu   sync.RWMutex
	subs map[domain.ListChangeKind]map[chan domain.ListChangeEvent]struct{}
}

func NewListChangeHub() *ListChangeHub {
	return &ListChangeHub{
		subs: make(map[domain.ListChangeKind]map[chan domain.ListChangeEvent]struct{}),
	}
}

func (h *ListChangeHub) EmitListChange(event domain.ListChangeEvent) {
	if h == nil {
		return
	}
	h.mu.RLock()
	subs := h.subs[event.Kind]
	h.mu.RUnlock()
	for ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *ListChangeHub) Subscribe(ctx context.Context, kind domain.ListChangeKind) <-chan domain.ListChangeEvent {
	ch := make(chan domain.ListChangeEvent, defaultListChangeBuffer)
	if h == nil {
		close(ch)
		return ch
	}

	h.mu.Lock()
	if h.subs[kind] == nil {
		h.subs[kind] = make(map[chan domain.ListChangeEvent]struct{})
	}
	h.subs[kind][ch] = struct{}{}
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		h.mu.Lock()
		if h.subs[kind] != nil {
			delete(h.subs[kind], ch)
		}
		close(ch)
		h.mu.Unlock()
	}()

	return ch
}

var _ domain.ListChangeEmitter = (*ListChangeHub)(nil)
