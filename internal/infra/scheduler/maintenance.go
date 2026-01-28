package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/telemetry"
)

// StartIdleManager begins periodic idle reap respecting idleSeconds/persistent/sticky/minReady.
func (s *BasicScheduler) StartIdleManager(interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	s.mu.Lock()
	if s.idleTicker != nil {
		s.mu.Unlock()
		return
	}
	s.idleTicker = time.NewTicker(interval)
	s.stopIdle = make(chan struct{})
	stop := s.stopIdle
	s.mu.Unlock()

	if s.health != nil {
		s.idleBeat = s.health.Register("scheduler_idle", interval*3)
	}
	go func() {
		for {
			select {
			case <-s.idleTicker.C:
				if s.idleBeat != nil {
					s.idleBeat.Beat()
				}
				s.reapIdle()
			case <-stop:
				return
			}
		}
	}()
}

// StopIdleManager ends idle reap.
func (s *BasicScheduler) StopIdleManager() {
	s.mu.Lock()
	if s.idleTicker == nil {
		s.mu.Unlock()
		return
	}
	s.idleTicker.Stop()
	s.idleTicker = nil
	close(s.stopIdle)
	s.stopIdle = make(chan struct{})
	if s.idleBeat != nil {
		s.idleBeat.Stop()
		s.idleBeat = nil
	}
	s.mu.Unlock()
}

// StartPingManager begins periodic health checks.
func (s *BasicScheduler) StartPingManager(interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	s.mu.Lock()
	if s.pingTicker != nil {
		s.mu.Unlock()
		return
	}
	s.pingTicker = time.NewTicker(interval)
	s.stopPing = make(chan struct{})
	stop := s.stopPing
	s.mu.Unlock()

	if s.health != nil {
		s.pingBeat = s.health.Register("scheduler_ping", interval*3)
	}
	go func() {
		for {
			select {
			case <-s.pingTicker.C:
				if s.pingBeat != nil {
					s.pingBeat.Beat()
				}
				s.probeInstances()
			case <-stop:
				return
			}
		}
	}()
}

// StopPingManager ends health checks.
func (s *BasicScheduler) StopPingManager() {
	s.mu.Lock()
	if s.pingTicker == nil {
		s.mu.Unlock()
		return
	}
	s.pingTicker.Stop()
	s.pingTicker = nil
	close(s.stopPing)
	s.stopPing = make(chan struct{})
	if s.pingBeat != nil {
		s.pingBeat.Stop()
		s.pingBeat = nil
	}
	s.mu.Unlock()
}

func (s *BasicScheduler) reapIdle() {
	now := time.Now()

	var candidates []stopCandidate
	for _, entry := range s.snapshotPools() {
		entry.state.mu.Lock()
		readyCount := entry.state.countReadyLocked()
		minReady := entry.state.minReady
		spec := entry.state.spec
		for _, inst := range entry.state.instances {
			if inst.instance.State() != domain.InstanceStateReady {
				continue
			}

			switch spec.Strategy {
			case domain.StrategyPersistent:
				continue
			case domain.StrategySingleton:
				continue
			case domain.StrategyStateful:
				if entry.state.hasActiveBindingsForInstanceLocked(inst) {
					continue
				}
				// Fall through to idle check
			case domain.StrategyStateless:
				// Fall through to idle check
			}

			if readyCount <= minReady {
				continue
			}
			idleFor := now.Sub(inst.instance.LastActive())
			// When minReady=0 (on-demand servers with no activation), reap immediately
			// regardless of IdleSeconds to clean up after bootstrap/temporary usage.
			if minReady == 0 || idleFor >= spec.IdleDuration() {
				inst.instance.SetState(domain.InstanceStateDraining)
				s.logger.Info("idle reap",
					telemetry.EventField(telemetry.EventIdleReap),
					telemetry.ServerTypeField(entry.specKey),
					telemetry.InstanceIDField(inst.instance.ID()),
					telemetry.StateField(string(inst.instance.State())),
					telemetry.DurationField(idleFor),
				)
				candidates = append(candidates, stopCandidate{
					specKey: entry.specKey,
					state:   entry.state,
					inst:    inst,
					reason:  "idle timeout",
				})
				readyCount--
			}
		}
		entry.state.mu.Unlock()
	}

	for _, candidate := range candidates {
		err := s.stopInstance(context.Background(), candidate.state.spec, candidate.inst.instance, candidate.reason)
		s.observeInstanceStop(candidate.state.spec.Name, err)
		s.recordInstanceStop(candidate.state)
		candidate.state.mu.Lock()
		candidate.state.removeInstanceLocked(candidate.inst)
		candidate.state.mu.Unlock()
		s.observePoolStats(candidate.state)
	}

	// Reap stale sticky bindings for stateful strategies
	s.reapStaleBindings()
}

func (s *BasicScheduler) reapStaleBindings() {
	now := time.Now()

	for _, entry := range s.snapshotPools() {
		if entry.state.spec.Strategy != domain.StrategyStateful {
			continue
		}

		ttl := time.Duration(entry.state.spec.SessionTTLSeconds) * time.Second
		if ttl <= 0 {
			continue
		}

		entry.state.mu.Lock()
		for key, binding := range entry.state.sticky {
			if now.Sub(binding.lastAccess) > ttl {
				s.logger.Info("binding expired",
					telemetry.EventField("binding_expired"),
					telemetry.ServerTypeField(entry.specKey),
					zap.String("routingKey", key),
					telemetry.DurationField(now.Sub(binding.lastAccess)),
				)
				if binding.inst != nil && binding.inst.instance != nil {
					binding.inst.instance.SetStickyKey("")
				}
				delete(entry.state.sticky, key)
			}
		}
		if len(entry.state.sticky) == 0 {
			entry.state.sticky = nil
		}
		entry.state.mu.Unlock()
	}
}

func (s *BasicScheduler) probeInstances() {
	if s.probe == nil {
		return
	}

	var candidates []stopCandidate
	var checks []stopCandidate

	for _, entry := range s.snapshotPools() {
		entry.state.mu.Lock()
		for _, inst := range entry.state.instances {
			if !isRoutable(inst.instance.State()) {
				continue
			}
			checks = append(checks, stopCandidate{
				specKey: entry.specKey,
				state:   entry.state,
				inst:    inst,
				reason:  "ping failure",
			})
		}
		entry.state.mu.Unlock()
	}

	for _, candidate := range checks {
		if err := s.probe.Ping(context.Background(), candidate.inst.instance.Conn()); err != nil {
			s.logger.Warn("ping failed",
				telemetry.EventField(telemetry.EventPingFailure),
				telemetry.ServerTypeField(candidate.specKey),
				telemetry.InstanceIDField(candidate.inst.instance.ID()),
				telemetry.StateField(string(candidate.inst.instance.State())),
				zap.Error(err),
			)
			candidates = append(candidates, candidate)
			continue
		}

		candidate.state.mu.Lock()
		candidate.inst.instance.SetLastHeartbeatAt(time.Now())
		candidate.state.mu.Unlock()
	}

	for _, candidate := range candidates {
		candidate.state.mu.Lock()
		candidate.inst.instance.SetState(domain.InstanceStateFailed)
		candidate.state.mu.Unlock()

		err := s.stopInstance(context.Background(), candidate.state.spec, candidate.inst.instance, candidate.reason)
		s.observeInstanceStop(candidate.state.spec.Name, err)
		s.recordInstanceStop(candidate.state)
		candidate.state.mu.Lock()
		candidate.state.removeInstanceLocked(candidate.inst)
		candidate.state.mu.Unlock()
		s.observePoolStats(candidate.state)
	}
}
