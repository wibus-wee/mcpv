package index

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/aggregator/core"
	"mcpv/internal/infra/hashutil"
	"mcpv/internal/infra/mcpcodec"
	"mcpv/internal/infra/telemetry"
)

// ResourceIndex aggregates resource metadata across specs and supports reads.
type ResourceIndex struct {
	base       *BaseIndex[domain.ResourceSnapshot, domain.ResourceTarget, resourceCache, serverResourceSnapshot]
	reqBuilder core.RequestBuilder
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
func NewResourceIndex(rt domain.Router, specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig, metadataCache *domain.MetadataCache, logger *zap.Logger, health *telemetry.HealthTracker, gate *core.RefreshGate, listChanges core.ListChangeSubscriber) *ResourceIndex {
	resourceIndex := &ResourceIndex{}
	hooks := BaseHooks[domain.ResourceSnapshot, domain.ResourceTarget, resourceCache]{
		Name:              "resource_index",
		LogLabel:          "resource",
		LoggerName:        "resource_index",
		FetchErrorMessage: "resource list fetch failed",
		ListChangeKind:    domain.ListChangeResources,
		ShouldStart:       func(domain.RuntimeConfig) bool { return true },
		ShouldListChange:  func(domain.RuntimeConfig) bool { return true },
		EmptySnapshot:     func() domain.ResourceSnapshot { return domain.ResourceSnapshot{} },
		CopySnapshot:      copyResourceSnapshot,
		SnapshotETag:      func(snapshot domain.ResourceSnapshot) string { return snapshot.ETag },
		BuildSnapshot:     resourceIndex.buildSnapshot,
		CacheETag:         func(cache resourceCache) string { return cache.etag },
		FetchServerCache:  resourceIndex.fetchServerCache,
		OnRefreshError:    resourceIndex.refreshErrorDecision,
	}
	resourceIndex.base = NewBaseIndex[domain.ResourceSnapshot, domain.ResourceTarget, resourceCache, serverResourceSnapshot](
		rt,
		specs,
		specKeys,
		cfg,
		metadataCache,
		logger,
		health,
		gate,
		listChanges,
		hooks,
	)
	return resourceIndex
}

// Start begins periodic refresh and list change tracking.
func (a *ResourceIndex) Start(ctx context.Context) {
	a.base.Start(ctx)
}

// Stop halts refresh activity and cancels bootstrap waits.
func (a *ResourceIndex) Stop() {
	a.base.Stop()
}

// Refresh fetches resource metadata on demand.
func (a *ResourceIndex) Refresh(ctx context.Context) error {
	return a.base.Refresh(ctx)
}

// Snapshot returns the latest resource snapshot.
func (a *ResourceIndex) Snapshot() domain.ResourceSnapshot {
	return a.base.Snapshot()
}

// SnapshotForServer returns the latest resource snapshot for a server.
func (a *ResourceIndex) SnapshotForServer(serverName string) (domain.ResourceSnapshot, bool) {
	entry, ok := a.base.SnapshotForServer(serverName)
	if !ok {
		return domain.ResourceSnapshot{}, false
	}
	return domain.CloneResourceSnapshot(entry.snapshot), true
}

// Subscribe streams resource snapshot updates.
func (a *ResourceIndex) Subscribe(ctx context.Context) <-chan domain.ResourceSnapshot {
	return a.base.Subscribe(ctx)
}

// Resolve locates a resource target by URI.
func (a *ResourceIndex) Resolve(uri string) (domain.ResourceTarget, bool) {
	return a.base.Resolve(uri)
}

// ResolveForServer locates a resource target for a server by URI.
func (a *ResourceIndex) ResolveForServer(serverName, uri string) (domain.ResourceTarget, bool) {
	if serverName == "" || uri == "" {
		return domain.ResourceTarget{}, false
	}
	entry, ok := a.base.SnapshotForServer(serverName)
	if !ok {
		return domain.ResourceTarget{}, false
	}
	target, ok := entry.targets[uri]
	return target, ok
}

// SetBootstrapWaiter registers a bootstrap completion hook.
func (a *ResourceIndex) SetBootstrapWaiter(waiter core.BootstrapWaiter) {
	a.base.SetBootstrapWaiter(waiter)
}

// ReadResource routes a resource read to the owning server.
func (a *ResourceIndex) ReadResource(ctx context.Context, uri string) (json.RawMessage, error) {
	// Wait for bootstrap completion if needed
	waiter := a.base.Waiter()

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

	resp, err := a.base.Router().Route(ctx, target.ServerType, target.SpecKey, "", payload)
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

	resp, err := a.base.Router().Route(ctx, target.ServerType, target.SpecKey, "", payload)
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
	a.base.UpdateSpecs(specs, specKeys, cfg)
}

// ApplyRuntimeConfig updates runtime configuration and refresh scheduling.
func (a *ResourceIndex) ApplyRuntimeConfig(cfg domain.RuntimeConfig) {
	a.base.ApplyRuntimeConfig(cfg)
}

func (a *ResourceIndex) buildSnapshot(cache map[string]resourceCache) (domain.ResourceSnapshot, map[string]domain.ResourceTarget) {
	merged := make([]domain.ResourceDefinition, 0)
	targets := make(map[string]domain.ResourceTarget)
	serverSnapshots := make(map[string]serverResourceSnapshot, len(cache))
	specs, _, _ := a.base.SpecsSnapshot()
	logger := a.base.Logger()

	serverTypes := core.SortedServerTypes(cache)
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
			logger.Warn("resource snapshot skipped: missing server name", zap.String("serverType", serverType))
		} else {
			serverSnapshots[spec.Name] = snapshot
		}

		for _, resource := range resources {
			target := server.targets[resource.URI]
			if _, exists := targets[resource.URI]; exists {
				logger.Warn("resource uri conflict", zap.String("serverType", serverType), zap.String("uri", resource.URI))
				continue
			}
			targets[resource.URI] = target
			merged = append(merged, resource)
		}
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].URI < merged[j].URI })

	a.base.StoreServerSnapshots(serverSnapshots)

	return domain.ResourceSnapshot{
		ETag:      a.hashResources(merged),
		Resources: merged,
	}, targets
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

func (a *ResourceIndex) refreshErrorDecision(_ string, err error) core.RefreshErrorDecision {
	if errors.Is(err, domain.ErrNoReadyInstance) {
		return core.RefreshErrorSkip
	}
	if errors.Is(err, domain.ErrMethodNotAllowed) {
		return core.RefreshErrorDropCache
	}
	return core.RefreshErrorLog
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
	metadataCache := a.base.MetadataCache()
	if metadataCache == nil {
		return resourceCache{}, false
	}
	_, specKeys, _ := a.base.SpecsSnapshot()
	specKey := specKeys[serverType]
	if specKey == "" {
		return resourceCache{}, false
	}
	resources, ok := metadataCache.GetResources(specKey)
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
	_, specKeys, _ := a.base.SpecsSnapshot()
	specKey := specKeys[serverType]
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

		resp, err := a.base.Router().RouteWithOptions(ctx, serverType, specKey, "", payload, domain.RouteOptions{AllowStart: false})
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
	return hashutil.ResourceETag(a.base.Logger(), resources)
}

func copyResourceSnapshot(snapshot domain.ResourceSnapshot) domain.ResourceSnapshot {
	return domain.CloneResourceSnapshot(snapshot)
}
