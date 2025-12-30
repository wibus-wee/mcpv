package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
)

type callerRegistry struct {
	state *controlPlaneState

	mu               sync.Mutex
	activeCallers    map[string]callerState
	activeCallerSubs map[chan domain.ActiveCallerSnapshot]struct{}
	profileCounts    map[string]int
	specCounts       map[string]int
	monitorStarted   bool
}

const callerReapTimeoutMultiplier = 2

func newCallerRegistry(state *controlPlaneState) *callerRegistry {
	return &callerRegistry{
		state:            state,
		activeCallers:    make(map[string]callerState),
		activeCallerSubs: make(map[chan domain.ActiveCallerSnapshot]struct{}),
		profileCounts:    make(map[string]int),
		specCounts:       make(map[string]int),
	}
}

func (r *callerRegistry) StartMonitor(ctx context.Context) {
	interval := time.Duration(r.state.runtime.CallerCheckSeconds) * time.Second
	if interval <= 0 {
		return
	}

	r.mu.Lock()
	if r.monitorStarted {
		r.mu.Unlock()
		return
	}
	r.monitorStarted = true
	r.mu.Unlock()

	if ctx == nil {
		ctx = r.state.ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.reapDeadCallers(ctx)
			}
		}
	}()
}

func (r *callerRegistry) RegisterCaller(ctx context.Context, caller string, pid int) (string, error) {
	if caller == "" {
		return "", errors.New("caller is required")
	}
	if pid <= 0 {
		return "", errors.New("pid must be > 0")
	}

	profileName, err := r.resolveProfileName(caller)
	if err != nil {
		return "", err
	}

	var toStartProfiles []string
	var toStopProfiles []string
	var toActivateSpecs []string
	var toDeactivateSpecs []string
	now := time.Now()
	var snapshot domain.ActiveCallerSnapshot
	var shouldBroadcast bool

	r.mu.Lock()
	if existing, ok := r.activeCallers[caller]; ok {
		if existing.pid == pid && existing.profile == profileName {
			existing.lastHeartbeat = now
			r.activeCallers[caller] = existing
			r.mu.Unlock()
			return profileName, nil
		}
		if existing.profile == profileName {
			existing.pid = pid
			existing.lastHeartbeat = now
			r.activeCallers[caller] = existing
			snapshot = r.snapshotActiveCallersLocked(now)
			shouldBroadcast = true
			r.mu.Unlock()
			r.broadcastActiveCallers(snapshot)
			return profileName, nil
		}
		r.removeProfileLocked(existing.profile, &toStopProfiles, &toDeactivateSpecs)
	}
	r.activeCallers[caller] = callerState{pid: pid, profile: profileName, lastHeartbeat: now}
	r.addProfileLocked(profileName, &toStartProfiles, &toActivateSpecs)
	snapshot = r.snapshotActiveCallersLocked(now)
	shouldBroadcast = true
	r.mu.Unlock()

	toActivateSpecs, toDeactivateSpecs = filterOverlap(toActivateSpecs, toDeactivateSpecs)

	if err := r.activateSpecs(ctx, toActivateSpecs); err != nil {
		_ = r.UnregisterCaller(ctx, caller)
		return "", err
	}
	if err := r.deactivateSpecs(ctx, toDeactivateSpecs); err != nil {
		r.state.logger.Warn("spec deactivation failed", zap.Error(err))
	}
	if shouldBroadcast {
		r.broadcastActiveCallers(snapshot)
	}
	r.activateProfiles(toStartProfiles)
	r.deactivateProfiles(toStopProfiles)
	return profileName, nil
}

func (r *callerRegistry) UnregisterCaller(ctx context.Context, caller string) error {
	if caller == "" {
		return errors.New("caller is required")
	}
	var toStopProfiles []string
	var toDeactivateSpecs []string
	var snapshot domain.ActiveCallerSnapshot
	var shouldBroadcast bool

	r.mu.Lock()
	state, ok := r.activeCallers[caller]
	if ok {
		delete(r.activeCallers, caller)
		r.removeProfileLocked(state.profile, &toStopProfiles, &toDeactivateSpecs)
		snapshot = r.snapshotActiveCallersLocked(time.Now())
		shouldBroadcast = true
	}
	r.mu.Unlock()

	if err := r.deactivateSpecs(ctx, toDeactivateSpecs); err != nil {
		r.state.logger.Warn("spec deactivation failed", zap.Error(err))
	}
	if shouldBroadcast {
		r.broadcastActiveCallers(snapshot)
	}
	r.deactivateProfiles(toStopProfiles)
	return nil
}

