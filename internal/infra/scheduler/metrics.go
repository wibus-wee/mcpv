package scheduler

import (
	"context"
	"errors"
	"time"

	"mcpv/internal/domain"
)

// GetPoolStatus returns a snapshot of all pool states for status queries.
func (s *BasicScheduler) GetPoolStatus(_ context.Context) ([]domain.PoolInfo, error) {
	entries := s.snapshotPools()
	result := make([]domain.PoolInfo, 0, len(entries))

	for _, entry := range entries {
		entry.state.mu.Lock()
		instances := make([]domain.InstanceInfo, 0, len(entry.state.instances)+len(entry.state.draining))
		metrics := domain.PoolMetrics{
			StartCount: entry.state.startCount,
			StopCount:  entry.state.stopCount,
		}

		// Include active instances
		for _, inst := range entry.state.instances {
			instances = append(instances, inst.instance.Info())
			stats := inst.instance.CallStats()
			metrics.TotalCalls += stats.TotalCalls
			metrics.TotalErrors += stats.TotalErrors
			metrics.TotalDuration += stats.TotalDuration
			if stats.LastCallAt.After(metrics.LastCallAt) {
				metrics.LastCallAt = stats.LastCallAt
			}
		}

		// Include draining instances
		for _, inst := range entry.state.draining {
			instances = append(instances, inst.instance.Info())
		}

		minReady := entry.state.minReady
		serverName := entry.state.spec.Name
		entry.state.mu.Unlock()

		result = append(result, domain.PoolInfo{
			SpecKey:    entry.specKey,
			ServerName: serverName,
			MinReady:   minReady,
			Instances:  instances,
			Metrics:    metrics,
		})
	}

	return result, nil
}

func (s *BasicScheduler) observeInstanceStart(serverType string, start time.Time, err error) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveInstanceStart(serverType, time.Since(start), err)
}

func (s *BasicScheduler) observeInstanceStartCause(ctx context.Context, serverType string) {
	if s.metrics == nil {
		return
	}
	cause, ok := domain.StartCauseFromContext(ctx)
	if !ok {
		s.metrics.ObserveInstanceStartCause(serverType, domain.StartCauseReason("unknown"))
		return
	}
	s.metrics.ObserveInstanceStartCause(serverType, cause.Reason)
}

func (s *BasicScheduler) observeInstanceStop(serverType string, err error) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveInstanceStop(serverType, err)
}

func (s *BasicScheduler) observePoolWait(serverType string, duration time.Duration, outcome domain.PoolWaitOutcome) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObservePoolWait(serverType, duration, outcome)
}

func (s *BasicScheduler) observePoolAcquireFailure(serverType string, err error) {
	if s.metrics == nil {
		return
	}
	reason, ok := classifyAcquireFailure(err)
	if !ok {
		return
	}
	s.metrics.ObservePoolAcquireFailure(serverType, reason)
}

func classifyAcquireFailure(err error) (domain.AcquireFailureReason, bool) {
	switch {
	case errors.Is(err, domain.ErrNoReadyInstance):
		return domain.AcquireFailureNoReady, true
	case errors.Is(err, ErrNoCapacity):
		return domain.AcquireFailureNoCapacity, true
	case errors.Is(err, ErrStickyBusy):
		return domain.AcquireFailureStickyBusy, true
	default:
		return "", false
	}
}

func (s *BasicScheduler) recordInstanceStop(state *poolState) {
	state.mu.Lock()
	state.stopCount++
	state.mu.Unlock()
}

func (s *BasicScheduler) observePoolStats(state *poolState) {
	if s.metrics == nil {
		return
	}
	state.mu.Lock()
	activeCount := len(state.instances)
	busyCount := 0
	maxConcurrent := state.spec.MaxConcurrent
	serverType := state.spec.Name
	waiterCount := state.waiters
	startingCount := state.starting
	for _, inst := range state.instances {
		busyCount += inst.instance.BusyCount()
	}
	state.mu.Unlock()

	s.metrics.SetStartingInstances(serverType, startingCount)
	s.metrics.SetActiveInstances(serverType, activeCount)
	capacity := activeCount * maxConcurrent
	ratio := 0.0
	if capacity > 0 {
		ratio = float64(busyCount) / float64(capacity)
	}
	s.metrics.SetPoolCapacityRatio(serverType, ratio)
	s.metrics.SetPoolWaiters(serverType, waiterCount)
}
