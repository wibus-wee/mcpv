package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
)

type clientRegistry struct {
	state *controlPlaneState

	mu               sync.Mutex
	activeClients    map[string]clientState
	activeClientSubs map[chan domain.ActiveClientSnapshot]struct{}
	clientChangeSubs map[chan clientChangeEvent]struct{}
	specCounts       map[string]int
	monitorStarted   bool
}

type clientChangeEvent struct {
	Client string
}

const clientReapTimeoutMultiplier = 2

func newClientRegistry(state *controlPlaneState) *clientRegistry {
	return &clientRegistry{
		state:            state,
		activeClients:    make(map[string]clientState),
		activeClientSubs: make(map[chan domain.ActiveClientSnapshot]struct{}),
		clientChangeSubs: make(map[chan clientChangeEvent]struct{}),
		specCounts:       make(map[string]int),
	}
}

// StartMonitor begins monitoring client heartbeats.
func (r *clientRegistry) StartMonitor(ctx context.Context) {
	runtime := r.state.Runtime()
	interval := time.Duration(runtime.ClientCheckSeconds) * time.Second
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
				r.reapDeadClients(ctx)
			}
		}
	}()
}

// RegisterClient registers a client and returns registration metadata.
func (r *clientRegistry) RegisterClient(ctx context.Context, client string, pid int, tags []string) (domain.ClientRegistration, error) {
	if client == "" {
		return domain.ClientRegistration{}, errors.New("client is required")
	}
	if pid <= 0 {
		return domain.ClientRegistration{}, errors.New("pid must be > 0")
	}
	normalizedTags := normalizeTags(tags)
	visibleSpecKeys, visibleServerCount := r.visibleSpecKeys(normalizedTags)

	var toActivate []string
	var toDeactivate []string
	now := time.Now()
	var snapshot domain.ActiveClientSnapshot
	var shouldBroadcast bool
	var tagsChanged bool

	r.mu.Lock()
	if existing, ok := r.activeClients[client]; ok {
		tagsChanged = !tagsEqual(existing.tags, normalizedTags)
		if existing.pid == pid && !tagsChanged {
			existing.lastHeartbeat = now
			r.activeClients[client] = existing
			r.mu.Unlock()
			return domain.ClientRegistration{
				Client:             client,
				Tags:               normalizedTags,
				VisibleServerCount: visibleServerCount,
			}, nil
		}
		toActivate, toDeactivate = diffKeys(visibleSpecKeys, existing.specKeys)
		existing.pid = pid
		existing.tags = normalizedTags
		existing.specKeys = visibleSpecKeys
		existing.lastHeartbeat = now
		r.activeClients[client] = existing
		applySpecDelta(r.specCounts, toActivate, toDeactivate)
		snapshot = r.snapshotActiveClientsLocked(now)
		shouldBroadcast = true
	} else {
		r.activeClients[client] = clientState{
			pid:           pid,
			tags:          normalizedTags,
			specKeys:      visibleSpecKeys,
			lastHeartbeat: now,
		}
		applySpecDelta(r.specCounts, visibleSpecKeys, nil)
		toActivate = visibleSpecKeys
		snapshot = r.snapshotActiveClientsLocked(now)
		shouldBroadcast = true
	}
	r.mu.Unlock()

	toActivate, toDeactivate = filterOverlap(toActivate, toDeactivate)

	if err := r.activateSpecs(ctx, toActivate, client); err != nil {
		_ = r.UnregisterClient(ctx, client)
		return domain.ClientRegistration{}, err
	}
	if err := r.deactivateSpecs(ctx, toDeactivate); err != nil {
		r.state.logger.Warn("spec deactivation failed", zap.Error(err))
	}
	if shouldBroadcast {
		r.broadcastActiveClients(finalizeActiveClientSnapshot(snapshot))
	}
	if tagsChanged {
		r.broadcastClientChange(clientChangeEvent{Client: client})
	}

	return domain.ClientRegistration{
		Client:             client,
		Tags:               normalizedTags,
		VisibleServerCount: visibleServerCount,
	}, nil
}

