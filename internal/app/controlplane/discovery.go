package controlplane

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/hashutil"
)

type DiscoveryService struct {
	state    *State
	registry *ClientRegistry
}

const snapshotPageSize = 200

func NewDiscoveryService(state *State, registry *ClientRegistry) *DiscoveryService {
	return &DiscoveryService{
		state:    state,
		registry: registry,
	}
}

// StartClientChangeListener is a no-op for the server-centric discovery flow.
func (d *DiscoveryService) StartClientChangeListener(_ context.Context) {}

// ListTools lists tools visible to a client.
func (d *DiscoveryService) ListTools(_ context.Context, client string) (domain.ToolSnapshot, error) {
	serverName, err := d.registry.resolveClientServer(client)
	if err != nil {
		return domain.ToolSnapshot{}, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Tools() == nil {
		return domain.ToolSnapshot{}, nil
	}
	if serverName != "" {
		snapshot, ok := runtime.Tools().SnapshotForServer(serverName)
		if !ok {
			return domain.ToolSnapshot{}, nil
		}
		return snapshot, nil
	}
	visibleSpecKeys, err := d.registry.resolveVisibleSpecKeys(client)
	if err != nil {
		return domain.ToolSnapshot{}, err
	}
	return d.filterToolSnapshot(runtime.Tools().Snapshot(), visibleSpecKeys), nil
}

// ListToolCatalog returns the full tool catalog snapshot.
func (d *DiscoveryService) ListToolCatalog(_ context.Context) (domain.ToolCatalogSnapshot, error) {
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Tools() == nil {
		return domain.ToolCatalogSnapshot{}, nil
	}
	live := runtime.Tools().Snapshot().Tools
	cached := runtime.Tools().CachedSnapshot().Tools

	cachedAt := make(map[string]time.Time)
	if len(cached) > 0 {
		cache := d.metadataCache()
		if cache != nil {
			for _, tool := range cached {
				specKey := tool.SpecKey
				if specKey == "" {
					continue
				}
				if _, ok := cachedAt[specKey]; ok {
					continue
				}
				if ts, ok := cache.GetCachedAt(specKey); ok {
					cachedAt[specKey] = ts
				}
			}
		}
	}

	return buildToolCatalogSnapshot(d.state.logger, live, cached, cachedAt), nil
}

// WatchTools streams tool snapshots for a client.
func (d *DiscoveryService) WatchTools(ctx context.Context, client string) (<-chan domain.ToolSnapshot, error) {
	if _, err := d.registry.resolveClientServer(client); err != nil {
		return closedToolSnapshotChannel(), err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Tools() == nil {
		return closedToolSnapshotChannel(), nil
	}

	output := make(chan domain.ToolSnapshot, 1)
	indexCh := runtime.Tools().Subscribe(ctx)
	changes := d.registry.WatchClientChanges(ctx)

	go func() {
		defer close(output)
		var last domain.ToolSnapshot
		last = runtime.Tools().Snapshot()
		d.sendFilteredTools(output, client, last)
		for {
			select {
			case <-ctx.Done():
				return
			case snapshot, ok := <-indexCh:
				if !ok {
					return
				}
				last = snapshot
				d.sendFilteredTools(output, client, snapshot)
			case event, ok := <-changes:
				if !ok {
					return
				}
				if event.Client == client {
					d.sendFilteredTools(output, client, last)
				}
			}
		}
	}()

	return output, nil
}

// CallTool executes a tool on behalf of a client.
func (d *DiscoveryService) CallTool(ctx context.Context, client, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	serverName, err := d.registry.resolveClientServer(client)
	if err != nil {
		return nil, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Tools() == nil {
		return nil, domain.ErrToolNotFound
	}
	if serverName != "" {
		if _, ok := runtime.Tools().ResolveForServer(serverName, name); !ok {
			return nil, domain.ErrToolNotFound
		}
		ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: client})
		ctx = domain.WithStartCause(ctx, domain.StartCause{
			Reason:   domain.StartCauseToolCall,
			Client:   client,
			ToolName: name,
		})
		return runtime.Tools().CallToolForServer(ctx, serverName, name, args, routingKey)
	}
	visibleSpecKeys, err := d.registry.resolveVisibleSpecKeys(client)
	if err != nil {
		return nil, err
	}
	target, ok := runtime.Tools().Resolve(name)
	if !ok {
		return nil, domain.ErrToolNotFound
	}
	visibleSpecSet := toSpecKeySet(visibleSpecKeys)
	if target.SpecKey != "" {
		if _, ok := visibleSpecSet[target.SpecKey]; !ok {
			return nil, domain.ErrToolNotFound
		}
	} else if !d.isServerVisible(visibleSpecSet, target.ServerType) {
		return nil, domain.ErrToolNotFound
	}
	ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: client})
	ctx = domain.WithStartCause(ctx, domain.StartCause{
		Reason:   domain.StartCauseToolCall,
		Client:   client,
		ToolName: name,
	})
	return runtime.Tools().CallTool(ctx, name, args, routingKey)
}

