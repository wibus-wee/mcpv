package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"mcpd/internal/domain"
)

type discoveryService struct {
	state    *controlPlaneState
	registry *callerRegistry
}

const snapshotPageSize = 200

func newDiscoveryService(state *controlPlaneState, registry *callerRegistry) *discoveryService {
	return &discoveryService{state: state, registry: registry}
}

func (d *discoveryService) ListTools(ctx context.Context, caller string) (domain.ToolSnapshot, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return domain.ToolSnapshot{}, err
	}
	if profile.tools == nil {
		return domain.ToolSnapshot{}, nil
	}
	return profile.tools.Snapshot(), nil
}

func (d *discoveryService) ListToolsAllProfiles(ctx context.Context) (domain.ToolSnapshot, error) {
	profileNames := d.registry.activeProfileNames()
	if len(profileNames) == 0 {
		return domain.ToolSnapshot{}, nil
	}

	merged := make([]domain.ToolDefinition, 0)
	seen := make(map[string]struct{})

	for _, name := range profileNames {
		runtime := d.state.profiles[name]
		if runtime == nil || runtime.tools == nil {
			continue
		}
		snapshot := runtime.tools.Snapshot()
		for _, tool := range snapshot.Tools {
			key := tool.SpecKey
			if key == "" {
				key = tool.ServerName
			}
			if key == "" {
				key = tool.Name
			}
			dedupeKey := key + "\x00" + tool.Name
			if _, ok := seen[dedupeKey]; ok {
				continue
			}
			seen[dedupeKey] = struct{}{}
			merged = append(merged, tool)
		}
	}

	if len(merged) == 0 {
		return domain.ToolSnapshot{}, nil
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].SpecKey != merged[j].SpecKey {
			return merged[i].SpecKey < merged[j].SpecKey
		}
		if merged[i].Name != merged[j].Name {
			return merged[i].Name < merged[j].Name
		}
		return merged[i].ServerName < merged[j].ServerName
	})

	return domain.ToolSnapshot{
		ETag:  hashTools(merged),
		Tools: merged,
	}, nil
}

func (d *discoveryService) WatchTools(ctx context.Context, caller string) (<-chan domain.ToolSnapshot, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return closedToolSnapshotChannel(), err
	}
	if profile.tools == nil {
		return closedToolSnapshotChannel(), nil
	}
	return profile.tools.Subscribe(ctx), nil
}

func (d *discoveryService) CallTool(ctx context.Context, caller, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return nil, err
	}
	if profile.tools == nil {
		return nil, domain.ErrToolNotFound
	}
	return profile.tools.CallTool(ctx, name, args, routingKey)
}

func (d *discoveryService) CallToolAllProfiles(ctx context.Context, name string, args json.RawMessage, routingKey, specKey string) (json.RawMessage, error) {
	profileNames := d.registry.activeProfileNames()
	for _, profileName := range profileNames {
		runtime := d.state.profiles[profileName]
		if runtime == nil || runtime.tools == nil {
			continue
		}
		if specKey != "" && !d.registry.profileContainsSpecKey(runtime, specKey) {
			continue
		}
		result, err := runtime.tools.CallTool(ctx, name, args, routingKey)
		if err == nil {
			return result, nil
		}
		if errors.Is(err, domain.ErrToolNotFound) {
			continue
		}
		return nil, err
	}
	return nil, domain.ErrToolNotFound
}

func (d *discoveryService) ListResources(ctx context.Context, caller string, cursor string) (domain.ResourcePage, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	if profile.resources == nil {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}
	snapshot := profile.resources.Snapshot()
	return paginateResources(snapshot, cursor)
}

func (d *discoveryService) ListResourcesAllProfiles(ctx context.Context, cursor string) (domain.ResourcePage, error) {
	profileNames := d.registry.activeProfileNames()
	if len(profileNames) == 0 {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}

	merged := make([]domain.ResourceDefinition, 0)
	seen := make(map[string]struct{})

	for _, profileName := range profileNames {
		runtime := d.state.profiles[profileName]
		if runtime == nil || runtime.resources == nil {
			continue
		}
		snapshot := runtime.resources.Snapshot()
		for _, resource := range snapshot.Resources {
			if resource.URI == "" {
				continue
			}
			if _, ok := seen[resource.URI]; ok {
				continue
			}
			seen[resource.URI] = struct{}{}
			merged = append(merged, resource)
		}
	}

	if len(merged) == 0 {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].URI < merged[j].URI })
	snapshot := domain.ResourceSnapshot{
		ETag:      hashResources(merged),
		Resources: merged,
	}
	return paginateResources(snapshot, cursor)
}

func (d *discoveryService) WatchResources(ctx context.Context, caller string) (<-chan domain.ResourceSnapshot, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return closedResourceSnapshotChannel(), err
	}
	if profile.resources == nil {
		return closedResourceSnapshotChannel(), nil
	}
	return profile.resources.Subscribe(ctx), nil
}

