package registry

import (
	"context"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
)

// ApplyCatalogUpdate updates client state based on catalog changes.
func (r *ClientRegistry) ApplyCatalogUpdate(ctx context.Context, update domain.CatalogUpdate) error {
	now := time.Now()

	r.mu.Lock()
	oldSpecCounts := copyCounts(r.specCounts)
	newSpecCounts := make(map[string]int)
	changedClients := make([]string, 0)

	for client, state := range r.activeClients {
		nextSpecKeys, _ := r.resolver.VisibleSpecKeysForCatalog(update.Snapshot.Catalog, update.Snapshot.Summary.ServerSpecKeys, state.tags, state.server)
		if !sameKeySet(state.specKeys, nextSpecKeys) {
			changedClients = append(changedClients, client)
			state.specKeys = nextSpecKeys
			r.activeClients[client] = state
		}
		if !isInternalClientName(client) {
			for _, specKey := range nextSpecKeys {
				newSpecCounts[specKey]++
			}
		}
	}
	r.specCounts = newSpecCounts
	snapshot := r.snapshotActiveClientsLocked(now)
	r.mu.Unlock()

	specsToStart, specsToStop := diffCounts(oldSpecCounts, newSpecCounts)
	specsToStart, specsToStop = filterOverlap(specsToStart, specsToStop)

	if err := r.activateSpecs(ctx, specsToStart, ""); err != nil {
		return err
	}
	if err := r.deactivateSpecs(ctx, specsToStop); err != nil {
		r.state.Logger().Warn("spec deactivation failed", zap.Error(err))
	}
	r.broadcastActiveClients(finalizeActiveClientSnapshot(snapshot))
	for _, client := range changedClients {
		r.broadcastClientChange(ClientChangeEvent{Client: client})
	}
	return nil
}
