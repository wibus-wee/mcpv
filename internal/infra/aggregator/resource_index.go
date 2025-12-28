package aggregator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/telemetry"
)

type ResourceIndex struct {
	router   domain.Router
	specs    map[string]domain.ServerSpec
	specKeys map[string]string
	cfg      domain.RuntimeConfig
	logger   *zap.Logger
	health   *telemetry.HealthTracker
	gate     *RefreshGate

	mu          sync.Mutex
	started     bool
	ticker      *time.Ticker
	stop        chan struct{}
	serverCache map[string]resourceCache
	subs        map[chan domain.ResourceSnapshot]struct{}
	refreshBeat *telemetry.Heartbeat
	state       atomic.Value
	reqBuilder  requestBuilder
}

type resourceCache struct {
	resources []domain.ResourceDefinition
	targets   map[string]domain.ResourceTarget
}

type resourceIndexState struct {
	snapshot domain.ResourceSnapshot
	targets  map[string]domain.ResourceTarget
}

func NewResourceIndex(rt domain.Router, specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig, logger *zap.Logger, health *telemetry.HealthTracker, gate *RefreshGate) *ResourceIndex {
	if logger == nil {
		logger = zap.NewNop()
	}
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	index := &ResourceIndex{
		router:      rt,
		specs:       specs,
		specKeys:    specKeys,
		cfg:         cfg,
		logger:      logger.Named("resource_index"),
		health:      health,
		gate:        gate,
		stop:        make(chan struct{}),
		serverCache: make(map[string]resourceCache),
		subs:        make(map[chan domain.ResourceSnapshot]struct{}),
	}
	index.state.Store(resourceIndexState{
		snapshot: domain.ResourceSnapshot{},
		targets:  make(map[string]domain.ResourceTarget),
	})
	return index
}

