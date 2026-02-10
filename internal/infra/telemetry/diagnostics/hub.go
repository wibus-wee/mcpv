package diagnostics

import (
	"context"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry"
)

const (
	// DefaultEventBufferSize is the default ring buffer size for events.
	DefaultEventBufferSize = 2048
	// DefaultEventQueueSize is the default channel size for events.
	DefaultEventQueueSize = 2048
	// DefaultLogBufferSize is the default ring buffer size for logs.
	DefaultLogBufferSize = 1024
	// DefaultLogQueueSize is the default channel size for logs.
	DefaultLogQueueSize = 1024
)

// Hub stores diagnostics events and logs in a non-blocking buffer.
type Hub struct {
	events           *AsyncBuffer[Event]
	logs             *AsyncBuffer[domain.LogEntry]
	captureSensitive bool
}

// HubOptions configures the diagnostics hub.
type HubOptions struct {
	EventBufferSize  int
	EventQueueSize   int
	LogBufferSize    int
	LogQueueSize     int
	CaptureSensitive bool
}

// NewHub constructs a diagnostics hub and starts background collectors.
func NewHub(ctx context.Context, logs *telemetry.LogBroadcaster, opts HubOptions) *Hub {
	if ctx == nil {
		ctx = context.Background()
	}
	eventBufferSize := opts.EventBufferSize
	if eventBufferSize <= 0 {
		eventBufferSize = DefaultEventBufferSize
	}
	eventQueueSize := opts.EventQueueSize
	if eventQueueSize <= 0 {
		eventQueueSize = DefaultEventQueueSize
	}
	logBufferSize := opts.LogBufferSize
	if logBufferSize <= 0 {
		logBufferSize = DefaultLogBufferSize
	}
	logQueueSize := opts.LogQueueSize
	if logQueueSize <= 0 {
		logQueueSize = DefaultLogQueueSize
	}

	hub := &Hub{
		events:           NewAsyncBuffer[Event](eventBufferSize, eventQueueSize),
		logs:             NewAsyncBuffer[domain.LogEntry](logBufferSize, logQueueSize),
		captureSensitive: opts.CaptureSensitive,
	}
	hub.events.Start(ctx)
	hub.logs.Start(ctx)

	if logs != nil {
		ch := logs.Subscribe(ctx)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case entry, ok := <-ch:
					if !ok {
						return
					}
					hub.logs.Add(entry)
				}
			}
		}()
	}

	return hub
}

// Record stores a diagnostics event asynchronously.
func (h *Hub) Record(event Event) {
	if h == nil || h.events == nil {
		return
	}
	h.events.Add(event)
}

// CaptureSensitive reports whether sensitive data collection is enabled.
func (h *Hub) CaptureSensitive() bool {
	if h == nil {
		return false
	}
	return h.captureSensitive
}

// Events returns a snapshot of stored diagnostics events.
func (h *Hub) Events() []Event {
	if h == nil || h.events == nil {
		return nil
	}
	return h.events.Snapshot()
}

// Logs returns a snapshot of stored log entries.
func (h *Hub) Logs() []domain.LogEntry {
	if h == nil || h.logs == nil {
		return nil
	}
	return h.logs.Snapshot()
}

// DroppedEvents returns the number of dropped diagnostics events.
func (h *Hub) DroppedEvents() uint64 {
	if h == nil || h.events == nil {
		return 0
	}
	return h.events.Dropped()
}

// DroppedLogs returns the number of dropped log entries.
func (h *Hub) DroppedLogs() uint64 {
	if h == nil || h.logs == nil {
		return 0
	}
	return h.logs.Dropped()
}

var _ Probe = (*Hub)(nil)
var _ SensitiveProbe = (*Hub)(nil)
