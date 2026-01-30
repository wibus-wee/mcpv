package scheduler

import (
	"context"
	"sync"
	"time"

	"mcpv/internal/domain"
)

func (s *poolState) acquireReadyLocked(routingKey string) (*domain.Instance, error) {
	switch s.spec.Strategy {
	case domain.StrategySingleton:
		// Singleton: return the single instance if available
		if len(s.instances) > 0 {
			inst := s.instances[0]
			if !isRoutable(inst.instance.State()) {
				return nil, domain.ErrNoReadyInstance
			}
			if inst.instance.BusyCount() >= s.spec.MaxConcurrent {
				return nil, ErrNoCapacity
			}
			return s.markBusyLocked(inst), nil
		}
		return nil, domain.ErrNoReadyInstance

	case domain.StrategyStateful:
		// Stateful: check sticky binding first
		if routingKey != "" {
			if binding := s.lookupStickyLocked(routingKey); binding != nil {
				if !isRoutable(binding.inst.instance.State()) {
					s.unbindStickyLocked(routingKey)
				} else {
					if binding.inst.instance.BusyCount() >= s.spec.MaxConcurrent {
						return nil, ErrStickyBusy
					}
					binding.lastAccess = time.Now()
					return s.markBusyLocked(binding.inst), nil
				}
			}
		}
		// Fall through to find available instance
		if inst := s.findReadyInstanceLocked(); inst != nil {
			return s.markBusyLocked(inst), nil
		}
		return nil, domain.ErrNoReadyInstance

	case domain.StrategyStateless, domain.StrategyPersistent:
		// Stateless/Persistent: least-loaded with round-robin tie-breaker
		if inst := s.findReadyInstanceLocked(); inst != nil {
			return s.markBusyLocked(inst), nil
		}
		return nil, domain.ErrNoReadyInstance

	default:
		// Unknown strategy, treat as stateless with least-loaded selection
		if inst := s.findReadyInstanceLocked(); inst != nil {
			return s.markBusyLocked(inst), nil
		}
		return nil, domain.ErrNoReadyInstance
	}
}

func (s *poolState) waitForSignalLocked(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if s.waitCond == nil {
		s.waitCond = sync.NewCond(&s.mu)
	}
	s.waiters++
	seq := s.signalSeq
	stop := context.AfterFunc(ctx, func() {
		s.mu.Lock()
		s.signalSeq++
		s.waitCond.Broadcast()
		s.mu.Unlock()
	})
	defer func() {
		stop()
		s.waiters--
	}()
	for seq == s.signalSeq && ctx.Err() == nil {
		s.waitCond.Wait()
	}
	return ctx.Err()
}

func (s *poolState) signalWaiterLocked() {
	if s.waitCond == nil || s.waiters == 0 {
		return
	}
	s.signalSeq++
	s.waitCond.Broadcast()
}

func (s *poolState) lookupStickyLocked(routingKey string) *stickyBinding {
	if s.sticky == nil {
		return nil
	}
	return s.sticky[routingKey]
}

func (s *poolState) bindStickyLocked(routingKey string, inst *trackedInstance) {
	if s.sticky == nil {
		s.sticky = make(map[string]*stickyBinding)
	}
	s.sticky[routingKey] = &stickyBinding{
		inst:       inst,
		lastAccess: time.Now(),
	}
	inst.instance.SetStickyKey(routingKey)
}

func (s *poolState) unbindStickyLocked(routingKey string) {
	if s.sticky == nil {
		return
	}
	delete(s.sticky, routingKey)
	if len(s.sticky) == 0 {
		s.sticky = nil
	}
}

func (s *poolState) findReadyInstanceLocked() *trackedInstance {
	list := s.instances
	if len(list) == 0 {
		return nil
	}
	s.rrIndex %= len(list)

	bestIdx := -1
	bestBusy := 0

	start := s.rrIndex
	for i := 0; i < len(list); i++ {
		idx := (start + i) % len(list)
		inst := list[idx]
		if inst.instance.BusyCount() >= s.spec.MaxConcurrent {
			continue
		}
		if !isRoutable(inst.instance.State()) {
			continue
		}
		busy := inst.instance.BusyCount()
		if bestIdx == -1 || busy < bestBusy {
			bestIdx = idx
			bestBusy = busy
			if bestBusy == 0 {
				break
			}
		}
	}
	if bestIdx == -1 {
		return nil
	}
	s.rrIndex = (bestIdx + 1) % len(list)
	return list[bestIdx]
}

func (s *poolState) markBusyLocked(inst *trackedInstance) *domain.Instance {
	inst.instance.IncBusyCount()
	inst.instance.SetState(domain.InstanceStateBusy)
	inst.instance.SetLastActive(time.Now())
	return inst.instance
}

// hasActiveBindingsForInstanceLocked checks if an instance has any active sticky bindings.
func (s *poolState) hasActiveBindingsForInstanceLocked(inst *trackedInstance) bool {
	if s.sticky == nil {
		return false
	}
	for _, binding := range s.sticky {
		if binding.inst == inst {
			return true
		}
	}
	return false
}

func (s *poolState) removeInstanceLocked(inst *trackedInstance) int {
	list := s.instances
	if len(list) == 0 {
		return 0
	}

	out := list[:0]
	for _, candidate := range list {
		if candidate != inst {
			out = append(out, candidate)
		}
	}
	if len(out) == 0 {
		s.instances = nil
		s.rrIndex = 0
	} else {
		s.instances = out
		s.rrIndex %= len(s.instances)
	}

	if s.sticky != nil {
		for key, binding := range s.sticky {
			if binding.inst == inst {
				delete(s.sticky, key)
			}
		}
		if len(s.sticky) == 0 {
			s.sticky = nil
		}
	}
	if s.instances == nil {
		return 0
	}
	return len(s.instances)
}

func (s *poolState) countReadyLocked() int {
	count := 0
	for _, inst := range s.instances {
		if inst.instance.State() == domain.InstanceStateReady {
			count++
		}
	}
	return count
}

func (s *poolState) findDrainingByIDLocked(id string) *trackedInstance {
	for _, inst := range s.draining {
		if inst.instance.ID() == id {
			return inst
		}
	}
	return nil
}

func (s *poolState) removeDrainingLocked(inst *trackedInstance) {
	list := s.draining
	if len(list) == 0 {
		return
	}
	out := list[:0]
	for _, candidate := range list {
		if candidate != inst {
			out = append(out, candidate)
		}
	}
	if len(out) == 0 {
		s.draining = nil
	} else {
		s.draining = out
	}
}

func isRoutable(state domain.InstanceState) bool {
	return state == domain.InstanceStateReady || state == domain.InstanceStateBusy
}
