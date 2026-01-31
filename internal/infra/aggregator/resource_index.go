package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/hashutil"
	"mcpv/internal/infra/mcpcodec"
	"mcpv/internal/infra/telemetry"
)

// ResourceIndex aggregates resource metadata across specs and supports reads.
type ResourceIndex struct {
	router               domain.Router
	specs                map[string]domain.ServerSpec
	specKeys             map[string]string
	cfg                  domain.RuntimeConfig
	metadataCache        *domain.MetadataCache
	logger               *zap.Logger
	health               *telemetry.HealthTracker
	gate                 *RefreshGate
	listChanges          listChangeSubscriber
	specsMu              sync.RWMutex
	specKeySet           map[string]struct{}
	bootstrapWaiter      BootstrapWaiter
	bootstrapWaiterMu    sync.RWMutex
	bootstrapRefreshOnce sync.Once
	baseMu               sync.RWMutex
	baseCtx              context.Context
	baseCancel           context.CancelFunc
	serverMu             sync.RWMutex
	serverSnapshots      map[string]serverResourceSnapshot

	reqBuilder requestBuilder
	index      *GenericIndex[domain.ResourceSnapshot, domain.ResourceTarget, resourceCache]
}

type resourceCache struct {
	resources []domain.ResourceDefinition
	targets   map[string]domain.ResourceTarget
	etag      string
}

type serverResourceSnapshot struct {
	snapshot domain.ResourceSnapshot
	targets  map[string]domain.ResourceTarget
}

// NewResourceIndex builds a ResourceIndex for the provided runtime configuration.
func NewResourceIndex(rt domain.Router, specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig, metadataCache *domain.MetadataCache, logger *zap.Logger, health *telemetry.HealthTracker, gate *RefreshGate, listChanges listChangeSubscriber) *ResourceIndex {
	if logger == nil {
		logger = zap.NewNop()
	}
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	resourceIndex := &ResourceIndex{
		router:          rt,
		specs:           specs,
		specKeys:        specKeys,
		cfg:             cfg,
		metadataCache:   metadataCache,
		logger:          logger.Named("resource_index"),
		health:          health,
		gate:            gate,
		listChanges:     listChanges,
		specKeySet:      specKeySet(specKeys),
		serverSnapshots: map[string]serverResourceSnapshot{},
	}
	resourceIndex.index = NewGenericIndex(GenericIndexOptions[domain.ResourceSnapshot, domain.ResourceTarget, resourceCache]{
		Name:              "resource_index",
		LogLabel:          "resource",
		FetchErrorMessage: "resource list fetch failed",
		Specs:             specs,
		Config:            cfg,
		Logger:            resourceIndex.logger,
		Health:            health,
		Gate:              gate,
		EmptySnapshot:     func() domain.ResourceSnapshot { return domain.ResourceSnapshot{} },
		CopySnapshot:      copyResourceSnapshot,
		SnapshotETag:      func(snapshot domain.ResourceSnapshot) string { return snapshot.ETag },
		BuildSnapshot:     resourceIndex.buildSnapshot,
		CacheETag:         func(cache resourceCache) string { return cache.etag },
		Fetch:             resourceIndex.fetchServerCache,
		OnRefreshError:    resourceIndex.refreshErrorDecision,
		ShouldStart:       func(domain.RuntimeConfig) bool { return true },
	})
	return resourceIndex
}

// Start begins periodic refresh and list change tracking.
func (a *ResourceIndex) Start(ctx context.Context) {
	baseCtx := a.setBaseContext(ctx)
	a.index.Start(baseCtx)
	a.startListChangeListener(baseCtx)
	a.startBootstrapRefresh(baseCtx)
}

// Stop halts refresh activity and cancels bootstrap waits.
func (a *ResourceIndex) Stop() {
	a.index.Stop()
	a.clearBaseContext()
}

// Refresh fetches resource metadata on demand.
func (a *ResourceIndex) Refresh(ctx context.Context) error {
	return a.index.Refresh(ctx)
}

// Snapshot returns the latest resource snapshot.
func (a *ResourceIndex) Snapshot() domain.ResourceSnapshot {
	return a.index.Snapshot()
}

// SnapshotForServer returns the latest resource snapshot for a server.
func (a *ResourceIndex) SnapshotForServer(serverName string) (domain.ResourceSnapshot, bool) {
	if serverName == "" {
		return domain.ResourceSnapshot{}, false
	}
	a.serverMu.RLock()
	entry, ok := a.serverSnapshots[serverName]
	a.serverMu.RUnlock()
	if !ok {
		return domain.ResourceSnapshot{}, false
	}
	return domain.CloneResourceSnapshot(entry.snapshot), true
}