// UnregisterClient unregisters a client.
func (r *clientRegistry) UnregisterClient(ctx context.Context, client string) error {
	if client == "" {
		return errors.New("client is required")
	}
	var toDeactivate []string
	var snapshot domain.ActiveClientSnapshot
	var shouldBroadcast bool

	r.mu.Lock()
	state, ok := r.activeClients[client]
	if ok {
		delete(r.activeClients, client)
		applySpecDelta(r.specCounts, nil, state.specKeys)
		toDeactivate = state.specKeys
		snapshot = r.snapshotActiveClientsLocked(time.Now())
		shouldBroadcast = true
	}
	r.mu.Unlock()

	if err := r.deactivateSpecs(ctx, toDeactivate); err != nil {
		r.state.logger.Warn("spec deactivation failed", zap.Error(err))
	}
	if shouldBroadcast {
		r.broadcastActiveClients(finalizeActiveClientSnapshot(snapshot))
	}
	return nil
}

// ListActiveClients lists active clients.
func (r *clientRegistry) ListActiveClients(ctx context.Context) ([]domain.ActiveClient, error) {
	now := time.Now()
	r.mu.Lock()
	snapshot := r.snapshotActiveClientsLocked(now)
	r.mu.Unlock()
	return finalizeActiveClientSnapshot(snapshot).Clients, nil
}

// WatchActiveClients streams active client updates.
func (r *clientRegistry) WatchActiveClients(ctx context.Context) (<-chan domain.ActiveClientSnapshot, error) {
	ch := make(chan domain.ActiveClientSnapshot, 1)
	r.mu.Lock()
	r.activeClientSubs[ch] = struct{}{}
	snapshot := r.snapshotActiveClientsLocked(time.Now())
	r.mu.Unlock()

	sendActiveClientSnapshot(ch, finalizeActiveClientSnapshot(snapshot))

	go func() {
		<-ctx.Done()
		r.mu.Lock()
		delete(r.activeClientSubs, ch)
		r.mu.Unlock()
	}()

	return ch, nil
}

// WatchClientChanges streams client change events.
func (r *clientRegistry) WatchClientChanges(ctx context.Context) <-chan clientChangeEvent {
	ch := make(chan clientChangeEvent, 16)
	r.mu.Lock()
	r.clientChangeSubs[ch] = struct{}{}
	r.mu.Unlock()

	go func() {
		<-ctx.Done()
		r.mu.Lock()
		delete(r.clientChangeSubs, ch)
		r.mu.Unlock()
		close(ch)
	}()

	return ch
}

func (r *clientRegistry) resolveClientTags(client string) ([]string, error) {
	if client == "" {
		return nil, domain.ErrClientNotRegistered
	}
	r.mu.Lock()
	state, ok := r.activeClients[client]
	if ok {
		state.lastHeartbeat = time.Now()
		r.activeClients[client] = state
	}
	r.mu.Unlock()
	if !ok {
		return nil, domain.ErrClientNotRegistered
	}
	return append([]string(nil), state.tags...), nil
}

func (r *clientRegistry) resolveVisibleSpecKeys(client string) ([]string, error) {
	if client == "" {
		return nil, domain.ErrClientNotRegistered
	}
	r.mu.Lock()
	state, ok := r.activeClients[client]
	if ok {
		state.lastHeartbeat = time.Now()
		r.activeClients[client] = state
	}
	r.mu.Unlock()
	if !ok {
		return nil, domain.ErrClientNotRegistered
	}
	return append([]string(nil), state.specKeys...), nil
}

func (r *clientRegistry) visibleSpecKeys(tags []string) ([]string, int) {
	catalog := r.state.Catalog()
	serverSpecKeys := r.state.ServerSpecKeys()
	if len(serverSpecKeys) == 0 {
		return nil, 0
	}
	visible := make(map[string]struct{})
	serverCount := 0
	for name, specKey := range serverSpecKeys {
		spec, ok := catalog.Specs[name]
		if !ok {
			continue
		}
		if isVisibleToTags(tags, spec.Tags) {
			serverCount++
			visible[specKey] = struct{}{}
		}
	}
	return keysFromSet(visible), serverCount
}