func (a *ResourceIndex) Start(ctx context.Context) {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return
	}
	a.started = true
	if a.stop == nil {
		a.stop = make(chan struct{})
	}
	a.mu.Unlock()

	interval := time.Duration(a.cfg.ToolRefreshSeconds) * time.Second
	if interval > 0 && a.health != nil && a.refreshBeat == nil {
		a.refreshBeat = a.health.Register("resource_index.refresh", interval*3)
	}
	if a.refreshBeat != nil {
		a.refreshBeat.Beat()
	}
	if err := a.refresh(ctx); err != nil {
		a.logger.Warn("initial resource refresh failed", zap.Error(err))
	}
	if interval <= 0 {
		return
	}

	a.mu.Lock()
	if a.ticker != nil {
		a.mu.Unlock()
		return
	}
	a.ticker = time.NewTicker(interval)
	a.mu.Unlock()

	go func() {
		for {
			select {
			case <-a.ticker.C:
				if a.refreshBeat != nil {
					a.refreshBeat.Beat()
				}
				if err := a.refresh(ctx); err != nil {
					a.logger.Warn("resource refresh failed", zap.Error(err))
				}
			case <-a.stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (a *ResourceIndex) Stop() {
	a.mu.Lock()
	if a.ticker != nil {
		a.ticker.Stop()
		a.ticker = nil
	}
	if a.refreshBeat != nil {
		a.refreshBeat.Stop()
		a.refreshBeat = nil
	}
	if a.stop != nil {
		close(a.stop)
		a.stop = nil
	}
	a.started = false
	a.mu.Unlock()
}

func (a *ResourceIndex) Snapshot() domain.ResourceSnapshot {
	state := a.state.Load().(resourceIndexState)
	return copyResourceSnapshot(state.snapshot)
}

func (a *ResourceIndex) Subscribe(ctx context.Context) <-chan domain.ResourceSnapshot {
	ch := make(chan domain.ResourceSnapshot, 1)

	a.mu.Lock()
	a.subs[ch] = struct{}{}
	a.mu.Unlock()

	state := a.state.Load().(resourceIndexState)
	sendResourceSnapshot(ch, state.snapshot)

	go func() {
		<-ctx.Done()
		a.mu.Lock()
		delete(a.subs, ch)
		a.mu.Unlock()
	}()

	return ch
}

func (a *ResourceIndex) Resolve(uri string) (domain.ResourceTarget, bool) {
	state := a.state.Load().(resourceIndexState)
	target, ok := state.targets[uri]
	return target, ok
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

func (a *ResourceIndex) refresh(ctx context.Context) error {
	if err := a.gate.Acquire(ctx); err != nil {
		return err
	}
	defer a.gate.Release()

	serverTypes := sortedServerTypes(a.specs)
	if len(serverTypes) == 0 {
		return nil
	}

	type refreshResult struct {
		serverType string
		resources  []domain.ResourceDefinition
		targets    map[string]domain.ResourceTarget
		err        error
	}

	results := make(chan refreshResult, len(serverTypes))
	timeout := refreshTimeout(a.cfg)
	workerCount := refreshWorkerCount(a.cfg, len(serverTypes))
	if workerCount == 0 {
		return nil
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case serverType, ok := <-jobs:
					if !ok {
						return
					}
					spec := a.specs[serverType]
					fetchCtx, cancel := context.WithTimeout(ctx, timeout)
					resources, targets, err := a.fetchServerResources(fetchCtx, serverType, spec)
					cancel()
					results <- refreshResult{
						serverType: serverType,
						resources:  resources,
						targets:    targets,
						err:        err,
					}
				}
			}
		}()
	}

	go func() {
		for _, serverType := range serverTypes {
			jobs <- serverType
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.err != nil {
			if errors.Is(res.err, domain.ErrNoReadyInstance) {
				continue
			}
			if errors.Is(res.err, domain.ErrMethodNotAllowed) {
				a.mu.Lock()
				delete(a.serverCache, res.serverType)
				a.mu.Unlock()
				a.rebuildSnapshot()
				continue
			}
			a.logger.Warn("resource list fetch failed", zap.String("serverType", res.serverType), zap.Error(res.err))
			continue
		}

		a.mu.Lock()
		a.serverCache[res.serverType] = resourceCache{
			resources: res.resources,
			targets:   res.targets,
		}
		a.mu.Unlock()
		a.rebuildSnapshot()
	}
	return nil
}

func (a *ResourceIndex) rebuildSnapshot() {
	cache := a.copyServerCache()
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

	etag := hashResources(merged)
	state := a.state.Load().(resourceIndexState)
	if etag == state.snapshot.ETag {
		return
	}

	snapshot := domain.ResourceSnapshot{
		ETag:      etag,
		Resources: merged,
	}
	a.state.Store(resourceIndexState{
		snapshot: snapshot,
		targets:  targets,
	})
	a.broadcast(snapshot)
}

func (a *ResourceIndex) broadcast(snapshot domain.ResourceSnapshot) {
	subs := a.copySubscribers()
	for _, ch := range subs {
		sendResourceSnapshot(ch, snapshot)
	}
}

func (a *ResourceIndex) copyServerCache() map[string]resourceCache {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make(map[string]resourceCache, len(a.serverCache))
	for key, cache := range a.serverCache {
		out[key] = cache
	}
	return out
}

func (a *ResourceIndex) copySubscribers() []chan domain.ResourceSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]chan domain.ResourceSnapshot, 0, len(a.subs))
	for ch := range a.subs {
		out = append(out, ch)
	}
	return out
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
		raw, err := json.Marshal(&resourceCopy)
		if err != nil {
			a.logger.Warn("marshal resource failed", zap.String("serverType", serverType), zap.String("uri", resource.URI), zap.Error(err))
			continue
		}
		result = append(result, domain.ResourceDefinition{
			URI:          resource.URI,
			ResourceJSON: raw,
		})
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
	hasher := sha256.New()
	for _, resource := range resources {
		_, _ = hasher.Write([]byte(resource.URI))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(resource.ResourceJSON)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func copyResourceSnapshot(snapshot domain.ResourceSnapshot) domain.ResourceSnapshot {
	out := domain.ResourceSnapshot{
		ETag:      snapshot.ETag,
		Resources: make([]domain.ResourceDefinition, 0, len(snapshot.Resources)),
	}
	for _, resource := range snapshot.Resources {
		raw := make([]byte, len(resource.ResourceJSON))
		copy(raw, resource.ResourceJSON)
		out.Resources = append(out.Resources, domain.ResourceDefinition{
			URI:          resource.URI,
			ResourceJSON: raw,
		})
	}
	return out
}

func sendResourceSnapshot(ch chan domain.ResourceSnapshot, snapshot domain.ResourceSnapshot) {
	select {
	case ch <- copyResourceSnapshot(snapshot):
	default:
	}
}