// Subscribe streams resource snapshot updates.
func (a *ResourceIndex) Subscribe(ctx context.Context) <-chan domain.ResourceSnapshot {
	return a.index.Subscribe(ctx)
}

// Resolve locates a resource target by URI.
func (a *ResourceIndex) Resolve(uri string) (domain.ResourceTarget, bool) {
	return a.index.Resolve(uri)
}

// ResolveForServer locates a resource target for a server by URI.
func (a *ResourceIndex) ResolveForServer(serverName, uri string) (domain.ResourceTarget, bool) {
	if serverName == "" || uri == "" {
		return domain.ResourceTarget{}, false
	}
	a.serverMu.RLock()
	entry, ok := a.serverSnapshots[serverName]
	a.serverMu.RUnlock()
	if !ok {
		return domain.ResourceTarget{}, false
	}
	target, ok := entry.targets[uri]
	return target, ok
}

// SetBootstrapWaiter registers a bootstrap completion hook.
func (a *ResourceIndex) SetBootstrapWaiter(waiter BootstrapWaiter) {
	a.bootstrapWaiterMu.Lock()
	a.bootstrapWaiter = waiter
	a.bootstrapWaiterMu.Unlock()
	if baseCtx := a.baseContext(); baseCtx != nil {
		a.startBootstrapRefresh(baseCtx)
	}
}

// ReadResource routes a resource read to the owning server.
func (a *ResourceIndex) ReadResource(ctx context.Context, uri string) (json.RawMessage, error) {
	// Wait for bootstrap completion if needed
	a.bootstrapWaiterMu.RLock()
	waiter := a.bootstrapWaiter
	a.bootstrapWaiterMu.RUnlock()

	if waiter != nil {
		// Create context with 60s timeout for bootstrap wait
		waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		if err := waiter(waitCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("bootstrap timeout: %w", err)
			}
			return nil, fmt.Errorf("bootstrap wait failed: %w", err)
		}
	}

	target, ok := a.Resolve(uri)
	if !ok {
		return nil, domain.ErrResourceNotFound
	}

	params := &mcp.ReadResourceParams{
		URI: target.URI,
	}
	payload, err := a.reqBuilder.Build("resources/read", params)
	if err != nil {
		return nil, err
	}

	resp, err := a.router.Route(ctx, target.ServerType, target.SpecKey, "", payload)
	if err != nil {
		return nil, err
	}

	result, err := decodeReadResourceResult(resp)
	if err != nil {
		return nil, err
	}
	return marshalReadResourceResult(result)
}

// ReadResourceForServer routes a resource read to the owning server using a URI.
func (a *ResourceIndex) ReadResourceForServer(ctx context.Context, serverName, uri string) (json.RawMessage, error) {
	target, ok := a.ResolveForServer(serverName, uri)
	if !ok {
		return nil, domain.ErrResourceNotFound
	}

	params := &mcp.ReadResourceParams{
		URI: target.URI,
	}
	payload, err := a.reqBuilder.Build("resources/read", params)
	if err != nil {
		return nil, err
	}

	resp, err := a.router.Route(ctx, target.ServerType, target.SpecKey, "", payload)
	if err != nil {
		return nil, err
	}

	result, err := decodeReadResourceResult(resp)
	if err != nil {
		return nil, err
	}
	return marshalReadResourceResult(result)
}

// UpdateSpecs replaces the registry backing the resource index.
func (a *ResourceIndex) UpdateSpecs(specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig) {
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	if specs == nil {
		specs = map[string]domain.ServerSpec{}
	}
	specsCopy := make(map[string]domain.ServerSpec, len(specs))
	for key, value := range specs {
		specsCopy[key] = value
	}
	specKeysCopy := make(map[string]string, len(specKeys))
	for key, value := range specKeys {
		specKeysCopy[key] = value
	}
	specKeySetCopy := specKeySet(specKeysCopy)

	a.specsMu.Lock()
	a.specs = specsCopy
	a.specKeys = specKeysCopy
	a.specKeySet = specKeySetCopy
	a.cfg = cfg
	a.specsMu.Unlock()
	a.index.UpdateSpecs(specsCopy, cfg)
}

// ApplyRuntimeConfig updates runtime configuration and refresh scheduling.
func (a *ResourceIndex) ApplyRuntimeConfig(cfg domain.RuntimeConfig) {
	a.specsMu.Lock()
	prevCfg := a.cfg
	specsCopy := copyServerSpecs(a.specs)
	a.cfg = cfg
	a.specsMu.Unlock()
	a.index.UpdateSpecs(specsCopy, cfg)

	baseCtx := a.baseContext()
	if baseCtx == nil {
		return
	}
	if prevCfg.ToolRefreshInterval() != cfg.ToolRefreshInterval() {
		a.index.Stop()
		a.index.Start(baseCtx)
	}
}

