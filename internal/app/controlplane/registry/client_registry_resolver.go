package registry

import (
	"time"

	"mcpv/internal/domain"
)

func (r *ClientRegistry) ResolveClientTags(client string) ([]string, error) {
	state, ok := r.loadClientState(client)
	if !ok {
		return nil, domain.ErrClientNotRegistered
	}
	return append([]string(nil), state.tags...), nil
}

func (r *ClientRegistry) ResolveClientServer(client string) (string, error) {
	state, ok := r.loadClientState(client)
	if !ok {
		return "", domain.ErrClientNotRegistered
	}
	return state.server, nil
}

func (r *ClientRegistry) ResolveVisibleSpecKeys(client string) ([]string, error) {
	state, ok := r.loadClientState(client)
	if !ok {
		return nil, domain.ErrClientNotRegistered
	}
	return append([]string(nil), state.specKeys...), nil
}

func (r *ClientRegistry) loadClientState(client string) (clientState, bool) {
	if client == "" {
		return clientState{}, false
	}
	r.mu.Lock()
	state, ok := r.activeClients[client]
	if ok {
		state.lastHeartbeat = time.Now()
		r.activeClients[client] = state
	}
	r.mu.Unlock()
	return state, ok
}
