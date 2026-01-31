package controlplane

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/domain"
)

type ClientRegistry struct {
	state *State

	mu               sync.Mutex
	activeClients    map[string]clientState
	activeClientSubs map[chan domain.ActiveClientSnapshot]struct{}
	clientChangeSubs map[chan ClientChangeEvent]struct{}
	specCounts       map[string]int
	monitorStarted   bool
	monitorCancel    context.CancelFunc
}

type ClientChangeEvent struct {
	Client string
}

const clientReapTimeoutMultiplier = 2

func isInternalClientName(client string) bool {
	return client == domain.InternalUIClientName
}

func NewClientRegistry(state *State) *ClientRegistry {
	return &ClientRegistry{
		state:            state,
		activeClients:    make(map[string]clientState),
		activeClientSubs: make(map[chan domain.ActiveClientSnapshot]struct{}),
		clientChangeSubs: make(map[chan ClientChangeEvent]struct{}),
		specCounts:       make(map[string]int),
	}
}

// StartMonitor begins monitoring client heartbeats.
func (r *ClientRegistry) StartMonitor(ctx context.Context) {
	runtime := r.state.Runtime()
	interval := runtime.ClientCheckInterval()
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

	monitorCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.monitorCancel = cancel
	r.mu.Unlock()

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				r.reapDeadClients(monitorCtx)
			}
		}
	}()
}

// UpdateRuntimeConfig updates runtime settings for client monitoring.
func (r *ClientRegistry) UpdateRuntimeConfig(ctx context.Context, prev, next domain.RuntimeConfig) error {
	prevInterval := prev.ClientCheckInterval()
	nextInterval := next.ClientCheckInterval()
	if prevInterval == nextInterval {
		return nil
	}

	r.mu.Lock()
	cancel := r.monitorCancel
	r.monitorCancel = nil
	r.monitorStarted = false
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if nextInterval <= 0 {
		return nil
	}
	r.StartMonitor(ctx)
	return nil
}

// RegisterClient registers a client and returns registration metadata.
func (r *ClientRegistry) RegisterClient(ctx context.Context, client string, pid int, tags []string, server string) (domain.ClientRegistration, error) {
	if client == "" {
		return domain.ClientRegistration{}, errors.New("client is required")
	}
	if pid <= 0 {
		return domain.ClientRegistration{}, errors.New("pid must be > 0")
	}
	normalizedTags := normalizeTags(tags)
	normalizedServer := normalizeServerName(server)
	if normalizedServer != "" && len(normalizedTags) > 0 {
		return domain.ClientRegistration{}, errors.New("server and tags are mutually exclusive")
	}
	visibleSpecKeys, visibleServerCount := r.visibleSpecKeys(normalizedTags, normalizedServer)
	internalClient := isInternalClientName(client)

	var toActivate []string
	var toDeactivate []string
	now := time.Now()
	var snapshot domain.ActiveClientSnapshot
	var shouldBroadcast bool
	var selectorChanged bool

	r.mu.Lock()
	if existing, ok := r.activeClients[client]; ok {
		selectorChanged = !tagsEqual(existing.tags, normalizedTags) || existing.server != normalizedServer
		if existing.pid == pid && !selectorChanged {
			existing.lastHeartbeat = now
			r.activeClients[client] = existing
			r.mu.Unlock()
			return domain.ClientRegistration{
				Client:             client,
				Tags:               normalizedTags,
				VisibleServerCount: visibleServerCount,
			}, nil
		}
		if !internalClient {
			toActivate, toDeactivate = diffKeys(visibleSpecKeys, existing.specKeys)
		}
		existing.pid = pid
		existing.tags = normalizedTags
		existing.server = normalizedServer
		existing.specKeys = visibleSpecKeys
		existing.lastHeartbeat = now
		r.activeClients[client] = existing
		if !internalClient {
			applySpecDelta(r.specCounts, toActivate, toDeactivate)
		}
		snapshot = r.snapshotActiveClientsLocked(now)
		shouldBroadcast = !internalClient
	} else {
		r.activeClients[client] = clientState{
			pid:           pid,
			tags:          normalizedTags,
			server:        normalizedServer,
			specKeys:      visibleSpecKeys,
			lastHeartbeat: now,
		}
		if !internalClient {
			applySpecDelta(r.specCounts, visibleSpecKeys, nil)
			toActivate = visibleSpecKeys
		}
		snapshot = r.snapshotActiveClientsLocked(now)
		shouldBroadcast = !internalClient
	}
	r.mu.Unlock()

	toActivate, toDeactivate = filterOverlap(toActivate, toDeactivate)

	if !internalClient {
		if err := r.activateSpecs(ctx, toActivate, client); err != nil {
			_ = r.UnregisterClient(ctx, client)
			return domain.ClientRegistration{}, err
		}
		if err := r.deactivateSpecs(ctx, toDeactivate); err != nil {
			r.state.logger.Warn("spec deactivation failed", zap.Error(err))
		}
	}
	if shouldBroadcast {
		r.broadcastActiveClients(finalizeActiveClientSnapshot(snapshot))
	}
	if selectorChanged {
		r.broadcastClientChange(ClientChangeEvent{Client: client})
	}

	return domain.ClientRegistration{
		Client:             client,
		Tags:               normalizedTags,
		VisibleServerCount: visibleServerCount,
	}, nil
}