func (r *callerRegistry) ListActiveCallers(ctx context.Context) ([]domain.ActiveCaller, error) {
	now := time.Now()
	r.mu.Lock()
	snapshot := r.snapshotActiveCallersLocked(now)
	r.mu.Unlock()
	return snapshot.Callers, nil
}

func (r *callerRegistry) WatchActiveCallers(ctx context.Context) (<-chan domain.ActiveCallerSnapshot, error) {
	ch := make(chan domain.ActiveCallerSnapshot, 1)
	r.mu.Lock()
	r.activeCallerSubs[ch] = struct{}{}
	snapshot := r.snapshotActiveCallersLocked(time.Now())
	r.mu.Unlock()

	sendActiveCallerSnapshot(ch, snapshot)

	go func() {
		<-ctx.Done()
		r.mu.Lock()
		delete(r.activeCallerSubs, ch)
		r.mu.Unlock()
	}()

	return ch, nil
}

func (r *callerRegistry) resolveProfile(caller string) (*profileRuntime, error) {
	if caller == "" {
		return nil, domain.ErrCallerNotRegistered
	}
	r.mu.Lock()
	state, ok := r.activeCallers[caller]
	if ok {
		state.lastHeartbeat = time.Now()
		r.activeCallers[caller] = state
	}
	r.mu.Unlock()
	if !ok {
		return nil, domain.ErrCallerNotRegistered
	}
	profile, ok := r.state.profiles[state.profile]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", state.profile)
	}
	return profile, nil
}

func (r *callerRegistry) resolveProfileName(caller string) (string, error) {
	profileName := domain.DefaultProfileName
	if caller != "" {
		if mapped, ok := r.state.callers[caller]; ok {
			profileName = mapped
		}
	}
	if _, ok := r.state.profiles[profileName]; !ok {
		return "", fmt.Errorf("profile %q not found", profileName)
	}
	return profileName, nil
}