// CallToolAll executes a tool without client visibility checks.
func (d *DiscoveryService) CallToolAll(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Tools() == nil {
		return nil, domain.ErrToolNotFound
	}
	if _, ok := runtime.Tools().Resolve(name); !ok {
		return nil, domain.ErrToolNotFound
	}
	ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: domain.InternalUIClientName})
	ctx = domain.WithStartCause(ctx, domain.StartCause{
		Reason:   domain.StartCauseToolCall,
		Client:   domain.InternalUIClientName,
		ToolName: name,
	})
	return runtime.Tools().CallTool(ctx, name, args, routingKey)
}

// ListResources lists resources visible to a client.
func (d *DiscoveryService) ListResources(_ context.Context, client string, cursor string) (domain.ResourcePage, error) {
	serverName, err := d.registry.resolveClientServer(client)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Resources() == nil {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}
	if serverName != "" {
		snapshot, ok := runtime.Resources().SnapshotForServer(serverName)
		if !ok {
			return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
		}
		return paginateResources(snapshot, cursor)
	}
	visibleSpecKeys, err := d.registry.resolveVisibleSpecKeys(client)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	snapshot := runtime.Resources().Snapshot()
	filtered := d.filterResourceSnapshot(snapshot, visibleSpecKeys)
	return paginateResources(filtered, cursor)
}

// ListResourcesAll lists resources across all servers.
func (d *DiscoveryService) ListResourcesAll(_ context.Context, cursor string) (domain.ResourcePage, error) {
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Resources() == nil {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}
	snapshot := runtime.Resources().Snapshot()
	return paginateResources(snapshot, cursor)
}

// WatchResources streams resource snapshots for a client.
func (d *DiscoveryService) WatchResources(ctx context.Context, client string) (<-chan domain.ResourceSnapshot, error) {
	if _, err := d.registry.resolveClientServer(client); err != nil {
		return closedResourceSnapshotChannel(), err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Resources() == nil {
		return closedResourceSnapshotChannel(), nil
	}

	output := make(chan domain.ResourceSnapshot, 1)
	indexCh := runtime.Resources().Subscribe(ctx)
	changes := d.registry.WatchClientChanges(ctx)

	go func() {
		defer close(output)
		var last domain.ResourceSnapshot
		last = runtime.Resources().Snapshot()
		d.sendFilteredResources(output, client, last)
		for {
			select {
			case <-ctx.Done():
				return
			case snapshot, ok := <-indexCh:
				if !ok {
					return
				}
				last = snapshot
				d.sendFilteredResources(output, client, snapshot)
			case event, ok := <-changes:
				if !ok {
					return
				}
				if event.Client == client {
					d.sendFilteredResources(output, client, last)
				}
			}
		}
	}()

	return output, nil
}

// ReadResource reads a resource on behalf of a client.
func (d *DiscoveryService) ReadResource(ctx context.Context, client, uri string) (json.RawMessage, error) {
	serverName, err := d.registry.resolveClientServer(client)
	if err != nil {
		return nil, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Resources() == nil {
		return nil, domain.ErrResourceNotFound
	}
	if serverName != "" {
		if _, ok := runtime.Resources().ResolveForServer(serverName, uri); !ok {
			return nil, domain.ErrResourceNotFound
		}
		ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: client})
		return runtime.Resources().ReadResourceForServer(ctx, serverName, uri)
	}
	visibleSpecKeys, err := d.registry.resolveVisibleSpecKeys(client)
	if err != nil {
		return nil, err
	}
	target, ok := runtime.Resources().Resolve(uri)
	if !ok {
		return nil, domain.ErrResourceNotFound
	}
	visibleSpecSet := toSpecKeySet(visibleSpecKeys)
	if target.SpecKey != "" {
		if _, ok := visibleSpecSet[target.SpecKey]; !ok {
			return nil, domain.ErrResourceNotFound
		}
	} else if !d.isServerVisible(visibleSpecSet, target.ServerType) {
		return nil, domain.ErrResourceNotFound
	}
	ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: client})
	return runtime.Resources().ReadResource(ctx, uri)
}

