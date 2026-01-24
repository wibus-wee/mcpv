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

// RuntimeStatusIndex manages server runtime status snapshots with broadcast support
type RuntimeStatusIndex struct {
	scheduler domain.Scheduler
	logger    *zap.Logger

	mu    sync.RWMutex
	subs  map[chan domain.RuntimeStatusSnapshot]struct{}
	state atomic.Value // runtimeStatusIndexState
}

type runtimeStatusIndexState struct {
	snapshot domain.RuntimeStatusSnapshot
}

// NewRuntimeStatusIndex creates a new runtime status index
func NewRuntimeStatusIndex(scheduler domain.Scheduler, logger *zap.Logger) *RuntimeStatusIndex {
	if logger == nil {
		logger = zap.NewNop()
	}

	idx := &RuntimeStatusIndex{
		scheduler: scheduler,
		logger:    logger.Named("runtime_status_index"),
		subs:      make(map[chan domain.RuntimeStatusSnapshot]struct{}),
	}

	// Initialize with empty snapshot
	idx.state.Store(runtimeStatusIndexState{
		snapshot: domain.RuntimeStatusSnapshot{
			ETag:        "",
			Statuses:    []domain.ServerRuntimeStatus{},
			GeneratedAt: time.Now(),
		},
	})

	return idx
}

// Subscribe returns a channel that receives runtime status snapshots
func (idx *RuntimeStatusIndex) Subscribe(ctx context.Context) <-chan domain.RuntimeStatusSnapshot {
	ch := make(chan domain.RuntimeStatusSnapshot, 1)

	idx.mu.Lock()
	idx.subs[ch] = struct{}{}
	idx.mu.Unlock()

	// Send current snapshot immediately
	state := idx.state.Load().(runtimeStatusIndexState)
	sendRuntimeStatusSnapshot(ch, state.snapshot)

	// Auto-cleanup on context cancel
	go func() {
		<-ctx.Done()
		idx.mu.Lock()
		delete(idx.subs, ch)
		idx.mu.Unlock()
	}()

	return ch
}

// Current returns the latest runtime status snapshot.
func (idx *RuntimeStatusIndex) Current() domain.RuntimeStatusSnapshot {
	state := idx.state.Load().(runtimeStatusIndexState)
	return state.snapshot
}

// Refresh polls the scheduler for current status and broadcasts to subscribers
func (idx *RuntimeStatusIndex) Refresh(ctx context.Context) error {
	poolInfos, err := idx.scheduler.GetPoolStatus(ctx)
	if err != nil {
		return err
	}

	statuses := make([]domain.ServerRuntimeStatus, 0, len(poolInfos))
	for _, pool := range poolInfos {
		instances := make([]domain.InstanceStatusInfo, 0, len(pool.Instances))
		for _, inst := range pool.Instances {
			instances = append(instances, domain.InstanceStatusInfo{
				ID:              inst.ID,
				State:           inst.State,
				BusyCount:       inst.BusyCount,
				LastActive:      inst.LastActive,
				SpawnedAt:       inst.SpawnedAt,
				HandshakedAt:    inst.HandshakedAt,
				LastHeartbeatAt: inst.LastHeartbeatAt,
				LastStartCause:  domain.CloneStartCause(inst.LastStartCause),
			})
		}

		// Calculate pool stats
		stats := domain.PoolStats{}
		for _, inst := range pool.Instances {
			stats.Total++
			switch inst.State {
			case domain.InstanceStateReady:
				stats.Ready++
			case domain.InstanceStateBusy:
				stats.Busy++
			case domain.InstanceStateStarting:
				stats.Starting++
			case domain.InstanceStateInitializing:
				stats.Initializing++
			case domain.InstanceStateHandshaking:
				stats.Handshaking++
			case domain.InstanceStateDraining:
				stats.Draining++
			case domain.InstanceStateFailed:
				stats.Failed++
			}
		}

		statuses = append(statuses, domain.ServerRuntimeStatus{
			SpecKey:    pool.SpecKey,
			ServerName: pool.ServerName,
			Instances:  instances,
			Stats:      stats,
			Metrics:    pool.Metrics,
		})
	}

	// Sort for consistent ETag generation
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].SpecKey < statuses[j].SpecKey
	})

	newETag := computeRuntimeStatusETag(statuses)

	// Skip broadcast if unchanged (ETag deduplication)
	state := idx.state.Load().(runtimeStatusIndexState)
	if state.snapshot.ETag == newETag {
		return nil // No change, skip broadcast
	}

	snapshot := domain.RuntimeStatusSnapshot{
		ETag:        newETag,
		Statuses:    statuses,
		GeneratedAt: time.Now(),
	}

	// Update state
	idx.state.Store(runtimeStatusIndexState{snapshot: snapshot})

	// Broadcast to subscribers
	idx.broadcast(snapshot)

	return nil
}

// broadcast sends the snapshot to all subscribers (non-blocking)
func (idx *RuntimeStatusIndex) broadcast(snapshot domain.RuntimeStatusSnapshot) {
	subs := idx.copySubscribers()
	for _, ch := range subs {
		sendRuntimeStatusSnapshot(ch, snapshot)
	}
}

// copySubscribers creates a copy of the subscriber list
func (idx *RuntimeStatusIndex) copySubscribers() []chan domain.RuntimeStatusSnapshot {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	channels := make([]chan domain.RuntimeStatusSnapshot, 0, len(idx.subs))
	for ch := range idx.subs {
		channels = append(channels, ch)
	}
	return channels
}

// computeRuntimeStatusETag generates an ETag based on status content
func computeRuntimeStatusETag(statuses []domain.ServerRuntimeStatus) string {
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

// sendRuntimeStatusSnapshot sends a snapshot to a channel (non-blocking)
func sendRuntimeStatusSnapshot(ch chan domain.RuntimeStatusSnapshot, snapshot domain.RuntimeStatusSnapshot) {
	select {
	case ch <- snapshot:
	default:
		// Skip if channel is full (slow consumer protection)
	}
}
