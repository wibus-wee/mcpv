package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mcpv/internal/domain"
)

// SetDesiredMinReady ensures a minimum ready instance count for the spec.
func (s *BasicScheduler) SetDesiredMinReady(ctx context.Context, specKey string, minReady int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, ok := s.specForKey(specKey)
	if !ok {
		return wrapSchedulerError("scheduler min ready", ErrUnknownSpecKey)
	}

	state := s.getPool(specKey, spec)
	cause, hasCause := domain.StartCauseFromContext(ctx)
	state.mu.Lock()
	state.minReady = minReady
	if hasCause {
		s.applyStartCauseLocked(state, cause, time.Now())
	}
	// state.starting acts as a reservation counter for in-flight starts.
	// Both increment and decrement are protected by state.mu to ensure
	// consistent capacity calculations across concurrent calls.
	active := len(state.instances) + state.starting
	toStart := minReady - active
	if toStart <= 0 {
		state.mu.Unlock()
		return nil
	}
	state.starting += toStart // Reserve starting slots under lock
	state.mu.Unlock()

	var firstErr error
	for i := 0; i < toStart; i++ {
		state.mu.Lock()
		startGen := state.generation
		state.mu.Unlock()

		started := time.Now()
		var (
			inst *domain.Instance
			err  error
		)
		func() {
			defer func() {
				r := recover()
				if r != nil {
					err = fmt.Errorf("start instance panic: %v", r)
				}
				// Release reservation: decrement must be done under lock
				// to maintain consistency with concurrent capacity checks.
				state.mu.Lock()
				state.starting--
				if err == nil {
					state.startCount++
				}
				state.mu.Unlock()
				if r != nil {
					panic(r)
				}
			}()
			s.observeInstanceStartCause(ctx, state.spec.Name)
			inst, err = s.lifecycle.StartInstance(ctx, specKey, state.spec)
			s.observeInstanceStart(state.spec.Name, started, err)
			if err == nil {
				s.applyStartCause(ctx, inst, started)
			}
		}()
		state.mu.Lock()
		if err == nil {
			if state.generation != startGen {
				state.mu.Unlock()
				stopErr := s.stopInstance(context.Background(), state.spec, inst, "start superseded")
				s.observeInstanceStop(state.spec.Name, stopErr)
				s.recordInstanceStop(state)
				continue
			}
			if state.minReady == 0 {
				state.mu.Unlock()
				stopErr := s.stopInstance(context.Background(), state.spec, inst, "min ready dropped")
				s.observeInstanceStop(state.spec.Name, stopErr)
				s.recordInstanceStop(state)
				continue
			}
			state.instances = append(state.instances, &trackedInstance{instance: inst})
			state.signalWaiterLocked()
			state.mu.Unlock()
			s.observePoolStats(state)
			continue
		}
		state.mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}
	return wrapSchedulerError("scheduler min ready", firstErr)
}

// StopSpec stops instances for the given spec key.
func (s *BasicScheduler) StopSpec(ctx context.Context, specKey, reason string) error {
	spec, ok := s.specForKey(specKey)
	state := s.poolByKey(specKey)
	if !ok && state == nil {
		return wrapSchedulerError("scheduler stop spec", ErrUnknownSpecKey)
	}
	if state == nil {
		state = s.getPool(specKey, spec)
	}
	if !ok {
		spec = state.spec
	}
	state.mu.Lock()
	state.minReady = 0
	state.generation++
	startCancel := state.startCancel
	state.startCancel = nil

	var immediate []*trackedInstance
	var deferred []*trackedInstance

	for _, inst := range state.instances {
		if inst.instance.BusyCount() > 0 {
			inst.instance.SetState(domain.InstanceStateDraining)
			deferred = append(deferred, inst)
		} else {
			inst.instance.SetState(domain.InstanceStateDraining)
			immediate = append(immediate, inst)
		}
	}
	state.instances = nil
	state.draining = append(state.draining, deferred...)
	state.sticky = nil
	state.mu.Unlock()

	if startCancel != nil {
		startCancel()
	}

	// Stop idle instances in parallel
	if len(immediate) > 0 {
		var wg sync.WaitGroup
		wg.Add(len(immediate))
		for _, inst := range immediate {
			go func(inst *trackedInstance) {
				defer wg.Done()
				err := s.stopInstance(ctx, spec, inst.instance, reason)
				s.observeInstanceStop(spec.Name, err)
				s.recordInstanceStop(state)
			}(inst)
		}
		wg.Wait()
	}

	drainTimeout := spec.DrainTimeout()

	for _, inst := range deferred {
		s.startDrain(specKey, inst, drainTimeout, reason)
	}

	s.observePoolStats(state)
	return nil
}

func stopTimeout(spec domain.ServerSpec) time.Duration {
	timeout := time.Duration(spec.DrainTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(domain.DefaultDrainTimeoutSeconds) * time.Second
	}
	return timeout
}

func (s *BasicScheduler) stopInstance(ctx context.Context, spec domain.ServerSpec, inst *domain.Instance, reason string) error {
	if ctx == nil {
		ctx = context.Background()
	} else if ctx.Err() != nil {
		ctx = context.WithoutCancel(ctx)
	}
	if _, ok := ctx.Deadline(); ok {
		return s.lifecycle.StopInstance(ctx, inst, reason)
	}
	stopCtx, cancel := context.WithTimeout(ctx, stopTimeout(spec))
	defer cancel()
	return s.lifecycle.StopInstance(stopCtx, inst, reason)
}

// StopAll terminates all known instances for graceful shutdown.
func (s *BasicScheduler) StopAll(ctx context.Context) {
	var candidates []stopCandidate

	entries := s.snapshotPools()
	for _, entry := range entries {
		entry.state.mu.Lock()
		entry.state.generation++
		startCancel := entry.state.startCancel
		entry.state.startCancel = nil
		entry.state.mu.Unlock()
		if startCancel != nil {
			startCancel()
		}
	}
	for _, entry := range entries {
		entry.state.mu.Lock()
		for _, inst := range entry.state.instances {
			candidates = append(candidates, stopCandidate{
				specKey: entry.specKey,
				state:   entry.state,
				inst:    inst,
				reason:  "shutdown",
			})
		}
		entry.state.mu.Unlock()
	}

	for _, candidate := range candidates {
		err := s.stopInstance(ctx, candidate.state.spec, candidate.inst.instance, candidate.reason)
		s.observeInstanceStop(candidate.state.spec.Name, err)
		s.recordInstanceStop(candidate.state)
	}

	for _, entry := range entries {
		s.observePoolStats(entry.state)
	}
	s.poolsMu.Lock()
	s.pools = make(map[string]*poolState)
	s.poolsMu.Unlock()
}
