package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/infra/telemetry"

	"mcpd/internal/domain"
)

var (
	ErrUnknownSpecKey = errors.New("unknown spec key")
	ErrNoCapacity     = errors.New("no capacity available")
	ErrStickyBusy     = errors.New("sticky instance at capacity")
	ErrNotImplemented = errors.New("scheduler not implemented")
)

type SchedulerOptions struct {
	Probe        domain.HealthProbe
	PingInterval time.Duration
	Logger       *zap.Logger
	Metrics      domain.Metrics
	Health       *telemetry.HealthTracker
}

type BasicScheduler struct {
	lifecycle domain.Lifecycle
	specs     map[string]domain.ServerSpec

	poolsMu sync.RWMutex
	pools   map[string]*poolState

	probe   domain.HealthProbe
	logger  *zap.Logger
	metrics domain.Metrics
	health  *telemetry.HealthTracker

	mu         sync.Mutex
	idleTicker *time.Ticker
	stopIdle   chan struct{}
	pingTicker *time.Ticker
	stopPing   chan struct{}

	idleBeat *telemetry.Heartbeat
	pingBeat *telemetry.Heartbeat
}

type trackedInstance struct {
	instance *domain.Instance
}

type poolState struct {
	mu        sync.Mutex
	spec      domain.ServerSpec
	minReady  int
	starting  int
	startCh   chan struct{}
	instances []*trackedInstance
	sticky    map[string]*trackedInstance
}

type stopCandidate struct {
	specKey string
	state   *poolState
	inst    *trackedInstance
	reason  string
}

type poolEntry struct {
	specKey string
	state   *poolState
}

func NewBasicScheduler(lifecycle domain.Lifecycle, specs map[string]domain.ServerSpec, opts SchedulerOptions) (*BasicScheduler, error) {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BasicScheduler{
		lifecycle: lifecycle,
		specs:     specs,
		pools:     make(map[string]*poolState),
		probe:     opts.Probe,
		logger:    logger.Named("scheduler"),
		metrics:   opts.Metrics,
		health:    opts.Health,
		stopIdle:  make(chan struct{}),
		stopPing:  make(chan struct{}),
	}, nil
}

