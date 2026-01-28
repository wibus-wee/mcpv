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
	// ErrUnknownSpecKey indicates the spec key does not exist.
	ErrUnknownSpecKey = domain.ErrUnknownSpecKey
	// ErrNoCapacity indicates no instance capacity is available.
	ErrNoCapacity = errors.New("no capacity available")
	// ErrStickyBusy indicates a sticky instance is busy.
	ErrStickyBusy = errors.New("sticky instance at capacity")
	// ErrNotImplemented indicates the scheduler is not available.
	ErrNotImplemented = errors.New("scheduler not implemented")
)

// Options configures the basic scheduler.
type Options struct {
	Probe        domain.HealthProbe
	PingInterval time.Duration
	Logger       *zap.Logger
	Metrics      domain.Metrics
	Health       *telemetry.HealthTracker
}

// BasicScheduler orchestrates instance lifecycle and routing policies.
type BasicScheduler struct {
	lifecycle domain.Lifecycle
	specsMu   sync.RWMutex
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
	instance  *domain.Instance
	drainOnce sync.Once
	drainDone chan struct{}
}

type stickyBinding struct {
	inst       *trackedInstance
	lastAccess time.Time
}

type poolState struct {
	mu          sync.Mutex
	spec        domain.ServerSpec
	specKey     string
	minReady    int
	starting    int
	startCount  int
	stopCount   int
	startCh     chan struct{}
	startCancel context.CancelFunc
	generation  uint64
	instances   []*trackedInstance
	draining    []*trackedInstance
	sticky      map[string]*stickyBinding
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

// NewBasicScheduler constructs a BasicScheduler using the provided options.
func NewBasicScheduler(lifecycle domain.Lifecycle, specs map[string]domain.ServerSpec, opts Options) (*BasicScheduler, error) {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BasicScheduler{
		lifecycle: lifecycle,
		specs:     cloneSpecRegistry(specs),
		pools:     make(map[string]*poolState),
		probe:     opts.Probe,
		logger:    logger.Named("scheduler"),
		metrics:   opts.Metrics,
		health:    opts.Health,
		stopIdle:  make(chan struct{}),
		stopPing:  make(chan struct{}),
	}, nil
}

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
		inst, err := state.acquireReadyLocked(routingKey)
		if err == nil {
			state.mu.Unlock()
			return inst, nil
		}
		if err == ErrStickyBusy {
			state.mu.Unlock()
			return nil, err
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

		startGen := state.generation
		state.startCh = make(chan struct{})
		startCtx, cancel := context.WithCancel(ctx)
		state.startCancel = cancel
		state.starting++
		state.mu.Unlock()

		started := time.Now()
		newInst, err := s.lifecycle.StartInstance(startCtx, specKey, state.spec)
		s.observeInstanceStart(state.spec.Name, started, err)
		if err == nil {
			s.applyStartCause(ctx, newInst, started)
		}

		state.mu.Lock()
		waitCh := state.startCh
		state.startCh = nil
		startCancel := state.startCancel
		state.startCancel = nil
		state.starting--
		if err == nil {
			state.startCount++
		}
		if err != nil {
			state.mu.Unlock()
			if waitCh != nil {
				close(waitCh)
			}
			if startCancel != nil {
				startCancel()
			}
			return nil, fmt.Errorf("start instance: %w", err)
		}
		if startCancel != nil {
			startCancel()
		}
		tracked := &trackedInstance{instance: newInst}
		if state.generation != startGen {
			state.mu.Unlock()
			if waitCh != nil {
				close(waitCh)
			}
			stopErr := s.stopInstance(context.Background(), state.spec, newInst, "start superseded")
			s.observeInstanceStop(state.spec.Name, stopErr)
			s.recordInstanceStop(state)
			return nil, ErrNoCapacity
		}

		// For singleton, check if we already have an instance
		if state.spec.Strategy == domain.StrategySingleton && len(state.instances) > 0 {
			state.mu.Unlock()
			if waitCh != nil {
				close(waitCh)
			}
			stopErr := s.stopInstance(context.Background(), state.spec, newInst, "singleton already exists")
			s.observeInstanceStop(state.spec.Name, stopErr)
			s.recordInstanceStop(state)
			// Try to acquire the existing singleton
			state.mu.Lock()
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
		state.mu.Unlock()
		if waitCh != nil {
			close(waitCh)
		}
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

// SetDesiredMinReady ensures a minimum ready instance count for the spec.
func (s *BasicScheduler) SetDesiredMinReady(ctx context.Context, specKey string, minReady int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, ok := s.specForKey(specKey)
	if !ok {
		return ErrUnknownSpecKey
	}

	state := s.getPool(specKey, spec)
	cause, hasCause := domain.StartCauseFromContext(ctx)
	state.mu.Lock()
	state.minReady = minReady
	if hasCause {
		s.applyStartCauseLocked(state, cause, time.Now())
	}
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
		state.mu.Lock()
		startGen := state.generation
		state.mu.Unlock()

		started := time.Now()
		inst, err := s.lifecycle.StartInstance(ctx, specKey, state.spec)
		s.observeInstanceStart(state.spec.Name, started, err)
		if err == nil {
			s.applyStartCause(ctx, inst, started)
		}
		state.mu.Lock()
		state.starting--
		if err == nil {
			state.startCount++
		}
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
			state.mu.Unlock()
			s.observePoolStats(state)
			continue
		}
		state.mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// StopSpec stops instances for the given spec key.
func (s *BasicScheduler) StopSpec(ctx context.Context, specKey, reason string) error {
	spec, ok := s.specForKey(specKey)
	state := s.poolByKey(specKey)
	if !ok && state == nil {
		return ErrUnknownSpecKey
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

	for _, inst := range immediate {
		err := s.stopInstance(ctx, spec, inst.instance, reason)
		s.observeInstanceStop(spec.Name, err)
		s.recordInstanceStop(state)
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

// ApplyCatalogDiff updates scheduler state after catalog changes.
func (s *BasicScheduler) ApplyCatalogDiff(ctx context.Context, diff domain.CatalogDiff, registry map[string]domain.ServerSpec) error {
	if ctx == nil {
		ctx = context.Background()
	}
	specs := cloneSpecRegistry(registry)
	s.specsMu.Lock()
	s.specs = specs
	s.specsMu.Unlock()

	s.poolsMu.RLock()
	for specKey, state := range s.pools {
		spec, ok := specs[specKey]
		if !ok {
			continue
		}
		state.mu.Lock()
		state.spec = spec
		state.mu.Unlock()
	}
	s.poolsMu.RUnlock()

	stopSet := make(map[string]struct{})
	for _, key := range diff.RemovedSpecKeys {
		stopSet[key] = struct{}{}
	}
	for _, key := range diff.ReplacedSpecKeys {
		stopSet[key] = struct{}{}
	}

	for specKey := range stopSet {
		if err := s.StopSpec(ctx, specKey, "spec removed"); err != nil && !errors.Is(err, ErrUnknownSpecKey) {
			return err
		}
	}
	return nil
}

func (s *BasicScheduler) specForKey(specKey string) (domain.ServerSpec, bool) {
	s.specsMu.RLock()
	defer s.specsMu.RUnlock()
	spec, ok := s.specs[specKey]
	return spec, ok
}

func (s *BasicScheduler) poolByKey(specKey string) *poolState {
	s.poolsMu.RLock()
	state := s.pools[specKey]
	s.poolsMu.RUnlock()
	return state
}

func cloneSpecRegistry(specs map[string]domain.ServerSpec) map[string]domain.ServerSpec {
	if len(specs) == 0 {
		return map[string]domain.ServerSpec{}
	}
	clone := make(map[string]domain.ServerSpec, len(specs))
	for key, spec := range specs {
		clone[key] = spec
	}
	return clone
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
		// Use the spec as-is, preserving the original Name for display
		state = &poolState{
			spec:    spec,
			specKey: specKey,
		}
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
		// Stateless/Persistent: round-robin across available instances
		if inst := s.findReadyInstanceLocked(); inst != nil {
			return s.markBusyLocked(inst), nil
		}
		return nil, domain.ErrNoReadyInstance

	default:
		// Unknown strategy, treat as stateless
		if inst := s.findReadyInstanceLocked(); inst != nil {
			return s.markBusyLocked(inst), nil
		}
		return nil, domain.ErrNoReadyInstance
	}
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
	for _, inst := range s.instances {
		if inst.instance.BusyCount() >= s.spec.MaxConcurrent {
			continue
		}
		if !isRoutable(inst.instance.State()) {
			continue
		}
		return inst
	}
	return nil
}

func (s *poolState) markBusyLocked(inst *trackedInstance) *domain.Instance {
	inst.instance.IncBusyCount()
	inst.instance.SetState(domain.InstanceStateBusy)
	inst.instance.SetLastActive(time.Now())
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

// StopIdleManager stops the idle reaper loop.
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

// StartPingManager launches the periodic ping loop.
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

// StopPingManager stops the periodic ping loop.
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
			if inst.instance.State() != domain.InstanceStateReady {
				continue
			}

			// Check strategy-based reclaim rules
			switch spec.Strategy {
			case domain.StrategyPersistent, domain.StrategySingleton:
				// Never reclaim persistent or singleton instances
				continue
			case domain.StrategyStateful:
				// Stateful: only reclaim if no active bindings for this instance
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
			stats := inst.instance.CallStats()
			metrics.TotalCalls += stats.TotalCalls
			metrics.TotalErrors += stats.TotalErrors
			metrics.TotalDuration += stats.TotalDuration
			if stats.LastCallAt.After(metrics.LastCallAt) {
				metrics.LastCallAt = stats.LastCallAt
			}
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

func (s *BasicScheduler) startDrain(specKey string, inst *trackedInstance, timeout time.Duration, reason string) {
	inst.drainOnce.Do(func() {
		inst.drainDone = make(chan struct{})

		s.logger.Info("drain started",
			telemetry.EventField("drain_start"),
			telemetry.ServerTypeField(specKey),
			telemetry.InstanceIDField(inst.instance.ID()),
			zap.Int("busyCount", inst.instance.BusyCount()),
			zap.Duration("timeout", timeout),
		)

		go func() {
			timer := time.NewTimer(timeout)
			defer timer.Stop()

			timedOut := false
			select {
			case <-inst.drainDone:
			case <-timer.C:
				timedOut = true
			}

			state := s.getPool(specKey, inst.instance.Spec())
			state.mu.Lock()
			state.removeDrainingLocked(inst)
			state.mu.Unlock()

			finalReason := reason
			if timedOut {
				finalReason = "drain timeout"
				s.logger.Warn("drain timeout, forcing stop",
					telemetry.EventField("drain_timeout"),
					telemetry.ServerTypeField(specKey),
					telemetry.InstanceIDField(inst.instance.ID()),
				)
			} else {
				s.logger.Info("drain completed",
					telemetry.EventField("drain_complete"),
					telemetry.ServerTypeField(specKey),
					telemetry.InstanceIDField(inst.instance.ID()),
				)
			}

			err := s.stopInstance(context.Background(), state.spec, inst.instance, finalReason)
			s.observeInstanceStop(inst.instance.Spec().Name, err)
			s.recordInstanceStop(state)
		}()

		state := s.getPool(specKey, inst.instance.Spec())
		state.mu.Lock()
		busy := inst.instance.BusyCount()
		state.mu.Unlock()
		if busy == 0 {
			select {
			case <-inst.drainDone:
			default:
				close(inst.drainDone)
			}
		}
	})
}

func (s *BasicScheduler) applyStartCause(ctx context.Context, inst *domain.Instance, started time.Time) {
	if inst == nil {
		return
	}
	cause, ok := domain.StartCauseFromContext(ctx)
	if !ok {
		return
	}
	if cause.Timestamp.IsZero() {
		cause.Timestamp = started
	}
	if !shouldOverrideCause(inst.LastStartCause(), cause) {
		return
	}
	inst.SetLastStartCause(&cause)
}

func (s *BasicScheduler) applyStartCauseLocked(state *poolState, cause domain.StartCause, started time.Time) {
	if cause.Timestamp.IsZero() {
		cause.Timestamp = started
	}
	for _, inst := range state.instances {
		s.updateInstanceCauseLocked(inst.instance, cause)
	}
	for _, inst := range state.draining {
		s.updateInstanceCauseLocked(inst.instance, cause)
	}
}

func (s *BasicScheduler) updateInstanceCauseLocked(inst *domain.Instance, cause domain.StartCause) {
	if inst == nil {
		return
	}
	if !shouldOverrideCause(inst.LastStartCause(), cause) {
		return
	}
	inst.SetLastStartCause(&cause)
}

func shouldOverrideCause(existing *domain.StartCause, next domain.StartCause) bool {
	if existing == nil {
		return true
	}
	if existing.Reason == domain.StartCauseBootstrap && next.Reason != domain.StartCauseBootstrap {
		return true
	}
	return false
}

func (s *BasicScheduler) observeInstanceStart(serverType string, start time.Time, err error) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveInstanceStart(serverType, time.Since(start), err)
}

func (s *BasicScheduler) observeInstanceStop(serverType string, err error) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveInstanceStop(serverType, err)
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
	for _, inst := range state.instances {
		busyCount += inst.instance.BusyCount()
	}
	state.mu.Unlock()

	s.metrics.SetActiveInstances(serverType, activeCount)
	capacity := activeCount * maxConcurrent
	ratio := 0.0
	if capacity > 0 {
		ratio = float64(busyCount) / float64(capacity)
	}
	s.metrics.SetPoolCapacityRatio(serverType, ratio)
}

func isRoutable(state domain.InstanceState) bool {
	return state == domain.InstanceStateReady || state == domain.InstanceStateBusy
}