// UnregisterClient unregisters a client.
func (r *ClientRegistry) UnregisterClient(ctx context.Context, client string) error {
	if client == "" {
		return errors.New("client is required")
	}
	internalClient := isInternalClientName(client)
	var toDeactivate []string
	var snapshot domain.ActiveClientSnapshot
	var shouldBroadcast bool

	r.mu.Lock()
	state, ok := r.activeClients[client]
	if ok {
		delete(r.activeClients, client)
		if !internalClient {
			applySpecDelta(r.specCounts, nil, state.specKeys)
			toDeactivate = state.specKeys
		}
		snapshot = r.snapshotActiveClientsLocked(time.Now())
		shouldBroadcast = !internalClient
	}
	r.mu.Unlock()

	if !internalClient {
		if err := r.deactivateSpecs(ctx, toDeactivate); err != nil {
			r.state.logger.Warn("spec deactivation failed", zap.Error(err))
		}
	}
	if shouldBroadcast {
		r.broadcastActiveClients(finalizeActiveClientSnapshot(snapshot))
	}
	return nil
}

// ListActiveClients lists active clients.
func (r *ClientRegistry) ListActiveClients(_ context.Context) ([]domain.ActiveClient, error) {
	now := time.Now()
	r.mu.Lock()
	snapshot := r.snapshotActiveClientsLocked(now)
	r.mu.Unlock()
	return finalizeActiveClientSnapshot(snapshot).Clients, nil
}

