package discovery

import (
	"mcpv/internal/app/controlplane/registry"
	"mcpv/internal/domain"
)

type discoverySupport struct {
	state    State
	registry *registry.ClientRegistry
}

func newDiscoverySupport(state State, registry *registry.ClientRegistry) discoverySupport {
	return discoverySupport{state: state, registry: registry}
}

func (d discoverySupport) resolveClientServer(client string) (string, error) {
	return d.registry.ResolveClientServer(client)
}

func (d discoverySupport) resolveVisibleSpecKeys(client string) ([]string, error) {
	return d.registry.ResolveVisibleSpecKeys(client)
}

func (d discoverySupport) visibleServers(visibleSpecKeys []string) (map[string]struct{}, map[string]struct{}) {
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

func (d discoverySupport) isServerVisible(visibleSpecKeys map[string]struct{}, serverName string) bool {
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

func (d discoverySupport) metadataCache() *domain.MetadataCache {
	runtime := d.state.RuntimeState()
	if runtime == nil {
		return nil
	}
	return runtime.MetadataCache()
}
