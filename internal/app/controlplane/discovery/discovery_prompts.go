package discovery

import (
	"context"
	"encoding/json"
	"sort"

	"mcpv/internal/app/controlplane/registry"
	"mcpv/internal/domain"
	"mcpv/internal/infra/hashutil"
)

type PromptDiscoveryService struct {
	discoverySupport
}

func NewPromptDiscoveryService(state State, registry *registry.ClientRegistry) *PromptDiscoveryService {
	return &PromptDiscoveryService{discoverySupport: newDiscoverySupport(state, registry)}
}

// ListPrompts lists prompts visible to a client.
func (d *PromptDiscoveryService) ListPrompts(_ context.Context, client string, cursor string) (domain.PromptPage, error) {
	serverName, err := d.resolveClientServer(client)
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
	visibleSpecKeys, err := d.resolveVisibleSpecKeys(client)
	if err != nil {
		return domain.PromptPage{}, err
	}
	snapshot := runtime.Prompts().Snapshot()
	filtered := d.filterPromptSnapshot(snapshot, visibleSpecKeys)
	return paginatePrompts(filtered, cursor)
}

// ListPromptsAll lists prompts across all servers.
func (d *PromptDiscoveryService) ListPromptsAll(_ context.Context, cursor string) (domain.PromptPage, error) {
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Prompts() == nil {
		return domain.PromptPage{Snapshot: domain.PromptSnapshot{}}, nil
	}
	snapshot := runtime.Prompts().Snapshot()
	return paginatePrompts(snapshot, cursor)
}

// WatchPrompts streams prompt snapshots for a client.
func (d *PromptDiscoveryService) WatchPrompts(ctx context.Context, client string) (<-chan domain.PromptSnapshot, error) {
	if _, err := d.resolveClientServer(client); err != nil {
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
func (d *PromptDiscoveryService) GetPrompt(ctx context.Context, client, name string, args json.RawMessage) (json.RawMessage, error) {
	serverName, err := d.resolveClientServer(client)
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
	visibleSpecKeys, err := d.resolveVisibleSpecKeys(client)
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
func (d *PromptDiscoveryService) GetPromptAll(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
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

func (d *PromptDiscoveryService) sendFilteredPrompts(ch chan<- domain.PromptSnapshot, client string, snapshot domain.PromptSnapshot) {
	serverName, err := d.resolveClientServer(client)
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
	visibleSpecKeys, err := d.resolveVisibleSpecKeys(client)
	if err != nil {
		return
	}
	filtered := d.filterPromptSnapshot(snapshot, visibleSpecKeys)
	select {
	case ch <- filtered:
	default:
	}
}

func (d *PromptDiscoveryService) filterPromptSnapshot(snapshot domain.PromptSnapshot, visibleSpecKeys []string) domain.PromptSnapshot {
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
		ETag:    hashutil.PromptETag(d.state.Logger(), filtered),
		Prompts: filtered,
	}
}

func closedPromptSnapshotChannel() chan domain.PromptSnapshot {
	ch := make(chan domain.PromptSnapshot)
	close(ch)
	return ch
}