func (a *ResourceIndex) startListChangeListener(ctx context.Context) {
	if a.listChanges == nil {
		return
	}
	ch := a.listChanges.Subscribe(ctx, domain.ListChangeResources)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				a.specsMu.RLock()
				specs := a.specs
				specKeySet := a.specKeySet
				a.specsMu.RUnlock()
				if !listChangeApplies(specs, specKeySet, event) {
					continue
				}
				if err := a.index.Refresh(ctx); err != nil {
					a.logger.Warn("resource refresh after list change failed", zap.Error(err))
				}
			}
		}
	}()
}

func (a *ResourceIndex) startBootstrapRefresh(ctx context.Context) {
	a.bootstrapWaiterMu.RLock()
	waiter := a.bootstrapWaiter
	a.bootstrapWaiterMu.RUnlock()
	if waiter == nil {
		return
	}
	if ctx == nil {
		return
	}
	a.bootstrapRefreshOnce.Do(func() {
		go func() {
			if err := waiter(ctx); err != nil {
				a.logger.Warn("resource bootstrap wait failed", zap.Error(err))
				return
			}

			a.specsMu.RLock()
			cfg := a.cfg
			a.specsMu.RUnlock()
			refreshCtx, cancel := withRefreshTimeout(ctx, cfg)
			defer cancel()
			if err := a.index.Refresh(refreshCtx); err != nil {
				a.logger.Warn("resource refresh after bootstrap failed", zap.Error(err))
			}
		}()
	})
}

func (a *ResourceIndex) setBaseContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	a.baseMu.Lock()
	if a.baseCtx == nil {
		baseCtx, cancel := context.WithCancel(ctx)
		a.baseCtx = baseCtx
		a.baseCancel = cancel
	}
	baseCtx := a.baseCtx
	a.baseMu.Unlock()
	return baseCtx
}

func (a *ResourceIndex) baseContext() context.Context {
	a.baseMu.RLock()
	defer a.baseMu.RUnlock()
	return a.baseCtx
}

func (a *ResourceIndex) clearBaseContext() {
	a.baseMu.Lock()
	if a.baseCancel != nil {
		a.baseCancel()
	}
	a.baseCtx = nil
	a.baseCancel = nil
	a.baseMu.Unlock()
}

func (a *ResourceIndex) buildSnapshot(cache map[string]resourceCache) (domain.ResourceSnapshot, map[string]domain.ResourceTarget) {
	merged := make([]domain.ResourceDefinition, 0)
	targets := make(map[string]domain.ResourceTarget)
	serverSnapshots := make(map[string]serverResourceSnapshot, len(cache))
	a.specsMu.RLock()
	specs := a.specs
	a.specsMu.RUnlock()

	serverTypes := sortedServerTypes(cache)
	for _, serverType := range serverTypes {
		server := cache[serverType]
		spec := specs[serverType]
		resources := append([]domain.ResourceDefinition(nil), server.resources...)
		sort.Slice(resources, func(i, j int) bool { return resources[i].URI < resources[j].URI })

		snapshot := serverResourceSnapshot{
			snapshot: domain.ResourceSnapshot{
				ETag:      a.hashResources(resources),
				Resources: resources,
			},
			targets: copyResourceTargets(server.targets),
		}
		if spec.Name == "" {
			a.logger.Warn("resource snapshot skipped: missing server name", zap.String("serverType", serverType))
		} else {
			serverSnapshots[spec.Name] = snapshot
		}

		for _, resource := range resources {
			target := server.targets[resource.URI]
			if _, exists := targets[resource.URI]; exists {
				a.logger.Warn("resource uri conflict", zap.String("serverType", serverType), zap.String("uri", resource.URI))
				continue
			}
			targets[resource.URI] = target
			merged = append(merged, resource)
		}
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].URI < merged[j].URI })

	a.storeServerSnapshots(serverSnapshots)

	return domain.ResourceSnapshot{
		ETag:      a.hashResources(merged),
		Resources: merged,
	}, targets
}

func (a *ResourceIndex) storeServerSnapshots(snapshots map[string]serverResourceSnapshot) {
	a.serverMu.Lock()
	a.serverSnapshots = snapshots
	a.serverMu.Unlock()
}

func copyResourceTargets(in map[string]domain.ResourceTarget) map[string]domain.ResourceTarget {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]domain.ResourceTarget, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (a *ResourceIndex) refreshErrorDecision(_ string, err error) refreshErrorDecision {
	if errors.Is(err, domain.ErrNoReadyInstance) {
		return refreshErrorSkip
	}
	if errors.Is(err, domain.ErrMethodNotAllowed) {
		return refreshErrorDropCache
	}
	return refreshErrorLog
}