// ReadResourceAll reads a resource without client visibility checks.
func (d *DiscoveryService) ReadResourceAll(ctx context.Context, uri string) (json.RawMessage, error) {
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Resources() == nil {
		return nil, domain.ErrResourceNotFound
	}
	if _, ok := runtime.Resources().Resolve(uri); !ok {
		return nil, domain.ErrResourceNotFound
	}
	ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: domain.InternalUIClientName})
	return runtime.Resources().ReadResource(ctx, uri)
}

// ListPrompts lists prompts visible to a client.
func (d *DiscoveryService) ListPrompts(_ context.Context, client string, cursor string) (domain.PromptPage, error) {
	serverName, err := d.registry.resolveClientServer(client)
	if err != nil {
		return domain.PromptPage{}, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Prompts() == nil {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}
	if serverName != "" {
		snapshot, ok := runtime.Prompts().SnapshotForServer(serverName)
		if !ok {
			return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
		}
		return paginatePrompts(snapshot, cursor)
	}
	visibleSpecKeys, err := d.registry.resolveVisibleSpecKeys(client)
	if err != nil {
		return domain.PromptPage{}, err
	}
	snapshot := runtime.Prompts().Snapshot()
	filtered := d.filterPromptSnapshot(snapshot, visibleSpecKeys)
	return paginatePrompts(filtered, cursor)
}

// ListPromptsAll lists prompts across all servers.
func (d *DiscoveryService) ListPromptsAll(_ context.Context, cursor string) (domain.PromptPage, error) {
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Prompts() == nil {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}
	snapshot := runtime.Prompts().Snapshot()
	return paginatePrompts(snapshot, cursor)
}

// WatchPrompts streams prompt snapshots for a client.
func (d *DiscoveryService) WatchPrompts(ctx context.Context, client string) (<-chan domain.PromptSnapshot, error) {
	if _, err := d.registry.resolveClientServer(client); err != nil {
		return closedPromptSnapshotChannel(), err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Prompts() == nil {
		return closedPromptSnapshotChannel(), nil
	}

	output := make(chan domain.PromptSnapshot, 1)
	indexCh := runtime.Prompts().Subscribe(ctx)
	changes := d.registry.WatchClientChanges(ctx)

	go func() {
		defer close(output)
		var last domain.PromptSnapshot
		last = runtime.Prompts().Snapshot()
		d.sendFilteredPrompts(output, client, last)
		for {
			select {
			case <-ctx.Done():
				return
			case snapshot, ok := <-indexCh:
				if !ok {
					return
				}
				last = snapshot
				d.sendFilteredPrompts(output, client, snapshot)
			case event, ok := <-changes:
				if !ok {
					return
				}
				if event.Client == client {
					d.sendFilteredPrompts(output, client, last)
				}
			}
		}
	}()

	return output, nil
}

// GetPrompt resolves a prompt for a client.
func (d *DiscoveryService) GetPrompt(ctx context.Context, client, name string, args json.RawMessage) (json.RawMessage, error) {
	serverName, err := d.registry.resolveClientServer(client)
	if err != nil {
		return nil, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Prompts() == nil {
		return nil, domain.ErrPromptNotFound
	}
	if serverName != "" {
		if _, ok := runtime.Prompts().ResolveForServer(serverName, name); !ok {
			return nil, domain.ErrPromptNotFound
		}
		ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: client})
		return runtime.Prompts().GetPromptForServer(ctx, serverName, name, args)
	}
	visibleSpecKeys, err := d.registry.resolveVisibleSpecKeys(client)
	if err != nil {
		return nil, err
	}
	target, ok := runtime.Prompts().Resolve(name)
	if !ok {
		return nil, domain.ErrPromptNotFound
	}
	visibleSpecSet := toSpecKeySet(visibleSpecKeys)
	if target.SpecKey != "" {
		if _, ok := visibleSpecSet[target.SpecKey]; !ok {
			return nil, domain.ErrPromptNotFound
		}
	} else if !d.isServerVisible(visibleSpecSet, target.ServerType) {
		return nil, domain.ErrPromptNotFound
	}
	ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: client})
	return runtime.Prompts().GetPrompt(ctx, name, args)
}

