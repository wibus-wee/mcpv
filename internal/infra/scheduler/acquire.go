package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mcpd/internal/domain"
)

// Acquire obtains an instance for the given spec and routing key.
func (s *BasicScheduler) Acquire(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, ok := s.specForKey(specKey)
	if !ok {
		return nil, ErrUnknownSpecKey
	}

	state := s.getPool(specKey, spec)
	for {
		state.mu.Lock()
		inst, acquireErr := state.acquireReadyLocked(routingKey)
		if acquireErr == nil {
			state.mu.Unlock()
			return inst, nil
		}
		if acquireErr == ErrStickyBusy {
			state.mu.Unlock()
			return nil, acquireErr
		}

		if state.startInFlight {
			if err := state.waitForSignalLocked(ctx); err != nil {
				state.mu.Unlock()
				return nil, err
			}
			state.mu.Unlock()
			continue
		}

		if state.spec.Strategy == domain.StrategySingleton && len(state.instances) > 0 {
			if err := state.waitForSignalLocked(ctx); err != nil {
				state.mu.Unlock()
				return nil, err
			}
			state.mu.Unlock()
			continue
		}

		startGen := state.generation
		state.startInFlight = true
		startCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		state.startCancel = cancel
		state.starting++
		state.mu.Unlock()

		started := time.Now()
		var (
			newInst *domain.Instance
			err     error
		)
		func() {
			defer func() {
				r := recover()
				if r != nil {
					err = fmt.Errorf("start instance panic: %v", r)
				}
				state.mu.Lock()
				state.startInFlight = false
				startCancel := state.startCancel
				state.startCancel = nil
				state.starting--
				if err == nil {
					state.startCount++
				}
				if startCancel != nil {
					startCancel()
				}
				state.mu.Unlock()
				if r != nil {
					panic(r)
				}
			}()
			newInst, err = s.lifecycle.StartInstance(startCtx, specKey, state.spec)
			s.observeInstanceStart(state.spec.Name, started, err)
			if err == nil {
				s.applyStartCause(ctx, newInst, started)
			}
		}()

		if err != nil {
			state.mu.Lock()
			state.signalWaiterLocked()
			state.mu.Unlock()
			return nil, fmt.Errorf("start instance: %w", err)
		}
		tracked := &trackedInstance{instance: newInst}
		state.mu.Lock()
		if state.generation != startGen {
			state.signalWaiterLocked()
			state.mu.Unlock()
			stopErr := s.stopInstance(context.Background(), state.spec, newInst, "start superseded")
			s.observeInstanceStop(state.spec.Name, stopErr)
			s.recordInstanceStop(state)
			return nil, ErrNoCapacity
		}

		// For singleton, check if we already have an instance
		if state.spec.Strategy == domain.StrategySingleton && len(state.instances) > 0 {
			state.signalWaiterLocked()
			state.mu.Unlock()
			stopErr := s.stopInstance(context.Background(), state.spec, newInst, "singleton already exists")
			s.observeInstanceStop(state.spec.Name, stopErr)
			s.recordInstanceStop(state)
			// Try to acquire the existing singleton
			inst, err := state.acquireReadyLocked(routingKey)
			state.mu.Unlock()
			if err == nil {
				return inst, nil
			}
			return nil, ErrNoCapacity
		}

		state.instances = append(state.instances, tracked)
		if state.spec.Strategy == domain.StrategyStateful && routingKey != "" {
			state.bindStickyLocked(routingKey, tracked)
		}
		instance := state.markBusyLocked(tracked)
		state.signalWaiterLocked()
		state.mu.Unlock()
		s.observePoolStats(state)

		return instance, nil
	}
}

// AcquireReady returns a ready instance without starting new ones.
func (s *BasicScheduler) AcquireReady(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	spec, ok := s.specForKey(specKey)
	if !ok {
		return nil, ErrUnknownSpecKey
	}

	state := s.getPool(specKey, spec)
	state.mu.Lock()
	inst, err := state.acquireReadyLocked(routingKey)
	state.mu.Unlock()
	if err == nil {
		s.observePoolStats(state)
	}
	return inst, err
}

// Release marks an instance as idle and updates pool state.
func (s *BasicScheduler) Release(_ context.Context, instance *domain.Instance) error {
	if instance == nil {
		return errors.New("instance is nil")
	}

	specKey := instance.SpecKey()
	if specKey == "" {
		return errors.New("instance spec key is empty")
	}
	state := s.getPool(specKey, instance.Spec())
	state.mu.Lock()

	if instance.BusyCount() > 0 {
		instance.DecBusyCount()
	}
	instance.SetLastActive(time.Now())

	var triggerDrain *trackedInstance
	if instance.BusyCount() == 0 {
		switch instance.State() {
		case domain.InstanceStateBusy:
			instance.SetState(domain.InstanceStateReady)
			state.signalWaiterLocked()
		case domain.InstanceStateDraining:
			triggerDrain = state.findDrainingByIDLocked(instance.ID())
		case domain.InstanceStateReady,
			domain.InstanceStateStarting,
			domain.InstanceStateInitializing,
			domain.InstanceStateHandshaking,
			domain.InstanceStateStopped,
			domain.InstanceStateFailed:
		}
	}
	state.mu.Unlock()
	s.observePoolStats(state)

	if triggerDrain != nil && triggerDrain.drainDone != nil {
		select {
		case <-triggerDrain.drainDone:
		default:
			close(triggerDrain.drainDone)
		}
	}
	return nil
}