func (a *ResourceIndex) fetchServerCache(ctx context.Context, serverType string, spec domain.ServerSpec) (resourceCache, error) {
	resources, targets, err := a.fetchServerResources(ctx, serverType, spec)
	if err != nil {
		if errors.Is(err, domain.ErrNoReadyInstance) {
			if cached, ok := a.cachedServerCache(serverType, spec); ok {
				return cached, nil
			}
		}
		return resourceCache{}, err
	}
	return resourceCache{resources: resources, targets: targets, etag: a.hashResources(resources)}, nil
}

func (a *ResourceIndex) cachedServerCache(serverType string, spec domain.ServerSpec) (resourceCache, bool) {
	if a.metadataCache == nil {
		return resourceCache{}, false
	}
	a.specsMu.RLock()
	specKey := a.specKeys[serverType]
	a.specsMu.RUnlock()
	if specKey == "" {
		return resourceCache{}, false
	}
	resources, ok := a.metadataCache.GetResources(specKey)
	if !ok {
		return resourceCache{}, false
	}

	result := make([]domain.ResourceDefinition, 0, len(resources))
	targets := make(map[string]domain.ResourceTarget)

	for _, resource := range resources {
		if resource.URI == "" {
			continue
		}
		resourceDef := resource
		resourceDef.SpecKey = specKey
		resourceDef.ServerName = spec.Name
		result = append(result, resourceDef)
		targets[resource.URI] = domain.ResourceTarget{
			ServerType: serverType,
			SpecKey:    specKey,
			URI:        resource.URI,
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].URI < result[j].URI })
	return resourceCache{resources: result, targets: targets, etag: a.hashResources(result)}, true
}

func (a *ResourceIndex) fetchServerResources(ctx context.Context, serverType string, spec domain.ServerSpec) ([]domain.ResourceDefinition, map[string]domain.ResourceTarget, error) {
	a.specsMu.RLock()
	specKey := a.specKeys[serverType]
	a.specsMu.RUnlock()
	if specKey == "" {
		return nil, nil, fmt.Errorf("missing spec key for server type %q", serverType)
	}
	resources, err := a.fetchResources(ctx, serverType, specKey)
	if err != nil {
		return nil, nil, err
	}

	result := make([]domain.ResourceDefinition, 0, len(resources))
	targets := make(map[string]domain.ResourceTarget)

	for _, resource := range resources {
		if resource == nil {
			continue
		}
		if resource.URI == "" {
			continue
		}
		resourceCopy := *resource
		def := mcpcodec.ResourceFromMCP(&resourceCopy)
		def.SpecKey = specKey
		def.ServerName = spec.Name
		result = append(result, def)
		targets[resource.URI] = domain.ResourceTarget{
			ServerType: serverType,
			SpecKey:    specKey,
			URI:        resource.URI,
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].URI < result[j].URI })
	return result, targets, nil
}

func (a *ResourceIndex) fetchResources(ctx context.Context, serverType, specKey string) ([]*mcp.Resource, error) {
	var resources []*mcp.Resource
	cursor := ""

	for {
		params := &mcp.ListResourcesParams{Cursor: cursor}
		payload, err := a.reqBuilder.Build("resources/list", params)
		if err != nil {
			return nil, err
		}

		resp, err := a.router.RouteWithOptions(ctx, serverType, specKey, "", payload, domain.RouteOptions{AllowStart: false})
		if err != nil {
			return nil, err
		}

		result, err := decodeListResourcesResult(resp)
		if err != nil {
			return nil, err
		}
		resources = append(resources, result.Resources...)
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return resources, nil
}

func decodeListResourcesResult(raw json.RawMessage) (*mcp.ListResourcesResult, error) {
	resp, err := decodeJSONRPCResponse(raw)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("resources/list error: %w", resp.Error)
	}

	if len(resp.Result) == 0 {
		return nil, errors.New("resources/list response missing result")
	}

	var result mcp.ListResourcesResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode resources/list result: %w", err)
	}
	return &result, nil
}

func decodeReadResourceResult(raw json.RawMessage) (*mcp.ReadResourceResult, error) {
	resp, err := decodeJSONRPCResponse(raw)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("resources/read error: %w", resp.Error)
	}

	if len(resp.Result) == 0 {
		return nil, errors.New("resources/read response missing result")
	}

	var result mcp.ReadResourceResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode resources/read result: %w", err)
	}
	return &result, nil
}

func marshalReadResourceResult(result *mcp.ReadResourceResult) (json.RawMessage, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func (a *ResourceIndex) hashResources(resources []domain.ResourceDefinition) string {
	return hashutil.ResourceETag(a.logger, resources)
}

func copyResourceSnapshot(snapshot domain.ResourceSnapshot) domain.ResourceSnapshot {
	return domain.CloneResourceSnapshot(snapshot)
}
