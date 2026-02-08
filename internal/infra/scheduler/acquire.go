package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mcpv/internal/domain"
)

// Acquire obtains an instance for the given spec and routing key.
func (s *BasicScheduler) Acquire(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, ok := s.specForKey(specKey)
	if !ok {
		return nil, wrapSchedulerError("scheduler acquire", ErrUnknownSpecKey)
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
			serverType := state.spec.Name
			state.mu.Unlock()
			s.observePoolAcquireFailure(serverType, acquireErr)
			return nil, wrapSchedulerError("scheduler acquire", acquireErr)
		}

		if state.startInFlight {
			waitStart := time.Now()
			err := state.waitForSignalLocked(ctx)
			waitDuration := time.Since(waitStart)
			waitOutcome := domain.PoolWaitOutcomeSignaled
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					waitOutcome = domain.PoolWaitOutcomeTimeout
				} else {
					waitOutcome = domain.PoolWaitOutcomeCanceled
				}
			}
			serverType := state.spec.Name
			state.mu.Unlock()
			s.observePoolWait(serverType, waitDuration, waitOutcome)
			if err != nil {
				return nil, wrapSchedulerError("scheduler acquire", err)
			}
			continue
		}

		if state.spec.Strategy == domain.StrategySingleton && len(state.instances) > 0 {
			waitStart := time.Now()
			err := state.waitForSignalLocked(ctx)
			waitDuration := time.Since(waitStart)
			waitOutcome := domain.PoolWaitOutcomeSignaled
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					waitOutcome = domain.PoolWaitOutcomeTimeout
				} else {
					waitOutcome = domain.PoolWaitOutcomeCanceled
				}
			}
			serverType := state.spec.Name
			state.mu.Unlock()
			s.observePoolWait(serverType, waitDuration, waitOutcome)
			if err != nil {
				return nil, wrapSchedulerError("scheduler acquire", err)
			}
			continue
		}

		startGen := state.generation
		state.startInFlight = true
		startCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		state.startCancel = cancel
		// Reserve a starting slot: protected by lock for atomicity.
		// Actual StartInstance call happens outside lock to avoid blocking.
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
				// Release reservation atomically under lock.
				// This ensures no race with concurrent capacity checks.
				state.mu.Lock()
				if err != nil || r != nil {
					state.startInFlight = false
				}
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
			s.observeInstanceStartCause(ctx, state.spec.Name)
			newInst, err = s.lifecycle.StartInstance(startCtx, specKey, state.spec)
			s.observeInstanceStart(state.spec.Name, started, err)
			if err == nil {
				s.applyStartCause(ctx, newInst, started)
			}
		}()

		if err != nil {
			state.mu.Lock()
			state.startInFlight = false
			state.signalWaiterLocked()
			state.mu.Unlock()
			s.observePoolAcquireFailure(state.spec.Name, err)
			return nil, wrapSchedulerError("scheduler acquire", fmt.Errorf("start instance: %w", err))
		}
		tracked := &trackedInstance{instance: newInst}
		state.mu.Lock()
		if state.generation != startGen {
			state.startInFlight = false
			state.signalWaiterLocked()
			state.mu.Unlock()
			stopErr := s.stopInstance(context.Background(), state.spec, newInst, "start superseded")
			s.observeInstanceStop(state.spec.Name, stopErr)
			s.recordInstanceStop(state)
			s.observePoolAcquireFailure(state.spec.Name, ErrNoCapacity)
			return nil, wrapSchedulerError("scheduler acquire", ErrNoCapacity)
		}

		// For singleton, check if we already have an instance
		if state.spec.Strategy == domain.StrategySingleton && len(state.instances) > 0 {
			state.startInFlight = false
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
			s.observePoolAcquireFailure(state.spec.Name, err)
			return nil, wrapSchedulerError("scheduler acquire", ErrNoCapacity)
		}

		state.instances = append(state.instances, tracked)
		if state.spec.Strategy == domain.StrategyStateful && routingKey != "" {
			state.bindStickyLocked(routingKey, tracked)
		}
		instance := state.markBusyLocked(tracked)
		state.startInFlight = false
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
		return nil, wrapSchedulerError("scheduler acquire ready", err)
	}
	spec, ok := s.specForKey(specKey)
	if !ok {
		return nil, wrapSchedulerError("scheduler acquire ready", ErrUnknownSpecKey)
	}

	state := s.getPool(specKey, spec)
	state.mu.Lock()
	inst, err := state.acquireReadyLocked(routingKey)
	state.mu.Unlock()
	if err == nil {
		s.observePoolStats(state)
		return inst, nil
	}
	s.observePoolAcquireFailure(state.spec.Name, err)
	return inst, wrapSchedulerError("scheduler acquire ready", err)
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
