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
	ErrUnknownServerType = errors.New("unknown server type")
	ErrNoCapacity        = errors.New("no capacity available")
	ErrStickyBusy        = errors.New("sticky instance at capacity")
	ErrNotImplemented    = errors.New("scheduler not implemented")
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

	serversMu sync.RWMutex
	servers   map[string]*serverState

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

type serverState struct {
	mu        sync.Mutex
	instances []*trackedInstance
	sticky    map[string]*trackedInstance
}

type stopCandidate struct {
	serverType string
	state      *serverState
	inst       *trackedInstance
	reason     string
}

type serverEntry struct {
	serverType string
	state      *serverState
}

func NewBasicScheduler(lifecycle domain.Lifecycle, specs map[string]domain.ServerSpec, opts SchedulerOptions) *BasicScheduler {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BasicScheduler{
		lifecycle: lifecycle,
		specs:     specs,
		servers:   make(map[string]*serverState),
		probe:     opts.Probe,
		logger:    logger.Named("scheduler"),
		metrics:   opts.Metrics,
		health:    opts.Health,
		stopIdle:  make(chan struct{}),
		stopPing:  make(chan struct{}),
	}
}

func (s *BasicScheduler) Acquire(ctx context.Context, serverType, routingKey string) (*domain.Instance, error) {
	spec, ok := s.specs[serverType]
	if !ok {
		return nil, ErrUnknownServerType
	}

	state := s.getServerState(serverType)
	state.mu.Lock()
	if spec.Sticky && routingKey != "" {
		if inst := state.lookupStickyLocked(routingKey); inst != nil {
			if !isRoutable(inst.instance.State) {
				state.unbindStickyLocked(routingKey)
			} else {
				if inst.instance.BusyCount >= spec.MaxConcurrent {
					state.mu.Unlock()
					return nil, ErrStickyBusy
				}
				instance := state.markBusyLocked(inst)
				state.mu.Unlock()
				return instance, nil
			}
		}
	}

	if inst := state.findReadyInstanceLocked(spec); inst != nil {
		instance := state.markBusyLocked(inst)
		state.mu.Unlock()
		return instance, nil
	}
	state.mu.Unlock()

	started := time.Now()
	newInst, err := s.lifecycle.StartInstance(ctx, spec)
	s.observeInstanceStart(spec.Name, started, err)
	if err != nil {
		return nil, fmt.Errorf("start instance: %w", err)
	}
	tracked := &trackedInstance{instance: newInst}

	state.mu.Lock()
	state.instances = append(state.instances, tracked)
	if spec.Sticky && routingKey != "" {
		state.bindStickyLocked(routingKey, tracked)
	}
	instance := state.markBusyLocked(tracked)
	activeCount := len(state.instances)
	state.mu.Unlock()
	s.observeActiveInstances(serverType, activeCount)

	return instance, nil
}

