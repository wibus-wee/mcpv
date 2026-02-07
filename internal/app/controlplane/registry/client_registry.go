package registry

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
)

type ClientRegistry struct {
	state    State
	resolver *VisibilityResolver
	probe    CallerProbe

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

func NewClientRegistry(state State) *ClientRegistry {
	return &ClientRegistry{
		state:            state,
		resolver:         NewVisibilityResolver(state),
		probe:            NewCallerProbe(),
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
		ctx = r.state.Context()
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
	normalizedTags := r.resolver.NormalizeTags(tags)
	normalizedServer := r.resolver.NormalizeServerName(server)
	if normalizedServer != "" && len(normalizedTags) > 0 {
		return domain.ClientRegistration{}, errors.New("server and tags are mutually exclusive")
	}
	visibleSpecKeys, visibleServerCount := r.resolver.VisibleSpecKeys(normalizedTags, normalizedServer)
	internalClient := isInternalClientName(client)

	var toActivate []string
	var toDeactivate []string
	now := time.Now()
	var snapshot domain.ActiveClientSnapshot
	var shouldBroadcast bool
	var selectorChanged bool

	r.mu.Lock()
	if existing, ok := r.activeClients[client]; ok {
		selectorChanged = !r.resolver.TagsEqual(existing.tags, normalizedTags) || existing.server != normalizedServer
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
			r.state.Logger().Warn("spec deactivation failed", zap.Error(err))
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
			r.state.Logger().Warn("spec deactivation failed", zap.Error(err))
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
