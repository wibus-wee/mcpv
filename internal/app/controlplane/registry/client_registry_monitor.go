package registry

import (
	"context"
	"time"

	"go.uber.org/zap"
)

func (r *ClientRegistry) reapDeadClients(ctx context.Context) {
	now := time.Now()
	runtime := r.state.Runtime()
	timeout := runtime.ClientCheckInterval() * clientReapTimeoutMultiplier
	inactiveTimeout := runtime.ClientInactiveInterval()
	r.mu.Lock()
	clients := make([]string, 0, len(r.activeClients))
	for client, state := range r.activeClients {
		if inactiveTimeout > 0 && !state.lastHeartbeat.IsZero() && now.Sub(state.lastHeartbeat) > inactiveTimeout {
			clients = append(clients, client)
			continue
		}
		if timeout > 0 && !state.lastHeartbeat.IsZero() && now.Sub(state.lastHeartbeat) <= timeout {
			continue
		}
		if !r.probe.Alive(state.pid) {
			clients = append(clients, client)
		}
	}
	r.mu.Unlock()

	for _, client := range clients {
		if err := r.UnregisterClient(ctx, client); err != nil {
			r.state.Logger().Warn("client reap failed", zap.String("client", client), zap.Error(err))
		}
	}
}