// WatchActiveClients streams active client updates.
func (r *ClientRegistry) WatchActiveClients(ctx context.Context) (<-chan domain.ActiveClientSnapshot, error) {
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
func (r *ClientRegistry) WatchClientChanges(ctx context.Context) <-chan ClientChangeEvent {
	ch := make(chan ClientChangeEvent, 16)
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

func (r *ClientRegistry) resolveClientTags(client string) ([]string, error) {
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

func (r *ClientRegistry) resolveClientServer(client string) (string, error) {
	if client == "" {
		return "", domain.ErrClientNotRegistered
	}
	r.mu.Lock()
	state, ok := r.activeClients[client]
	if ok {
		state.lastHeartbeat = time.Now()
		r.activeClients[client] = state
	}
	r.mu.Unlock()
	if !ok {
		return "", domain.ErrClientNotRegistered
	}
	return state.server, nil
}

func (r *ClientRegistry) resolveVisibleSpecKeys(client string) ([]string, error) {
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

func (r *ClientRegistry) visibleSpecKeys(tags []string, server string) ([]string, int) {
	catalog := r.state.Catalog()
	serverSpecKeys := r.state.ServerSpecKeys()
	if len(serverSpecKeys) == 0 {
		return nil, 0
	}
	if server != "" {
		specKey, ok := serverSpecKeys[server]
		if !ok {
			return nil, 0
		}
		if _, ok := catalog.Specs[server]; !ok {
			return nil, 0
		}
		return []string{specKey}, 1
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

func (r *ClientRegistry) activateSpecs(ctx context.Context, specKeys []string, client string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	runtime := r.state.Runtime()
	registry := r.state.SpecRegistry()

	type activationTask struct {
		specKey  string
		minReady int
		cause    domain.StartCause
	}

	// filter the specs that need to be started (reference count from 0 to 1)
	r.mu.Lock()
	specsToStart := make([]string, 0, len(order))
	for _, specKey := range order {
		// only start if going from 0 to 1
		if r.specCounts[specKey] == 1 {
			specsToStart = append(specsToStart, specKey)
		}
	}
	r.mu.Unlock()

	// Prepare activation tasks outside the lock to avoid lock inversion.
	tasks := make([]activationTask, 0, len(specsToStart))
	for _, specKey := range specsToStart {
		spec, ok := registry[specKey]
		if !ok {
			return errors.New("unknown spec key " + specKey)
		}
		minReady := bootstrap.ActiveMinReady(spec)
		cause := bootstrap.ClientStartCause(runtime, spec, client, minReady)
		tasks = append(tasks, activationTask{
			specKey:  specKey,
			minReady: minReady,
			cause:    cause,
		})
	}

	// Perform the actual start operations outside the lock.
	for _, task := range tasks {
		causeCtx := domain.WithStartCause(ctx, task.cause)
		if r.state.initManager != nil {
			err := r.state.initManager.SetMinReady(task.specKey, task.minReady, task.cause)
			if err == nil {
				continue
			}
			r.state.logger.Warn("server init manager failed to set min ready", zap.String("specKey", task.specKey), zap.Error(err))
		}
		if r.state.scheduler == nil {
			return errors.New("scheduler not configured")
		}
		if err := r.state.scheduler.SetDesiredMinReady(causeCtx, task.specKey, task.minReady); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClientRegistry) deactivateSpecs(ctx context.Context, specKeys []string) error {
	if len(specKeys) == 0 {
		return nil
	}
	order := append([]string(nil), specKeys...)
	sort.Strings(order)
	runtime := r.state.Runtime()
	registry := r.state.SpecRegistry()
	var firstErr error

	// 关键修复：先在锁内过滤出引用计数为 0 的 spec
	r.mu.Lock()
	specsToStop := make([]string, 0, len(order))
	for _, specKey := range order {
		// 只停止引用计数为 0 的 spec（没有客户端在使用）
		if r.specCounts[specKey] > 0 {
			continue
		}
		spec, ok := registry[specKey]
		if ok && bootstrap.ResolveActivationMode(runtime, spec) == domain.ActivationAlwaysOn {
			continue
		}
		specsToStop = append(specsToStop, specKey)
	}
	r.mu.Unlock()

	// 在锁外执行实际的停止操作
	for _, specKey := range specsToStop {
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

func (r *ClientRegistry) reapDeadClients(ctx context.Context) {
	now := time.Now()
	runtime := r.state.Runtime()
	timeout := runtime.ClientCheckInterval() * clientReapTimeoutMultiplier
	inactiveTimeout := runtime.ClientInactiveInterval()
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

func (r *ClientRegistry) snapshotActiveClientsLocked(now time.Time) domain.ActiveClientSnapshot {
	clients := make([]domain.ActiveClient, 0, len(r.activeClients))
	for client, state := range r.activeClients {
		if isInternalClientName(client) {
			continue
		}
		clients = append(clients, domain.ActiveClient{
			Client:        client,
			PID:           state.pid,
			Tags:          append([]string(nil), state.tags...),
			Server:        state.server,
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

func (r *ClientRegistry) broadcastActiveClients(snapshot domain.ActiveClientSnapshot) {
	subs := r.copyActiveClientSubscribers()
	for _, ch := range subs {
		sendActiveClientSnapshot(ch, snapshot)
	}
}

// ApplyCatalogUpdate updates client state based on catalog changes.
func (r *ClientRegistry) ApplyCatalogUpdate(ctx context.Context, update domain.CatalogUpdate) error {
	now := time.Now()

	r.mu.Lock()
	oldSpecCounts := copyCounts(r.specCounts)
	newSpecCounts := make(map[string]int)
	changedClients := make([]string, 0)

	for client, state := range r.activeClients {
		nextSpecKeys, _ := visibleSpecKeysForCatalog(update.Snapshot.Catalog, update.Snapshot.Summary.ServerSpecKeys, state.tags, state.server)
		if !sameKeySet(state.specKeys, nextSpecKeys) {
			changedClients = append(changedClients, client)
			state.specKeys = nextSpecKeys
			r.activeClients[client] = state
		}
		if !isInternalClientName(client) {
			for _, specKey := range nextSpecKeys {
				newSpecCounts[specKey]++
			}
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
		r.broadcastClientChange(ClientChangeEvent{Client: client})
	}
	return nil
}

func (r *ClientRegistry) copyActiveClientSubscribers() []chan domain.ActiveClientSnapshot {
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

func (r *ClientRegistry) broadcastClientChange(event ClientChangeEvent) {
	r.mu.Lock()
	subs := make([]chan ClientChangeEvent, 0, len(r.clientChangeSubs))
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
		counts[key]++
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

func visibleSpecKeysForCatalog(catalog domain.Catalog, serverSpecKeys map[string]string, tags []string, server string) ([]string, int) {
	if len(serverSpecKeys) == 0 {
		return nil, 0
	}
	if server != "" {
		specKey, ok := serverSpecKeys[server]
		if !ok {
			return nil, 0
		}
		if _, ok := catalog.Specs[server]; !ok {
			return nil, 0
		}
		return []string{specKey}, 1
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

func normalizeServerName(server string) string {
	return strings.TrimSpace(server)
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