func (r *callerRegistry) activeProfileNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	names := make([]string, 0, len(r.profileCounts))
	for name, count := range r.profileCounts {
		if count > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (r *callerRegistry) profileContainsSpecKey(runtime *profileRuntime, specKey string) bool {
	for _, key := range runtime.specKeys {
		if key == specKey {
			return true
		}
	}
	return false
}

func (r *callerRegistry) addProfileLocked(profile string, profileStarts *[]string, specStarts *[]string) {
	runtime, ok := r.state.profiles[profile]
	if !ok {
		return
	}
	count := r.profileCounts[profile] + 1
	r.profileCounts[profile] = count
	if count == 1 {
		*profileStarts = append(*profileStarts, profile)
	}
	for _, specKey := range runtime.specKeys {
		specCount := r.specCounts[specKey] + 1
		r.specCounts[specKey] = specCount
		if specCount == 1 {
			*specStarts = append(*specStarts, specKey)
		}
	}
}

func (r *callerRegistry) removeProfileLocked(profile string, profileStops *[]string, specStops *[]string) {
	runtime, ok := r.state.profiles[profile]
	if !ok {
		return
	}
	count := r.profileCounts[profile]
	switch {
	case count <= 1:
		delete(r.profileCounts, profile)
		if count > 0 {
			*profileStops = append(*profileStops, profile)
		}
	default:
		r.profileCounts[profile] = count - 1
	}
	for _, specKey := range runtime.specKeys {
		specCount := r.specCounts[specKey]
		switch {
		case specCount <= 1:
			delete(r.specCounts, specKey)
			if specCount > 0 {
				*specStops = append(*specStops, specKey)
			}
		default:
			r.specCounts[specKey] = specCount - 1
		}
	}
}

func (r *callerRegistry) activateSpecs(ctx context.Context, specKeys []string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	for _, specKey := range order {
		spec, ok := r.state.specRegistry[specKey]
		if !ok {
			return fmt.Errorf("unknown spec key %q", specKey)
		}
		minReady := spec.MinReady
		if minReady < 1 {
			minReady = 1
		}
		if r.state.initManager != nil {
			err := r.state.initManager.SetMinReady(specKey, minReady)
			if err == nil {
				continue
			}
			r.state.logger.Warn("server init manager failed to set min ready", zap.String("specKey", specKey), zap.Error(err))
		}
		if r.state.scheduler == nil {
			return errors.New("scheduler not configured")
		}
		if err := r.state.scheduler.SetDesiredMinReady(ctx, specKey, minReady); err != nil {
			return err
		}
	}
	return nil
}

func (r *callerRegistry) deactivateSpecs(ctx context.Context, specKeys []string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	var firstErr error
	for _, specKey := range order {
		if r.state.initManager != nil {
			_ = r.state.initManager.SetMinReady(specKey, 0)
		}
		if r.state.scheduler == nil {
			if firstErr == nil {
				firstErr = errors.New("scheduler not configured")
			}
			continue
		}
		if err := r.state.scheduler.StopSpec(ctx, specKey, "caller inactive"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *callerRegistry) activateProfiles(profiles []string) {
	for _, profile := range profiles {
		if runtime, ok := r.state.profiles[profile]; ok {
			runtime.Activate(r.state.ctx)
		}
	}
}

func (r *callerRegistry) deactivateProfiles(profiles []string) {
	for _, profile := range profiles {
		if runtime, ok := r.state.profiles[profile]; ok {
			runtime.Deactivate()
		}
	}
}

func (r *callerRegistry) reapDeadCallers(ctx context.Context) {
	now := time.Now()
	timeout := time.Duration(r.state.runtime.CallerCheckSeconds*callerReapTimeoutMultiplier) * time.Second
	r.mu.Lock()
	callers := make([]string, 0, len(r.activeCallers))
	for caller, state := range r.activeCallers {
		if timeout > 0 && !state.lastHeartbeat.IsZero() && now.Sub(state.lastHeartbeat) <= timeout {
			continue
		}
		if !pidAlive(state.pid) {
			callers = append(callers, caller)
		}
	}
	r.mu.Unlock()

	for _, caller := range callers {
		if err := r.UnregisterCaller(ctx, caller); err != nil {
			r.state.logger.Warn("caller reap failed", zap.String("caller", caller), zap.Error(err))
		}
	}
}

func (r *callerRegistry) snapshotActiveCallersLocked(now time.Time) domain.ActiveCallerSnapshot {
	callers := make([]domain.ActiveCaller, 0, len(r.activeCallers))
	for caller, state := range r.activeCallers {
		callers = append(callers, domain.ActiveCaller{
			Caller:        caller,
			PID:           state.pid,
			Profile:       state.profile,
			LastHeartbeat: state.lastHeartbeat,
		})
	}

	sort.Slice(callers, func(i, j int) bool {
		return callers[i].Caller < callers[j].Caller
	})

	return domain.ActiveCallerSnapshot{
		Callers:     callers,
		GeneratedAt: now,
	}
}

func (r *callerRegistry) broadcastActiveCallers(snapshot domain.ActiveCallerSnapshot) {
	subs := r.copyActiveCallerSubscribers()
	for _, ch := range subs {
		sendActiveCallerSnapshot(ch, snapshot)
	}
}

func (r *callerRegistry) copyActiveCallerSubscribers() []chan domain.ActiveCallerSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	subs := make([]chan domain.ActiveCallerSnapshot, 0, len(r.activeCallerSubs))
	for ch := range r.activeCallerSubs {
		subs = append(subs, ch)
	}
	return subs
}

func sendActiveCallerSnapshot(ch chan domain.ActiveCallerSnapshot, snapshot domain.ActiveCallerSnapshot) {
	select {
	case ch <- snapshot:
	default:
	}
}

func filterOverlap(activate []string, deactivate []string) ([]string, []string) {
	if len(activate) == 0 || len(deactivate) == 0 {
		return activate, deactivate
	}
	deactivateSet := make(map[string]struct{}, len(deactivate))
	for _, key := range deactivate {
		deactivateSet[key] = struct{}{}
	}
	filteredActivate := make([]string, 0, len(activate))
	for _, key := range activate {
		if _, ok := deactivateSet[key]; ok {
			delete(deactivateSet, key)
			continue
		}
		filteredActivate = append(filteredActivate, key)
	}
	filteredDeactivate := make([]string, 0, len(deactivateSet))
	for _, key := range deactivate {
		if _, ok := deactivateSet[key]; ok {
			filteredDeactivate = append(filteredDeactivate, key)
		}
	}
	return filteredActivate, filteredDeactivate
}
