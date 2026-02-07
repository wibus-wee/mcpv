package registry

import (
	"sort"
	"time"

	"mcpv/internal/domain"
)

func (r *ClientRegistry) snapshotActiveClientsLocked(now time.Time) domain.ActiveClientSnapshot {
	clients := make([]domain.ActiveClient, 0, len(r.activeClients))
	for client, state := range r.activeClients {
		if isInternalClientName(client) {
			continue
		}
		clients = append(clients, domain.ActiveClient{
			Client:        client,
			PID:           state.pid,
			Tags:          append([]string(nil), state.tags...),
			Server:        state.server,
			LastHeartbeat: state.lastHeartbeat,
		})
	}

	return domain.ActiveClientSnapshot{
		Clients:     clients,
		GeneratedAt: now,
	}
}

func finalizeActiveClientSnapshot(snapshot domain.ActiveClientSnapshot) domain.ActiveClientSnapshot {
	if len(snapshot.Clients) == 0 {
		return snapshot
	}
	clients := append([]domain.ActiveClient(nil), snapshot.Clients...)
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].Client < clients[j].Client
	})
	snapshot.Clients = clients
	return snapshot
}

func (r *ClientRegistry) broadcastActiveClients(snapshot domain.ActiveClientSnapshot) {
	subs := r.copyActiveClientSubscribers()
	for _, ch := range subs {
		sendActiveClientSnapshot(ch, snapshot)
	}
}

func (r *ClientRegistry) copyActiveClientSubscribers() []chan domain.ActiveClientSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	subs := make([]chan domain.ActiveClientSnapshot, 0, len(r.activeClientSubs))
	for ch := range r.activeClientSubs {
		subs = append(subs, ch)
	}
	return subs
}

func sendActiveClientSnapshot(ch chan domain.ActiveClientSnapshot, snapshot domain.ActiveClientSnapshot) {
	select {
	case ch <- snapshot:
	default:
	}
}

func (r *ClientRegistry) broadcastClientChange(event ClientChangeEvent) {
	r.mu.Lock()
	subs := make([]chan ClientChangeEvent, 0, len(r.clientChangeSubs))
	for ch := range r.clientChangeSubs {
		subs = append(subs, ch)
	}
	r.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}
