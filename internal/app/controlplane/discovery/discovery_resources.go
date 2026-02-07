package discovery

import (
	"context"
	"encoding/json"
	"sort"

	"mcpv/internal/app/controlplane/registry"
	"mcpv/internal/domain"
	"mcpv/internal/infra/hashutil"
)

type ResourceDiscoveryService struct {
	discoverySupport
}

func NewResourceDiscoveryService(state State, registry *registry.ClientRegistry) *ResourceDiscoveryService {
	return &ResourceDiscoveryService{discoverySupport: newDiscoverySupport(state, registry)}
}

// ListResources lists resources visible to a client.
func (d *ResourceDiscoveryService) ListResources(_ context.Context, client string, cursor string) (domain.ResourcePage, error) {
	serverName, err := d.resolveClientServer(client)
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
	visibleSpecKeys, err := d.resolveVisibleSpecKeys(client)
	if err != nil {
		return domain.ResourcePage{}, err
	}
	snapshot := runtime.Resources().Snapshot()
	filtered := d.filterResourceSnapshot(snapshot, visibleSpecKeys)
	return paginateResources(filtered, cursor)
}

// ListResourcesAll lists resources across all servers.
func (d *ResourceDiscoveryService) ListResourcesAll(_ context.Context, cursor string) (domain.ResourcePage, error) {
	runtime := d.state.RuntimeState()
	if runtime == nil || runtime.Resources() == nil {
		return domain.ResourcePage{Snapshot: domain.ResourceSnapshot{}}, nil
	}
	snapshot := runtime.Resources().Snapshot()
	return paginateResources(snapshot, cursor)
}

// WatchResources streams resource snapshots for a client.
func (d *ResourceDiscoveryService) WatchResources(ctx context.Context, client string) (<-chan domain.ResourceSnapshot, error) {
	if _, err := d.resolveClientServer(client); err != nil {
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
func (d *ResourceDiscoveryService) ReadResource(ctx context.Context, client, uri string) (json.RawMessage, error) {
	serverName, err := d.resolveClientServer(client)
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
	visibleSpecKeys, err := d.resolveVisibleSpecKeys(client)
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
func (d *ResourceDiscoveryService) ReadResourceAll(ctx context.Context, uri string) (json.RawMessage, error) {
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

func (d *ResourceDiscoveryService) sendFilteredResources(ch chan<- domain.ResourceSnapshot, client string, snapshot domain.ResourceSnapshot) {
	serverName, err := d.resolveClientServer(client)
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
	visibleSpecKeys, err := d.resolveVisibleSpecKeys(client)
	if err != nil {
		return
	}
	filtered := d.filterResourceSnapshot(snapshot, visibleSpecKeys)
	select {
	case ch <- filtered:
	default:
	}
}

func (d *ResourceDiscoveryService) filterResourceSnapshot(snapshot domain.ResourceSnapshot, visibleSpecKeys []string) domain.ResourceSnapshot {
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
		ETag:      hashutil.ResourceETag(d.state.Logger(), filtered),
		Resources: filtered,
	}
}

func closedResourceSnapshotChannel() chan domain.ResourceSnapshot {
	ch := make(chan domain.ResourceSnapshot)
	close(ch)
	return ch
}