// GetPromptAll resolves a prompt without client visibility checks.
func (d *DiscoveryService) GetPromptAll(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Prompts() == nil {
		return nil, domain.ErrPromptNotFound
	}
	if _, ok := runtime.Prompts().Resolve(name); !ok {
		return nil, domain.ErrPromptNotFound
	}
	ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: domain.InternalUIClientName})
	return runtime.Prompts().GetPrompt(ctx, name, args)
}

// GetToolSnapshotForClient returns the tool snapshot for a client.
func (d *DiscoveryService) GetToolSnapshotForClient(client string) (domain.ToolSnapshot, error) {
	return d.ListTools(context.Background(), client)
}

func (d *DiscoveryService) sendFilteredTools(ch chan<- domain.ToolSnapshot, client string, snapshot domain.ToolSnapshot) {
	serverName, err := d.registry.resolveClientServer(client)
	if err != nil {
		return
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Tools() == nil {
		return
	}
	if serverName != "" {
		serverSnapshot, ok := runtime.Tools().SnapshotForServer(serverName)
		if !ok {
			return
		}
		select {
		case ch <- serverSnapshot:
		default:
		}
		return
	}
	visibleSpecKeys, err := d.registry.resolveVisibleSpecKeys(client)
	if err != nil {
		return
	}
	filtered := d.filterToolSnapshot(snapshot, visibleSpecKeys)
	select {
	case ch <- filtered:
	default:
	}
}

func (d *DiscoveryService) sendFilteredResources(ch chan<- domain.ResourceSnapshot, client string, snapshot domain.ResourceSnapshot) {
	serverName, err := d.registry.resolveClientServer(client)
	if err != nil {
		return
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Resources() == nil {
		return
	}
	if serverName != "" {
		serverSnapshot, ok := runtime.Resources().SnapshotForServer(serverName)
		if !ok {
			return
		}
		select {
		case ch <- serverSnapshot:
		default:
		}
		return
	}
	visibleSpecKeys, err := d.registry.resolveVisibleSpecKeys(client)
	if err != nil {
		return
	}
	filtered := d.filterResourceSnapshot(snapshot, visibleSpecKeys)
	select {
	case ch <- filtered:
	default:
	}
}

func (d *DiscoveryService) sendFilteredPrompts(ch chan<- domain.PromptSnapshot, client string, snapshot domain.PromptSnapshot) {
	serverName, err := d.registry.resolveClientServer(client)
	if err != nil {
		return
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Prompts() == nil {
		return
	}
	if serverName != "" {
		serverSnapshot, ok := runtime.Prompts().SnapshotForServer(serverName)
		if !ok {
			return
		}
		select {
		case ch <- serverSnapshot:
		default:
		}
		return
	}
	visibleSpecKeys, err := d.registry.resolveVisibleSpecKeys(client)
	if err != nil {
		return
	}
	filtered := d.filterPromptSnapshot(snapshot, visibleSpecKeys)
	select {
	case ch <- filtered:
	default:
	}
}

func (d *DiscoveryService) filterToolSnapshot(snapshot domain.ToolSnapshot, visibleSpecKeys []string) domain.ToolSnapshot {
	if len(snapshot.Tools) == 0 {
		return domain.ToolSnapshot{}
	}
	visibleServers, visibleSpecSet := d.visibleServers(visibleSpecKeys)
	filtered := make([]domain.ToolDefinition, 0, len(snapshot.Tools))
	for _, tool := range snapshot.Tools {
		if tool.ServerName != "" {
			if _, ok := visibleServers[tool.ServerName]; !ok {
				continue
			}
		} else if tool.SpecKey != "" {
			if _, ok := visibleSpecSet[tool.SpecKey]; !ok {
				continue
			}
		}
		filtered = append(filtered, tool)
	}
	if len(filtered) == 0 {
		return domain.ToolSnapshot{}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].SpecKey != filtered[j].SpecKey {
			return filtered[i].SpecKey < filtered[j].SpecKey
		}
		if filtered[i].Name != filtered[j].Name {
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].ServerName < filtered[j].ServerName
	})
	return domain.ToolSnapshot{
		ETag:  hashutil.ToolETag(d.state.logger, filtered),
		Tools: filtered,
	}
}

func (d *DiscoveryService) filterResourceSnapshot(snapshot domain.ResourceSnapshot, visibleSpecKeys []string) domain.ResourceSnapshot {
	if len(snapshot.Resources) == 0 {
		return domain.ResourceSnapshot{}
	}
	visibleServers, visibleSpecSet := d.visibleServers(visibleSpecKeys)
	filtered := make([]domain.ResourceDefinition, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		if resource.ServerName != "" {
			if _, ok := visibleServers[resource.ServerName]; !ok {
				continue
			}
		} else if resource.SpecKey != "" {
			if _, ok := visibleSpecSet[resource.SpecKey]; !ok {
				continue
			}
		}
		filtered = append(filtered, resource)
	}
	if len(filtered) == 0 {
		return domain.ResourceSnapshot{}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].URI < filtered[j].URI
	})
	return domain.ResourceSnapshot{
		ETag:      hashutil.ResourceETag(d.state.logger, filtered),
		Resources: filtered,
	}
}