func (r *clientRegistry) activateSpecs(ctx context.Context, specKeys []string, client string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	runtime := r.state.Runtime()
	registry := r.state.SpecRegistry()
	for _, specKey := range order {
		spec, ok := registry[specKey]
		if !ok {
			return errors.New("unknown spec key " + specKey)
		}
		minReady := activeMinReady(spec)
		cause := clientStartCause(runtime, spec, client, minReady)
		causeCtx := domain.WithStartCause(ctx, cause)
		if r.state.initManager != nil {
			err := r.state.initManager.SetMinReady(specKey, minReady, cause)
			if err == nil {
				continue
			}
			r.state.logger.Warn("server init manager failed to set min ready", zap.String("specKey", specKey), zap.Error(err))
		}
		if r.state.scheduler == nil {
			return errors.New("scheduler not configured")
		}
		if err := r.state.scheduler.SetDesiredMinReady(causeCtx, specKey, minReady); err != nil {
			return err
		}
	}
	return nil
}

func (r *clientRegistry) deactivateSpecs(ctx context.Context, specKeys []string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	runtime := r.state.Runtime()
	registry := r.state.SpecRegistry()
	var firstErr error
	for _, specKey := range order {
		spec, ok := registry[specKey]
		if ok && resolveActivationMode(runtime, spec) == domain.ActivationAlwaysOn {
			continue
		}
		if r.state.initManager != nil {
			_ = r.state.initManager.SetMinReady(specKey, 0, domain.StartCause{})
		}
		if r.state.scheduler == nil {
			if firstErr == nil {
				firstErr = errors.New("scheduler not configured")
			}
			continue
		}
		if err := r.state.scheduler.StopSpec(ctx, specKey, "client inactive"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *clientRegistry) reapDeadClients(ctx context.Context) {
	now := time.Now()
	runtime := r.state.Runtime()
	timeout := time.Duration(runtime.ClientCheckSeconds*clientReapTimeoutMultiplier) * time.Second
	inactiveTimeout := time.Duration(runtime.ClientInactiveSeconds) * time.Second
	r.mu.Lock()
	clients := make([]string, 0, len(r.activeClients))
	for client, state := range r.activeClients {
		if inactiveTimeout > 0 && !state.lastHeartbeat.IsZero() && now.Sub(state.lastHeartbeat) > inactiveTimeout {
			clients = append(clients, client)
			continue
		}
		if timeout > 0 && !state.lastHeartbeat.IsZero() && now.Sub(state.lastHeartbeat) <= timeout {
			continue
		}
		if !pidAlive(state.pid) {
			clients = append(clients, client)
		}
	}
	r.mu.Unlock()

	for _, client := range clients {
		if err := r.UnregisterClient(ctx, client); err != nil {
			r.state.logger.Warn("client reap failed", zap.String("client", client), zap.Error(err))
		}
	}
}

func (r *clientRegistry) snapshotActiveClientsLocked(now time.Time) domain.ActiveClientSnapshot {
	clients := make([]domain.ActiveClient, 0, len(r.activeClients))
	for client, state := range r.activeClients {
		clients = append(clients, domain.ActiveClient{
			Client:        client,
			PID:           state.pid,
			Tags:          append([]string(nil), state.tags...),
			LastHeartbeat: state.lastHeartbeat,
		})
	}

	return domain.ActiveClientSnapshot{
		Clients:     clients,
		GeneratedAt: now,
	}
}

func finalizeActiveClientSnapshot(snapshot domain.ActiveClientSnapshot) domain.ActiveClientSnapshot {
	if len(snapshot.Clients) == 0 {
		return snapshot
	}
	clients := append([]domain.ActiveClient(nil), snapshot.Clients...)
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].Client < clients[j].Client
	})
	snapshot.Clients = clients
	return snapshot
}

func (r *clientRegistry) broadcastActiveClients(snapshot domain.ActiveClientSnapshot) {
	subs := r.copyActiveClientSubscribers()
	for _, ch := range subs {
		sendActiveClientSnapshot(ch, snapshot)
	}
}