func (d *discoveryService) ReadResource(ctx context.Context, caller, uri string) (json.RawMessage, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return nil, err
	}
	if profile.resources == nil {
		return nil, domain.ErrResourceNotFound
	}
	return profile.resources.ReadResource(ctx, uri)
}

func (d *discoveryService) ReadResourceAllProfiles(ctx context.Context, uri, specKey string) (json.RawMessage, error) {
	profileNames := d.registry.activeProfileNames()
	for _, profileName := range profileNames {
		runtime := d.state.profiles[profileName]
		if runtime == nil || runtime.resources == nil {
			continue
		}
		if specKey != "" && !d.registry.profileContainsSpecKey(runtime, specKey) {
			continue
		}
		result, err := runtime.resources.ReadResource(ctx, uri)
		if err == nil {
			return result, nil
		}
		if errors.Is(err, domain.ErrResourceNotFound) {
			continue
		}
		return nil, err
	}
	return nil, domain.ErrResourceNotFound
}

func (d *discoveryService) ListPrompts(ctx context.Context, caller string, cursor string) (domain.PromptPage, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return domain.PromptPage{}, err
	}
	if profile.prompts == nil {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}
	snapshot := profile.prompts.Snapshot()
	return paginatePrompts(snapshot, cursor)
}

func (d *discoveryService) ListPromptsAllProfiles(ctx context.Context, cursor string) (domain.PromptPage, error) {
	profileNames := d.registry.activeProfileNames()
	if len(profileNames) == 0 {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}

	merged := make([]domain.PromptDefinition, 0)
	seen := make(map[string]struct{})

	for _, profileName := range profileNames {
		runtime := d.state.profiles[profileName]
		if runtime == nil || runtime.prompts == nil {
			continue
		}
		snapshot := runtime.prompts.Snapshot()
		for _, prompt := range snapshot.Prompts {
			if prompt.Name == "" {
				continue
			}
			if _, ok := seen[prompt.Name]; ok {
				continue
			}
			seen[prompt.Name] = struct{}{}
			merged = append(merged, prompt)
		}
	}

	if len(merged) == 0 {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })
	snapshot := domain.PromptSnapshot{
		ETag:    hashPrompts(merged),
		Prompts: merged,
	}
	return paginatePrompts(snapshot, cursor)
}

func (d *discoveryService) WatchPrompts(ctx context.Context, caller string) (<-chan domain.PromptSnapshot, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return closedPromptSnapshotChannel(), err
	}
	if profile.prompts == nil {
		return closedPromptSnapshotChannel(), nil
	}
	return profile.prompts.Subscribe(ctx), nil
}

func (d *discoveryService) GetPrompt(ctx context.Context, caller, name string, args json.RawMessage) (json.RawMessage, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return nil, err
	}
	if profile.prompts == nil {
		return nil, domain.ErrPromptNotFound
	}
	return profile.prompts.GetPrompt(ctx, name, args)
}

func (d *discoveryService) GetPromptAllProfiles(ctx context.Context, name string, args json.RawMessage, specKey string) (json.RawMessage, error) {
	profileNames := d.registry.activeProfileNames()
	for _, profileName := range profileNames {
		runtime := d.state.profiles[profileName]
		if runtime == nil || runtime.prompts == nil {
			continue
		}
		if specKey != "" && !d.registry.profileContainsSpecKey(runtime, specKey) {
			continue
		}
		result, err := runtime.prompts.GetPrompt(ctx, name, args)
		if err == nil {
			return result, nil
		}
		if errors.Is(err, domain.ErrPromptNotFound) {
			continue
		}
		return nil, err
	}
	return nil, domain.ErrPromptNotFound
}

func (d *discoveryService) GetToolSnapshotForCaller(caller string) (domain.ToolSnapshot, error) {
	profile, err := d.registry.resolveProfile(caller)
	if err != nil {
		return domain.ToolSnapshot{}, err
	}
	if profile.tools == nil {
		return domain.ToolSnapshot{}, nil
	}
	return profile.tools.Snapshot(), nil
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
	idx := sort.Search(len(resources), func(i int) bool {
		return resources[i].URI >= cursor
	})
	if idx >= len(resources) || resources[idx].URI != cursor {
		return -1
	}
	return idx + 1
}

func indexAfterPromptCursor(prompts []domain.PromptDefinition, cursor string) int {
	idx := sort.Search(len(prompts), func(i int) bool {
		return prompts[i].Name >= cursor
	})
	if idx >= len(prompts) || prompts[idx].Name != cursor {
		return -1
	}
	return idx + 1
}

func hashTools(tools []domain.ToolDefinition) string {
	hasher := sha256.New()
	for _, tool := range tools {
		_, _ = hasher.Write([]byte(tool.Name))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(tool.ToolJSON)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
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

func hashPrompts(prompts []domain.PromptDefinition) string {
	hasher := sha256.New()
	for _, prompt := range prompts {
		_, _ = hasher.Write([]byte(prompt.Name))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(prompt.PromptJSON)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
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