func (s *BasicScheduler) Release(ctx context.Context, instance *domain.Instance) error {
	if instance == nil {
		return errors.New("instance is nil")
	}

	state := s.getServerState(instance.Spec.Name)
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

func (s *BasicScheduler) getServerState(serverType string) *serverState {
	s.serversMu.RLock()
	state := s.servers[serverType]
	s.serversMu.RUnlock()

	if state != nil {
		return state
	}

	s.serversMu.Lock()
	defer s.serversMu.Unlock()
	state = s.servers[serverType]
	if state == nil {
		state = &serverState{}
		s.servers[serverType] = state
	}
	return state
}

func (s *BasicScheduler) snapshotServers() []serverEntry {
	s.serversMu.RLock()
	defer s.serversMu.RUnlock()

	entries := make([]serverEntry, 0, len(s.servers))
	for serverType, state := range s.servers {
		entries = append(entries, serverEntry{serverType: serverType, state: state})
	}
	return entries
}

func (s *serverState) lookupStickyLocked(routingKey string) *trackedInstance {
	if s.sticky == nil {
		return nil
	}
	return s.sticky[routingKey]
}

func (s *serverState) bindStickyLocked(routingKey string, inst *trackedInstance) {
	if s.sticky == nil {
		s.sticky = make(map[string]*trackedInstance)
	}
	s.sticky[routingKey] = inst
}

func (s *serverState) unbindStickyLocked(routingKey string) {
	if s.sticky == nil {
		return
	}
	delete(s.sticky, routingKey)
	if len(s.sticky) == 0 {
		s.sticky = nil
	}
}

func (s *serverState) findReadyInstanceLocked(spec domain.ServerSpec) *trackedInstance {
	for _, inst := range s.instances {
		if inst.instance.BusyCount >= spec.MaxConcurrent {
			continue
		}
		if !isRoutable(inst.instance.State) {
			continue
		}
		return inst
	}
	return nil
}

func (s *serverState) markBusyLocked(inst *trackedInstance) *domain.Instance {
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

	for _, entry := range s.snapshotServers() {
		spec := s.specs[entry.serverType]

		entry.state.mu.Lock()
		readyCount := entry.state.countReadyLocked()

		for _, inst := range entry.state.instances {
			if inst.instance.State != domain.InstanceStateReady {
				continue
			}
			if spec.Persistent || spec.Sticky {
				continue
			}
			if readyCount <= spec.MinReady {
				continue
			}
			idleFor := now.Sub(inst.instance.LastActive)
			if idleFor >= time.Duration(spec.IdleSeconds)*time.Second {
				inst.instance.State = domain.InstanceStateDraining
				s.logger.Info("idle reap",
					telemetry.EventField(telemetry.EventIdleReap),
					telemetry.ServerTypeField(entry.serverType),
					telemetry.InstanceIDField(inst.instance.ID),
					telemetry.StateField(string(inst.instance.State)),
					telemetry.DurationField(idleFor),
				)
				candidates = append(candidates, stopCandidate{
					serverType: entry.serverType,
					state:      entry.state,
					inst:       inst,
					reason:     "idle timeout",
				})
				readyCount--
			}
		}
		entry.state.mu.Unlock()
	}

	for _, candidate := range candidates {
		err := s.lifecycle.StopInstance(context.Background(), candidate.inst.instance, candidate.reason)
		s.observeInstanceStop(candidate.serverType, err)
		candidate.state.mu.Lock()
		activeCount := candidate.state.removeInstanceLocked(candidate.inst)
		candidate.state.mu.Unlock()
		s.observeActiveInstances(candidate.serverType, activeCount)
	}
}

func (s *BasicScheduler) probeInstances() {
	if s.probe == nil {
		return
	}

	var candidates []stopCandidate
	var checks []stopCandidate

	for _, entry := range s.snapshotServers() {
		entry.state.mu.Lock()
		for _, inst := range entry.state.instances {
			if !isRoutable(inst.instance.State) {
				continue
			}
			checks = append(checks, stopCandidate{
				serverType: entry.serverType,
				state:      entry.state,
				inst:       inst,
				reason:     "ping failure",
			})
		}
		entry.state.mu.Unlock()
	}

	for _, candidate := range checks {
		if err := s.probe.Ping(context.Background(), candidate.inst.instance.Conn); err != nil {
			s.logger.Warn("ping failed",
				telemetry.EventField(telemetry.EventPingFailure),
				telemetry.ServerTypeField(candidate.serverType),
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
		s.observeInstanceStop(candidate.serverType, err)
		candidate.state.mu.Lock()
		activeCount := candidate.state.removeInstanceLocked(candidate.inst)
		candidate.state.mu.Unlock()
		s.observeActiveInstances(candidate.serverType, activeCount)
	}
}

// StopAll terminates all known instances for graceful shutdown.
func (s *BasicScheduler) StopAll(ctx context.Context) {
	var candidates []stopCandidate

	entries := s.snapshotServers()
	for _, entry := range entries {
		entry.state.mu.Lock()
		for _, inst := range entry.state.instances {
			candidates = append(candidates, stopCandidate{
				serverType: entry.serverType,
				state:      entry.state,
				inst:       inst,
				reason:     "shutdown",
			})
		}
		entry.state.mu.Unlock()
	}

	for _, candidate := range candidates {
		err := s.lifecycle.StopInstance(ctx, candidate.inst.instance, candidate.reason)
		s.observeInstanceStop(candidate.serverType, err)
	}

	for _, entry := range entries {
		s.observeActiveInstances(entry.serverType, 0)
	}
	s.serversMu.Lock()
	s.servers = make(map[string]*serverState)
	s.serversMu.Unlock()
}

func (s *serverState) removeInstanceLocked(inst *trackedInstance) int {
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

func (s *serverState) countReadyLocked() int {
	count := 0
	for _, inst := range s.instances {
		if inst.instance.State == domain.InstanceStateReady {
			count++
		}
	}
	return count
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

func (s *BasicScheduler) observeActiveInstances(serverType string, count int) {
	if s.metrics == nil {
		return
	}
	s.metrics.SetActiveInstances(serverType, count)
}

func isRoutable(state domain.InstanceState) bool {
	return state == domain.InstanceStateReady || state == domain.InstanceStateBusy
}