// ApplyCatalogUpdate updates client state based on catalog changes.
func (r *clientRegistry) ApplyCatalogUpdate(ctx context.Context, update domain.CatalogUpdate) error {
	now := time.Now()

	r.mu.Lock()
	oldSpecCounts := copyCounts(r.specCounts)
	newSpecCounts := make(map[string]int)
	changedClients := make([]string, 0)

	for client, state := range r.activeClients {
		nextSpecKeys, _ := visibleSpecKeysForCatalog(update.Snapshot.Catalog, update.Snapshot.Summary.ServerSpecKeys, state.tags)
		if !sameKeySet(state.specKeys, nextSpecKeys) {
			changedClients = append(changedClients, client)
			state.specKeys = nextSpecKeys
			r.activeClients[client] = state
		}
		for _, specKey := range nextSpecKeys {
			newSpecCounts[specKey]++
		}
	}
	r.specCounts = newSpecCounts
	snapshot := r.snapshotActiveClientsLocked(now)
	r.mu.Unlock()

	specsToStart, specsToStop := diffCounts(oldSpecCounts, newSpecCounts)
	specsToStart, specsToStop = filterOverlap(specsToStart, specsToStop)

	if err := r.activateSpecs(ctx, specsToStart, ""); err != nil {
		return err
	}
	if err := r.deactivateSpecs(ctx, specsToStop); err != nil {
		r.state.logger.Warn("spec deactivation failed", zap.Error(err))
	}
	r.broadcastActiveClients(finalizeActiveClientSnapshot(snapshot))
	for _, client := range changedClients {
		r.broadcastClientChange(clientChangeEvent{Client: client})
	}
	return nil
}

func (r *clientRegistry) copyActiveClientSubscribers() []chan domain.ActiveClientSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	subs := make([]chan domain.ActiveClientSnapshot, 0, len(r.activeClientSubs))
	for ch := range r.activeClientSubs {
		subs = append(subs, ch)
	}
	return subs
}

func sendActiveClientSnapshot(ch chan domain.ActiveClientSnapshot, snapshot domain.ActiveClientSnapshot) {
	select {
	case ch <- snapshot:
	default:
	}
}

func (r *clientRegistry) broadcastClientChange(event clientChangeEvent) {
	r.mu.Lock()
	subs := make([]chan clientChangeEvent, 0, len(r.clientChangeSubs))
	for ch := range r.clientChangeSubs {
		subs = append(subs, ch)
	}
	r.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func applySpecDelta(counts map[string]int, add []string, remove []string) {
	for _, key := range add {
		counts[key] = counts[key] + 1
	}
	for _, key := range remove {
		count := counts[key]
		switch {
		case count <= 1:
			delete(counts, key)
		default:
			counts[key] = count - 1
		}
	}
}

func sameKeySet(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func diffKeys(next []string, prev []string) ([]string, []string) {
	nextSet := make(map[string]struct{}, len(next))
	for _, key := range next {
		nextSet[key] = struct{}{}
	}
	prevSet := make(map[string]struct{}, len(prev))
	for _, key := range prev {
		prevSet[key] = struct{}{}
	}
	var toActivate []string
	for key := range nextSet {
		if _, ok := prevSet[key]; !ok {
			toActivate = append(toActivate, key)
		}
	}
	var toDeactivate []string
	for key := range prevSet {
		if _, ok := nextSet[key]; !ok {
			toDeactivate = append(toDeactivate, key)
		}
	}
	return toActivate, toDeactivate
}

func visibleSpecKeysForCatalog(catalog domain.Catalog, serverSpecKeys map[string]string, tags []string) ([]string, int) {
	if len(serverSpecKeys) == 0 {
		return nil, 0
	}
	visible := make(map[string]struct{})
	serverCount := 0
	for name, specKey := range serverSpecKeys {
		spec, ok := catalog.Specs[name]
		if !ok {
			continue
		}
		if isVisibleToTags(tags, spec.Tags) {
			serverCount++
			visible[specKey] = struct{}{}
		}
	}
	return keysFromSet(visible), serverCount
}


func tagsEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	unique := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		unique[tag] = struct{}{}
	}
	if len(unique) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(unique))
	for tag := range unique {
		normalized = append(normalized, tag)
	}
	sort.Strings(normalized)
	return normalized
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

func diffCounts(oldCounts map[string]int, newCounts map[string]int) ([]string, []string) {
	var starts []string
	var stops []string

	for key, count := range newCounts {
		if count > 0 && oldCounts[key] == 0 {
			starts = append(starts, key)
		}
	}
	for key, count := range oldCounts {
		if count > 0 && newCounts[key] == 0 {
			stops = append(stops, key)
		}
	}
	return starts, stops
}

func copyCounts(src map[string]int) map[string]int {
	dst := make(map[string]int, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func keysFromSet(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
