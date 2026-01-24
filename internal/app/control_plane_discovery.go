package app

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/hashutil"
)

type discoveryService struct {
	state    *controlPlaneState
	registry *clientRegistry
}

const snapshotPageSize = 200

func newDiscoveryService(state *controlPlaneState, registry *clientRegistry) *discoveryService {
	return &discoveryService{
		state:    state,
		registry: registry,
	}
}

// StartClientChangeListener is a no-op for the server-centric discovery flow.
func (d *discoveryService) StartClientChangeListener(ctx context.Context) {}

// ListTools lists tools visible to a client.
func (d *discoveryService) ListTools(ctx context.Context, client string) (domain.ToolSnapshot, error) {
	tags, err := d.registry.resolveClientTags(client)
	if err != nil {
		return domain.ToolSnapshot{}, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.tools == nil {
		return domain.ToolSnapshot{}, nil
	}
	return d.filterToolSnapshot(runtime.tools.Snapshot(), tags), nil
}

// ListToolCatalog returns the full tool catalog snapshot.
func (d *discoveryService) ListToolCatalog(ctx context.Context) (domain.ToolCatalogSnapshot, error) {
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.tools == nil {
		return domain.ToolCatalogSnapshot{}, nil
	}
	live := runtime.tools.Snapshot().Tools
	cached := runtime.tools.CachedSnapshot().Tools

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
func (d *discoveryService) WatchTools(ctx context.Context, client string) (<-chan domain.ToolSnapshot, error) {
	if _, err := d.registry.resolveClientTags(client); err != nil {
		return closedToolSnapshotChannel(), err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.tools == nil {
		return closedToolSnapshotChannel(), nil
	}

	output := make(chan domain.ToolSnapshot, 1)
	indexCh := runtime.tools.Subscribe(ctx)
	changes := d.registry.WatchClientChanges(ctx)

	go func() {
		defer close(output)
		var last domain.ToolSnapshot
		last = runtime.tools.Snapshot()
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
func (d *discoveryService) CallTool(ctx context.Context, client, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	tags, err := d.registry.resolveClientTags(client)
	if err != nil {
		return nil, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.tools == nil {
		return nil, domain.ErrToolNotFound
	}
	target, ok := runtime.tools.Resolve(name)
	if !ok {
		return nil, domain.ErrToolNotFound
	}
	if !d.isServerVisible(tags, target.ServerType) {
		return nil, domain.ErrToolNotFound
	}
	ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: client})
	ctx = domain.WithStartCause(ctx, domain.StartCause{
		Reason:   domain.StartCauseToolCall,
		Client:   client,
		ToolName: name,
	})
	return runtime.tools.CallTool(ctx, name, args, routingKey)
}

// ListResources lists resources visible to a client.
func (d *discoveryService) ListResources(ctx context.Context, client string, cursor string) (domain.ResourcePage, error) {
	tags, err := d.registry.resolveClientTags(client)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.resources == nil {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}
	snapshot := runtime.resources.Snapshot()
	filtered := d.filterResourceSnapshot(snapshot, tags)
	return paginateResources(filtered, cursor)
}

// WatchResources streams resource snapshots for a client.
func (d *discoveryService) WatchResources(ctx context.Context, client string) (<-chan domain.ResourceSnapshot, error) {
	if _, err := d.registry.resolveClientTags(client); err != nil {
		return closedResourceSnapshotChannel(), err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.resources == nil {
		return closedResourceSnapshotChannel(), nil
	}

	output := make(chan domain.ResourceSnapshot, 1)
	indexCh := runtime.resources.Subscribe(ctx)
	changes := d.registry.WatchClientChanges(ctx)

	go func() {
		defer close(output)
		var last domain.ResourceSnapshot
		last = runtime.resources.Snapshot()
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
func (d *discoveryService) ReadResource(ctx context.Context, client, uri string) (json.RawMessage, error) {
	tags, err := d.registry.resolveClientTags(client)
	if err != nil {
		return nil, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.resources == nil {
		return nil, domain.ErrResourceNotFound
	}
	target, ok := runtime.resources.Resolve(uri)
	if !ok {
		return nil, domain.ErrResourceNotFound
	}
	if !d.isServerVisible(tags, target.ServerType) {
		return nil, domain.ErrResourceNotFound
	}
	ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: client})
	return runtime.resources.ReadResource(ctx, uri)
}

// ListPrompts lists prompts visible to a client.
func (d *discoveryService) ListPrompts(ctx context.Context, client string, cursor string) (domain.PromptPage, error) {
	tags, err := d.registry.resolveClientTags(client)
	if err != nil {
		return domain.PromptPage{}, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.prompts == nil {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}
	snapshot := runtime.prompts.Snapshot()
	filtered := d.filterPromptSnapshot(snapshot, tags)
	return paginatePrompts(filtered, cursor)
}

// WatchPrompts streams prompt snapshots for a client.
func (d *discoveryService) WatchPrompts(ctx context.Context, client string) (<-chan domain.PromptSnapshot, error) {
	if _, err := d.registry.resolveClientTags(client); err != nil {
		return closedPromptSnapshotChannel(), err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.prompts == nil {
		return closedPromptSnapshotChannel(), nil
	}

	output := make(chan domain.PromptSnapshot, 1)
	indexCh := runtime.prompts.Subscribe(ctx)
	changes := d.registry.WatchClientChanges(ctx)

	go func() {
		defer close(output)
		var last domain.PromptSnapshot
		last = runtime.prompts.Snapshot()
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
func (d *discoveryService) GetPrompt(ctx context.Context, client, name string, args json.RawMessage) (json.RawMessage, error) {
	tags, err := d.registry.resolveClientTags(client)
	if err != nil {
		return nil, err
	}
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.prompts == nil {
		return nil, domain.ErrPromptNotFound
	}
	target, ok := runtime.prompts.Resolve(name)
	if !ok {
		return nil, domain.ErrPromptNotFound
	}
	if !d.isServerVisible(tags, target.ServerType) {
		return nil, domain.ErrPromptNotFound
	}
	ctx = domain.WithRouteContext(ctx, domain.RouteContext{Client: client})
	return runtime.prompts.GetPrompt(ctx, name, args)
}

// GetToolSnapshotForClient returns the tool snapshot for a client.
func (d *discoveryService) GetToolSnapshotForClient(client string) (domain.ToolSnapshot, error) {
	return d.ListTools(context.Background(), client)
}

func (d *discoveryService) sendFilteredTools(ch chan<- domain.ToolSnapshot, client string, snapshot domain.ToolSnapshot) {
	tags, err := d.registry.resolveClientTags(client)
	if err != nil {
		return
	}
	filtered := d.filterToolSnapshot(snapshot, tags)
	select {
	case ch <- filtered:
	default:
	}
}

func (d *discoveryService) sendFilteredResources(ch chan<- domain.ResourceSnapshot, client string, snapshot domain.ResourceSnapshot) {
	tags, err := d.registry.resolveClientTags(client)
	if err != nil {
		return
	}
	filtered := d.filterResourceSnapshot(snapshot, tags)
	select {
	case ch <- filtered:
	default:
	}
}

func (d *discoveryService) sendFilteredPrompts(ch chan<- domain.PromptSnapshot, client string, snapshot domain.PromptSnapshot) {
	tags, err := d.registry.resolveClientTags(client)
	if err != nil {
		return
	}
	filtered := d.filterPromptSnapshot(snapshot, tags)
	select {
	case ch <- filtered:
	default:
	}
}

func (d *discoveryService) filterToolSnapshot(snapshot domain.ToolSnapshot, tags []string) domain.ToolSnapshot {
	if len(snapshot.Tools) == 0 {
		return domain.ToolSnapshot{}
	}
	visibleServers, visibleSpecKeys := d.visibleServers(tags)
	filtered := make([]domain.ToolDefinition, 0, len(snapshot.Tools))
	for _, tool := range snapshot.Tools {
		if tool.ServerName != "" {
			if _, ok := visibleServers[tool.ServerName]; !ok {
				continue
			}
		} else if tool.SpecKey != "" {
			if _, ok := visibleSpecKeys[tool.SpecKey]; !ok {
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

func (d *discoveryService) filterResourceSnapshot(snapshot domain.ResourceSnapshot, tags []string) domain.ResourceSnapshot {
	if len(snapshot.Resources) == 0 {
		return domain.ResourceSnapshot{}
	}
	visibleServers, visibleSpecKeys := d.visibleServers(tags)
	filtered := make([]domain.ResourceDefinition, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		if resource.ServerName != "" {
			if _, ok := visibleServers[resource.ServerName]; !ok {
				continue
			}
		} else if resource.SpecKey != "" {
			if _, ok := visibleSpecKeys[resource.SpecKey]; !ok {
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

func (d *discoveryService) filterPromptSnapshot(snapshot domain.PromptSnapshot, tags []string) domain.PromptSnapshot {
	if len(snapshot.Prompts) == 0 {
		return domain.PromptSnapshot{}
	}
	visibleServers, visibleSpecKeys := d.visibleServers(tags)
	filtered := make([]domain.PromptDefinition, 0, len(snapshot.Prompts))
	for _, prompt := range snapshot.Prompts {
		if prompt.ServerName != "" {
			if _, ok := visibleServers[prompt.ServerName]; !ok {
				continue
			}
		} else if prompt.SpecKey != "" {
			if _, ok := visibleSpecKeys[prompt.SpecKey]; !ok {
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

func (d *discoveryService) visibleServers(tags []string) (map[string]struct{}, map[string]struct{}) {
	catalog := d.state.Catalog()
	serverSpecKeys := d.state.ServerSpecKeys()
	visibleServers := make(map[string]struct{})
	visibleSpecKeys := make(map[string]struct{})
	for name, specKey := range serverSpecKeys {
		spec, ok := catalog.Specs[name]
		if !ok {
			continue
		}
		if isVisibleToTags(tags, spec.Tags) {
			visibleServers[name] = struct{}{}
			visibleSpecKeys[specKey] = struct{}{}
		}
	}
	return visibleServers, visibleSpecKeys
}

func (d *discoveryService) isServerVisible(tags []string, serverName string) bool {
	if serverName == "" {
		return false
	}
	catalog := d.state.Catalog()
	spec, ok := catalog.Specs[serverName]
	if !ok {
		return false
	}
	return isVisibleToTags(tags, spec.Tags)
}

func (d *discoveryService) metadataCache() *domain.MetadataCache {
	runtime := d.state.RuntimeState()
	if runtime == nil {
		return nil
	}
	return runtime.metadataCache
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
