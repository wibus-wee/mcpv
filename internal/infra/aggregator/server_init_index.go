package aggregator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
)

// ServerInitIndex manages server initialization status snapshots with broadcast support
type ServerInitIndex struct {
	cp     domain.ServerInitStatusReader
	logger *zap.Logger

	mu    sync.RWMutex
	subs  map[chan domain.ServerInitStatusSnapshot]struct{}
	state atomic.Value // serverInitStatusIndexState
}

type serverInitStatusIndexState struct {
	snapshot domain.ServerInitStatusSnapshot
}

// NewServerInitIndex creates a new server init status index
func NewServerInitIndex(cp domain.ServerInitStatusReader, logger *zap.Logger) *ServerInitIndex {
	if logger == nil {
		logger = zap.NewNop()
	}

	idx := &ServerInitIndex{
		cp:     cp,
		logger: logger.Named("server_init_index"),
		subs:   make(map[chan domain.ServerInitStatusSnapshot]struct{}),
	}

	// Initialize with empty snapshot
	idx.state.Store(serverInitStatusIndexState{
		snapshot: domain.ServerInitStatusSnapshot{
			Statuses:    []domain.ServerInitStatus{},
			GeneratedAt: time.Now(),
		},
	})

	return idx
}

// Subscribe returns a channel that receives server init status snapshots
func (idx *ServerInitIndex) Subscribe(ctx context.Context) <-chan domain.ServerInitStatusSnapshot {
	ch := make(chan domain.ServerInitStatusSnapshot, 1)

	idx.mu.Lock()
	idx.subs[ch] = struct{}{}
	idx.mu.Unlock()

	// Send current snapshot immediately
	state := idx.state.Load().(serverInitStatusIndexState)
	sendServerInitStatusSnapshot(ch, state.snapshot)

	// Auto-cleanup on context cancel
	go func() {
		<-ctx.Done()
		idx.mu.Lock()
		delete(idx.subs, ch)
		idx.mu.Unlock()
	}()

	return ch
}

// Current returns the latest server init status snapshot.
func (idx *ServerInitIndex) Current() domain.ServerInitStatusSnapshot {
	state := idx.state.Load().(serverInitStatusIndexState)
	return state.snapshot
}

// Refresh polls the control plane for current init status and broadcasts to subscribers
func (idx *ServerInitIndex) Refresh(ctx context.Context) error {
	statuses, err := idx.cp.GetServerInitStatus(ctx)
	if err != nil {
		return err
	}

	// Sort for consistent hashing
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].SpecKey < statuses[j].SpecKey
	})

	newHash := computeServerInitStatusHash(statuses)

	// Skip broadcast if unchanged (hash-based deduplication)
	state := idx.state.Load().(serverInitStatusIndexState)
	oldHash := computeServerInitStatusHash(state.snapshot.Statuses)
	if oldHash == newHash {
		return nil // No change, skip broadcast
	}

	snapshot := domain.ServerInitStatusSnapshot{
		Statuses:    statuses,
		GeneratedAt: time.Now(),
	}

	// Update state
	idx.state.Store(serverInitStatusIndexState{snapshot: snapshot})

	// Broadcast to subscribers
	idx.broadcast(snapshot)

	return nil
}

// broadcast sends the snapshot to all subscribers (non-blocking)
func (idx *ServerInitIndex) broadcast(snapshot domain.ServerInitStatusSnapshot) {
	subs := idx.copySubscribers()
	for _, ch := range subs {
		sendServerInitStatusSnapshot(ch, snapshot)
	}
}

// copySubscribers creates a copy of the subscriber list
func (idx *ServerInitIndex) copySubscribers() []chan domain.ServerInitStatusSnapshot {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	channels := make([]chan domain.ServerInitStatusSnapshot, 0, len(idx.subs))
	for ch := range idx.subs {
		channels = append(channels, ch)
	}
	return channels
}

// computeServerInitStatusHash generates a hash based on status content
func computeServerInitStatusHash(statuses []domain.ServerInitStatus) string {
	if len(statuses) == 0 {
		return ""
	}

	// Create a deterministic representation
	data, err := json.Marshal(statuses)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// sendServerInitStatusSnapshot sends a snapshot to a channel (non-blocking)
func sendServerInitStatusSnapshot(ch chan domain.ServerInitStatusSnapshot, snapshot domain.ServerInitStatusSnapshot) {
	select {
	case ch <- snapshot:
	default:
		// Skip if channel is full (slow consumer protection)
	}
}