func (d *DiscoveryService) filterPromptSnapshot(snapshot domain.PromptSnapshot, visibleSpecKeys []string) domain.PromptSnapshot {
	if len(snapshot.Prompts) == 0 {
		return domain.PromptSnapshot{}
	}
	visibleServers, visibleSpecSet := d.visibleServers(visibleSpecKeys)
	filtered := make([]domain.PromptDefinition, 0, len(snapshot.Prompts))
	for _, prompt := range snapshot.Prompts {
		if prompt.ServerName != "" {
			if _, ok := visibleServers[prompt.ServerName]; !ok {
				continue
			}
		} else if prompt.SpecKey != "" {
			if _, ok := visibleSpecSet[prompt.SpecKey]; !ok {
				continue
			}
		}
		filtered = append(filtered, prompt)
	}
	if len(filtered) == 0 {
		return domain.PromptSnapshot{}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})
	return domain.PromptSnapshot{
		ETag:    hashutil.PromptETag(d.state.logger, filtered),
		Prompts: filtered,
	}
}

func (d *DiscoveryService) visibleServers(visibleSpecKeys []string) (map[string]struct{}, map[string]struct{}) {
	visibleServers := make(map[string]struct{})
	visibleSpecSet := make(map[string]struct{})
	specRegistry := d.state.SpecRegistry()
	for _, specKey := range visibleSpecKeys {
		spec, ok := specRegistry[specKey]
		if !ok {
			continue
		}
		if spec.Name != "" {
			visibleServers[spec.Name] = struct{}{}
		}
		visibleSpecSet[specKey] = struct{}{}
	}
	return visibleServers, visibleSpecSet
}

func (d *DiscoveryService) isServerVisible(visibleSpecKeys map[string]struct{}, serverName string) bool {
	if serverName == "" {
		return false
	}
	serverSpecKeys := d.state.ServerSpecKeys()
	specKey, ok := serverSpecKeys[serverName]
	if !ok {
		return false
	}
	_, ok = visibleSpecKeys[specKey]
	return ok
}

func toSpecKeySet(keys []string) map[string]struct{} {
	if len(keys) == 0 {
		return map[string]struct{}{}
	}
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		set[key] = struct{}{}
	}
	return set
}

func (d *DiscoveryService) metadataCache() *domain.MetadataCache {
	runtime := d.state.RuntimeState()
	if runtime == nil {
		return nil
	}
	return runtime.MetadataCache()
}

func closedToolSnapshotChannel() chan domain.ToolSnapshot {
	ch := make(chan domain.ToolSnapshot)
	close(ch)
	return ch
}