func (s *BasicScheduler) Acquire(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	spec, ok := s.specs[specKey]
	if !ok {
		return nil, ErrUnknownSpecKey
	}

	state := s.getPool(specKey, spec)
	for {
		state.mu.Lock()
		if state.spec.Sticky && routingKey != "" {
			if inst := state.lookupStickyLocked(routingKey); inst != nil {
				if !isRoutable(inst.instance.State) {
					state.unbindStickyLocked(routingKey)
				} else {
					if inst.instance.BusyCount >= state.spec.MaxConcurrent {
						state.mu.Unlock()
						return nil, ErrStickyBusy
					}
					instance := state.markBusyLocked(inst)
					state.mu.Unlock()
					return instance, nil
				}
			}
		}

		if inst := state.findReadyInstanceLocked(); inst != nil {
			instance := state.markBusyLocked(inst)
			state.mu.Unlock()
			return instance, nil
		}

		if state.startCh != nil {
			waitCh := state.startCh
			state.mu.Unlock()
			select {
			case <-waitCh:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		state.startCh = make(chan struct{})
		state.starting++
		state.mu.Unlock()

		started := time.Now()
		newInst, err := s.lifecycle.StartInstance(ctx, state.spec)
		s.observeInstanceStart(specKey, started, err)

		state.mu.Lock()
		waitCh := state.startCh
		state.startCh = nil
		state.starting--
		if err != nil {
			state.mu.Unlock()
			if waitCh != nil {
				close(waitCh)
			}
			return nil, fmt.Errorf("start instance: %w", err)
		}
		tracked := &trackedInstance{instance: newInst}
		state.instances = append(state.instances, tracked)
		if state.spec.Sticky && routingKey != "" {
			state.bindStickyLocked(routingKey, tracked)
		}
		instance := state.markBusyLocked(tracked)
		activeCount := len(state.instances)
		state.mu.Unlock()
		if waitCh != nil {
			close(waitCh)
		}
		s.observeActiveInstances(specKey, activeCount)

		return instance, nil
	}
}

func (s *BasicScheduler) Release(ctx context.Context, instance *domain.Instance) error {
	if instance == nil {
		return errors.New("instance is nil")
	}

	state := s.getPool(instance.Spec.Name, instance.Spec)
	state.mu.Lock()
	defer state.mu.Unlock()

	if instance.BusyCount > 0 {
		instance.BusyCount--
	}
	instance.LastActive = time.Now()
	if instance.BusyCount == 0 && instance.State == domain.InstanceStateBusy {
		instance.State = domain.InstanceStateReady
	}
	return nil
}

func (s *BasicScheduler) SetDesiredMinReady(ctx context.Context, specKey string, minReady int) error {
	spec, ok := s.specs[specKey]
	if !ok {
		return ErrUnknownSpecKey
	}

	state := s.getPool(specKey, spec)
	state.mu.Lock()
	state.minReady = minReady
	active := len(state.instances) + state.starting
	toStart := minReady - active
	if toStart <= 0 {
		state.mu.Unlock()
		return nil
	}
	state.starting += toStart
	state.mu.Unlock()

	var firstErr error
	for i := 0; i < toStart; i++ {
		started := time.Now()
		inst, err := s.lifecycle.StartInstance(ctx, state.spec)
		s.observeInstanceStart(specKey, started, err)
		state.mu.Lock()
		state.starting--
		if err == nil {
			if state.minReady == 0 {
				state.mu.Unlock()
				stopErr := s.lifecycle.StopInstance(context.Background(), inst, "min ready dropped")
				s.observeInstanceStop(specKey, stopErr)
				continue
			}
			state.instances = append(state.instances, &trackedInstance{instance: inst})
			activeCount := len(state.instances)
			state.mu.Unlock()
			s.observeActiveInstances(specKey, activeCount)
			continue
		}
		state.mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *BasicScheduler) StopSpec(ctx context.Context, specKey, reason string) error {
	spec, ok := s.specs[specKey]
	if !ok {
		return ErrUnknownSpecKey
	}

	state := s.getPool(specKey, spec)
	state.mu.Lock()
	state.minReady = 0
	instances := append([]*trackedInstance(nil), state.instances...)
	state.instances = nil
	state.sticky = nil
	state.mu.Unlock()

	var firstErr error
	for _, inst := range instances {
		err := s.lifecycle.StopInstance(ctx, inst.instance, reason)
		s.observeInstanceStop(specKey, err)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.observeActiveInstances(specKey, 0)
	return firstErr
}

func (s *BasicScheduler) getPool(specKey string, spec domain.ServerSpec) *poolState {
	s.poolsMu.RLock()
	state := s.pools[specKey]
	s.poolsMu.RUnlock()

	if state != nil {
		return state
	}

	s.poolsMu.Lock()
	defer s.poolsMu.Unlock()
	state = s.pools[specKey]
	if state == nil {
		canonical := spec
		canonical.Name = specKey
		state = &poolState{spec: canonical}
		s.pools[specKey] = state
	}
	return state
}

func (s *BasicScheduler) snapshotPools() []poolEntry {
	s.poolsMu.RLock()
	defer s.poolsMu.RUnlock()

	entries := make([]poolEntry, 0, len(s.pools))
	for specKey, state := range s.pools {
		entries = append(entries, poolEntry{specKey: specKey, state: state})
	}
	return entries
}

func (s *poolState) lookupStickyLocked(routingKey string) *trackedInstance {
	if s.sticky == nil {
		return nil
	}
	return s.sticky[routingKey]
}

func (s *poolState) bindStickyLocked(routingKey string, inst *trackedInstance) {
	if s.sticky == nil {
		s.sticky = make(map[string]*trackedInstance)
	}
	s.sticky[routingKey] = inst
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
	for _, inst := range s.instances {
		if inst.instance.BusyCount >= s.spec.MaxConcurrent {
			continue
		}
		if !isRoutable(inst.instance.State) {
			continue
		}
		return inst
	}
	return nil
}

func (s *poolState) markBusyLocked(inst *trackedInstance) *domain.Instance {
	inst.instance.BusyCount++
	inst.instance.State = domain.InstanceStateBusy
	inst.instance.LastActive = time.Now()
	return inst.instance
}

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
	if s.stopIdle == nil {
		s.stopIdle = make(chan struct{})
	}
	if s.health != nil && s.idleBeat == nil {
		s.idleBeat = s.health.Register("scheduler.idle", interval*3)
	}
	s.idleTicker = time.NewTicker(interval)
	s.mu.Unlock()

	go func() {
		for {
			select {
			case <-s.idleTicker.C:
				if s.idleBeat != nil {
					s.idleBeat.Beat()
				}
				s.reapIdle()
			case <-s.stopIdle:
				return
			}
		}
	}()
}

func (s *BasicScheduler) StopIdleManager() {
	s.mu.Lock()
	if s.idleTicker != nil {
		s.idleTicker.Stop()
		s.idleTicker = nil
	}
	if s.idleBeat != nil {
		s.idleBeat.Stop()
		s.idleBeat = nil
	}
	if s.stopIdle != nil {
		close(s.stopIdle)
		s.stopIdle = nil
	}
	s.mu.Unlock()
}

func (s *BasicScheduler) StartPingManager(interval time.Duration) {
	if interval <= 0 || s.probe == nil {
		return
	}
	s.mu.Lock()
	if s.pingTicker != nil {
		s.mu.Unlock()
		return
	}
	if s.stopPing == nil {
		s.stopPing = make(chan struct{})
	}
	if s.health != nil && s.pingBeat == nil {
		s.pingBeat = s.health.Register("scheduler.ping", interval*3)
	}
	s.pingTicker = time.NewTicker(interval)
	s.mu.Unlock()

	go func() {
		for {
			select {
			case <-s.pingTicker.C:
				if s.pingBeat != nil {
					s.pingBeat.Beat()
				}
				s.probeInstances()
			case <-s.stopPing:
				return
			}
		}
	}()
}

func (s *BasicScheduler) StopPingManager() {
	s.mu.Lock()
	if s.pingTicker != nil {
		s.pingTicker.Stop()
		s.pingTicker = nil
	}
	if s.pingBeat != nil {
		s.pingBeat.Stop()
		s.pingBeat = nil
	}
	if s.stopPing != nil {
		close(s.stopPing)
		s.stopPing = nil
	}
	s.mu.Unlock()
}

func (s *BasicScheduler) reapIdle() {
	now := time.Now()
	var candidates []stopCandidate

	for _, entry := range s.snapshotPools() {
		spec := entry.state.spec
		entry.state.mu.Lock()
		readyCount := entry.state.countReadyLocked()
		minReady := entry.state.minReady

		for _, inst := range entry.state.instances {
			if inst.instance.State != domain.InstanceStateReady {
				continue
			}
			if spec.Persistent || spec.Sticky {
				continue
			}
			if readyCount <= minReady {
				continue
			}
			idleFor := now.Sub(inst.instance.LastActive)
			if idleFor >= time.Duration(spec.IdleSeconds)*time.Second {
				inst.instance.State = domain.InstanceStateDraining
				s.logger.Info("idle reap",
					telemetry.EventField(telemetry.EventIdleReap),
					telemetry.ServerTypeField(entry.specKey),
					telemetry.InstanceIDField(inst.instance.ID),
					telemetry.StateField(string(inst.instance.State)),
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
		err := s.lifecycle.StopInstance(context.Background(), candidate.inst.instance, candidate.reason)
		s.observeInstanceStop(candidate.specKey, err)
		candidate.state.mu.Lock()
		activeCount := candidate.state.removeInstanceLocked(candidate.inst)
		candidate.state.mu.Unlock()
		s.observeActiveInstances(candidate.specKey, activeCount)
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
			if !isRoutable(inst.instance.State) {
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
		if err := s.probe.Ping(context.Background(), candidate.inst.instance.Conn); err != nil {
			s.logger.Warn("ping failed",
				telemetry.EventField(telemetry.EventPingFailure),
				telemetry.ServerTypeField(candidate.specKey),
				telemetry.InstanceIDField(candidate.inst.instance.ID),
				telemetry.StateField(string(candidate.inst.instance.State)),
				zap.Error(err),
			)
			candidates = append(candidates, candidate)
		}
	}

	for _, candidate := range candidates {
		candidate.state.mu.Lock()
		candidate.inst.instance.State = domain.InstanceStateFailed
		candidate.state.mu.Unlock()

		err := s.lifecycle.StopInstance(context.Background(), candidate.inst.instance, candidate.reason)
		s.observeInstanceStop(candidate.specKey, err)
		candidate.state.mu.Lock()
		activeCount := candidate.state.removeInstanceLocked(candidate.inst)
		candidate.state.mu.Unlock()
		s.observeActiveInstances(candidate.specKey, activeCount)
	}
}

// StopAll terminates all known instances for graceful shutdown.
func (s *BasicScheduler) StopAll(ctx context.Context) {
	var candidates []stopCandidate

	entries := s.snapshotPools()
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
		err := s.lifecycle.StopInstance(ctx, candidate.inst.instance, candidate.reason)
		s.observeInstanceStop(candidate.specKey, err)
	}

	for _, entry := range entries {
		s.observeActiveInstances(entry.specKey, 0)
	}
	s.poolsMu.Lock()
	s.pools = make(map[string]*poolState)
	s.poolsMu.Unlock()
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
	} else {
		s.instances = out
	}

	if s.sticky != nil {
		for key, bound := range s.sticky {
			if bound == inst {
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
		if inst.instance.State == domain.InstanceStateReady {
			count++
		}
	}
	return count
}

func (s *BasicScheduler) observeInstanceStart(specKey string, start time.Time, err error) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveInstanceStart(specKey, time.Since(start), err)
}

func (s *BasicScheduler) observeInstanceStop(specKey string, err error) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveInstanceStop(specKey, err)
}

func (s *BasicScheduler) observeActiveInstances(specKey string, count int) {
	if s.metrics == nil {
		return
	}
	s.metrics.SetActiveInstances(specKey, count)
}

func isRoutable(state domain.InstanceState) bool {
	return state == domain.InstanceStateReady || state == domain.InstanceStateBusy
}
