package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry/diagnostics"
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
		s.recordAcquireFailureLocked(state, acquireErr)
		if acquireErr == ErrStickyBusy {
			serverType := state.spec.Name
			state.mu.Unlock()
			s.observePoolAcquireFailure(serverType, acquireErr)
			s.recordAcquireFailureEvent(state, routingKey, acquireErr)
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
				s.recordAcquireFailure(state, err)
				s.recordAcquireFailureEvent(state, routingKey, err)
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
				s.recordAcquireFailure(state, err)
				s.recordAcquireFailureEvent(state, routingKey, err)
				return nil, wrapSchedulerError("scheduler acquire", err)
			}
			continue
		}

		startGen := state.generation
		state.startInFlight = true
		startCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		startCtx, _ = diagnostics.EnsureAttemptID(startCtx, specKey, time.Now())
		state.startCancel = cancel
		// Reserve a starting slot: protected by lock for atomicity.
		// Actual StartInstance call happens outside lock to avoid blocking.
		state.starting++
		state.lastStartAttemptAt = time.Now()
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
				} else {
					state.lastStartError = err.Error()
					state.lastStartErrorAt = time.Now()
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
			s.recordAcquireFailure(state, err)
			s.recordAcquireFailureEvent(state, routingKey, err)
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
			s.recordAcquireFailure(state, ErrNoCapacity)
			s.recordAcquireFailureEvent(state, routingKey, ErrNoCapacity)
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
			s.recordAcquireFailure(state, err)
			s.recordAcquireFailureEvent(state, routingKey, err)
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
	s.recordAcquireFailure(state, err)
	s.recordAcquireFailureEvent(state, routingKey, err)
	return inst, wrapSchedulerError("scheduler acquire ready", err)
}

func (s *BasicScheduler) recordAcquireFailureLocked(state *poolState, err error) {
	if state == nil || err == nil {
		return
	}
	state.lastAcquireError = err.Error()
	state.lastAcquireErrorAt = time.Now()
	if reason, ok := classifyAcquireFailure(err); ok {
		state.lastAcquireReason = reason
	} else {
		state.lastAcquireReason = ""
	}
}

func (s *BasicScheduler) recordAcquireFailure(state *poolState, err error) {
	if state == nil || err == nil {
		return
	}
	state.mu.Lock()
	s.recordAcquireFailureLocked(state, err)
	state.mu.Unlock()
}

func (s *BasicScheduler) recordAcquireFailureEvent(state *poolState, routingKey string, err error) {
	if state == nil || err == nil || s.diag == nil {
		return
	}
	state.mu.Lock()
	serverName := state.spec.Name
	specKey := state.specKey
	maxConcurrent := state.spec.MaxConcurrent
	minReady := state.minReady
	active := len(state.instances)
	starting := state.starting
	waiters := state.waiters
	startInFlight := state.startInFlight
	strategy := string(state.spec.Strategy)
	busyCount := 0
	for _, inst := range state.instances {
		busyCount += inst.instance.BusyCount()
	}
	state.mu.Unlock()

	reason, _ := classifyAcquireFailure(err)
	attrs := map[string]string{
		"reason":           string(reason),
		"routingKey":       routingKey,
		"routingKeyActive": strconv.FormatBool(routingKey != ""),
		"strategy":         strategy,
		"minReady":         strconv.Itoa(minReady),
		"maxConcurrent":    strconv.Itoa(maxConcurrent),
		"activeInstances":  strconv.Itoa(active),
		"busyCount":        strconv.Itoa(busyCount),
		"starting":         strconv.Itoa(starting),
		"waiters":          strconv.Itoa(waiters),
		"startInFlight":    strconv.FormatBool(startInFlight),
	}

	s.diag.Record(diagnostics.Event{
		SpecKey:    specKey,
		ServerName: serverName,
		Step:       diagnostics.StepAcquireFailure,
		Phase:      diagnostics.PhaseError,
		Timestamp:  time.Now(),
		Error:      err.Error(),
		Attributes: attrs,
	})
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

	if triggerDrain != nil {
		triggerDrain.closeDrainDone()
	}
	return nil
}
