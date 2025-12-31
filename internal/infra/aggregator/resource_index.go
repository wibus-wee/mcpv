package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mcpcodec"
	"mcpd/internal/infra/telemetry"
)

type ResourceIndex struct {
	router      domain.Router
	specs       map[string]domain.ServerSpec
	specKeys    map[string]string
	cfg         domain.RuntimeConfig
	logger      *zap.Logger
	health      *telemetry.HealthTracker
	gate        *RefreshGate
	listChanges listChangeSubscriber
	specKeySet  map[string]struct{}

	reqBuilder requestBuilder
	index      *GenericIndex[domain.ResourceSnapshot, domain.ResourceTarget, resourceCache]
}

type resourceCache struct {
	resources []domain.ResourceDefinition
	targets   map[string]domain.ResourceTarget
	etag      string
}

func NewResourceIndex(rt domain.Router, specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig, logger *zap.Logger, health *telemetry.HealthTracker, gate *RefreshGate, listChanges listChangeSubscriber) *ResourceIndex {
	if logger == nil {
		logger = zap.NewNop()
	}
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	resourceIndex := &ResourceIndex{
		router:      rt,
		specs:       specs,
		specKeys:    specKeys,
		cfg:         cfg,
		logger:      logger.Named("resource_index"),
		health:      health,
		gate:        gate,
		listChanges: listChanges,
		specKeySet:  specKeySet(specKeys),
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

func (a *ResourceIndex) Start(ctx context.Context) {
	a.index.Start(ctx)
	a.startListChangeListener(ctx)
}

func (a *ResourceIndex) Stop() {
	a.index.Stop()
}

func (a *ResourceIndex) Refresh(ctx context.Context) error {
	return a.index.Refresh(ctx)
}

func (a *ResourceIndex) Snapshot() domain.ResourceSnapshot {
	return a.index.Snapshot()
}

func (a *ResourceIndex) Subscribe(ctx context.Context) <-chan domain.ResourceSnapshot {
	return a.index.Subscribe(ctx)
}

func (a *ResourceIndex) Resolve(uri string) (domain.ResourceTarget, bool) {
	return a.index.Resolve(uri)
}

func (a *ResourceIndex) ReadResource(ctx context.Context, uri string) (json.RawMessage, error) {
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

func (a *ResourceIndex) UpdateSpecs(specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig) {
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	a.specs = specs
	a.specKeys = specKeys
	a.specKeySet = specKeySet(specKeys)
	a.cfg = cfg
	a.index.UpdateSpecs(specs, cfg)
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
				if !listChangeApplies(a.specs, a.specKeySet, event) {
					continue
				}
				if err := a.index.Refresh(ctx); err != nil {
					a.logger.Warn("resource refresh after list change failed", zap.Error(err))
				}
			}
		}
	}()
}

func (a *ResourceIndex) buildSnapshot(cache map[string]resourceCache) (domain.ResourceSnapshot, map[string]domain.ResourceTarget) {
	merged := make([]domain.ResourceDefinition, 0)
	targets := make(map[string]domain.ResourceTarget)

	serverTypes := sortedServerTypes(cache)
	for _, serverType := range serverTypes {
		server := cache[serverType]
		resources := append([]domain.ResourceDefinition(nil), server.resources...)
		sort.Slice(resources, func(i, j int) bool { return resources[i].URI < resources[j].URI })

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

	return domain.ResourceSnapshot{
		ETag:      hashResources(merged),
		Resources: merged,
	}, targets
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
		return resourceCache{}, err
	}
	return resourceCache{resources: resources, targets: targets, etag: hashResources(resources)}, nil
}

func (a *ResourceIndex) fetchServerResources(ctx context.Context, serverType string, spec domain.ServerSpec) ([]domain.ResourceDefinition, map[string]domain.ResourceTarget, error) {
	specKey := a.specKeys[serverType]
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

func hashResources(resources []domain.ResourceDefinition) string {
	return mcpcodec.HashResourceDefinitions(resources)
}

func copyResourceSnapshot(snapshot domain.ResourceSnapshot) domain.ResourceSnapshot {
	return domain.CloneResourceSnapshot(snapshot)
}