func closedResourceSnapshotChannel() chan domain.ResourceSnapshot {
	ch := make(chan domain.ResourceSnapshot)
	close(ch)
	return ch
}

func closedPromptSnapshotChannel() chan domain.PromptSnapshot {
	ch := make(chan domain.PromptSnapshot)
	close(ch)
	return ch
}

func paginateResources(snapshot domain.ResourceSnapshot, cursor string) (domain.ResourcePage, error) {
	resources := snapshot.Resources
	start := 0
	if cursor != "" {
		start = indexAfterResourceCursor(resources, cursor)
		if start < 0 {
			return domain.ResourcePage{}, domain.ErrInvalidCursor
		}
	}

	end := start + snapshotPageSize
	if end > len(resources) {
		end = len(resources)
	}
	nextCursor := ""
	if end < len(resources) {
		nextCursor = resources[end-1].URI
	}
	page := domain.ResourceSnapshot{
		ETag:      snapshot.ETag,
		Resources: append([]domain.ResourceDefinition(nil), resources[start:end]...),
	}
	return domain.ResourcePage{Snapshot: page, NextCursor: nextCursor}, nil
}

func paginatePrompts(snapshot domain.PromptSnapshot, cursor string) (domain.PromptPage, error) {
	prompts := snapshot.Prompts
	start := 0
	if cursor != "" {
		start = indexAfterPromptCursor(prompts, cursor)
		if start < 0 {
			return domain.PromptPage{}, domain.ErrInvalidCursor
		}
	}

	end := start + snapshotPageSize
	if end > len(prompts) {
		end = len(prompts)
	}
	nextCursor := ""
	if end < len(prompts) {
		nextCursor = prompts[end-1].Name
	}
	page := domain.PromptSnapshot{
		ETag:    snapshot.ETag,
		Prompts: append([]domain.PromptDefinition(nil), prompts[start:end]...),
	}
	return domain.PromptPage{Snapshot: page, NextCursor: nextCursor}, nil
}

func indexAfterResourceCursor(resources []domain.ResourceDefinition, cursor string) int {
	for i, resource := range resources {
		if resource.URI == cursor {
			return i + 1
		}
	}
	return -1
}

func indexAfterPromptCursor(prompts []domain.PromptDefinition, cursor string) int {
	for i, prompt := range prompts {
		if prompt.Name == cursor {
			return i + 1
		}
	}
	return -1
}

func buildToolCatalogSnapshot(logger *zap.Logger, liveTools []domain.ToolDefinition, cachedTools []domain.ToolDefinition, cachedAt map[string]time.Time) domain.ToolCatalogSnapshot {
	entries := make(map[string]domain.ToolCatalogEntry)
	for _, tool := range cachedTools {
		entries[toolCatalogKey(tool)] = buildToolCatalogEntry(tool, domain.ToolSourceCache, cachedAt)
	}
	for _, tool := range liveTools {
		entries[toolCatalogKey(tool)] = buildToolCatalogEntry(tool, domain.ToolSourceLive, cachedAt)
	}

	list := make([]domain.ToolCatalogEntry, 0, len(entries))
	for _, entry := range entries {
		list = append(list, entry)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Definition.SpecKey != list[j].Definition.SpecKey {
			return list[i].Definition.SpecKey < list[j].Definition.SpecKey
		}
		if list[i].Definition.Name != list[j].Definition.Name {
			return list[i].Definition.Name < list[j].Definition.Name
		}
		return list[i].Definition.ServerName < list[j].Definition.ServerName
	})

	return domain.ToolCatalogSnapshot{
		Tools: list,
		ETag:  hashutil.ToolCatalogETag(logger, list),
	}
}

func buildToolCatalogEntry(tool domain.ToolDefinition, source domain.ToolSource, cachedAt map[string]time.Time) domain.ToolCatalogEntry {
	entry := domain.ToolCatalogEntry{
		Definition: tool,
		Source:     source,
	}
	if ts, ok := cachedAt[tool.SpecKey]; ok {
		entry.CachedAt = ts
	}
	return entry
}

func toolCatalogKey(tool domain.ToolDefinition) string {
	name := tool.Name
	specKey := tool.SpecKey
	if specKey == "" {
		specKey = tool.ServerName
	}
	return specKey + "\x00" + name
}
