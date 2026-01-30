package scheduler

import (
	"context"

	"mcpv/internal/domain"
)

// ApplyCatalogDiff updates scheduler state after catalog changes.
func (s *BasicScheduler) ApplyCatalogDiff(ctx context.Context, diff domain.CatalogDiff, registry map[string]domain.ServerSpec) error {
	if ctx == nil {
		ctx = context.Background()
	}
	specs := cloneSpecRegistry(registry)
	s.specsMu.Lock()
	s.specs = specs
	s.specsMu.Unlock()

	s.poolsMu.RLock()
	for specKey, state := range s.pools {
		spec, ok := specs[specKey]
		if !ok {
			continue
		}
		state.mu.Lock()
		state.spec = spec
		state.mu.Unlock()
	}
	s.poolsMu.RUnlock()

	for _, specKey := range diff.RemovedSpecKeys {
		if state := s.poolByKey(specKey); state != nil {
			if err := s.StopSpec(ctx, specKey, "catalog remove"); err != nil {
				return err
			}
			s.tryRemovePool(specKey, state)
		}
	}
	for _, specKey := range diff.RestartRequiredSpecKeys {
		if state := s.poolByKey(specKey); state != nil {
			if err := s.StopSpec(ctx, specKey, "catalog restart"); err != nil {
				return err
			}
		}
	}
	for _, specKey := range diff.UpdatedSpecKeys {
		if spec, ok := specs[specKey]; ok {
			s.getPool(specKey, spec)
		}
	}
	return nil
}

func (s *BasicScheduler) specForKey(specKey string) (domain.ServerSpec, bool) {
	s.specsMu.RLock()
	spec, ok := s.specs[specKey]
	s.specsMu.RUnlock()
	return spec, ok
}

func (s *BasicScheduler) poolByKey(specKey string) *poolState {
	s.poolsMu.RLock()
	state := s.pools[specKey]
	s.poolsMu.RUnlock()
	return state
}

func (s *BasicScheduler) removePool(specKey string) {
	s.poolsMu.Lock()
	delete(s.pools, specKey)
	s.poolsMu.Unlock()
}

func (s *BasicScheduler) tryRemovePool(specKey string, state *poolState) {
	if _, ok := s.specForKey(specKey); ok {
		return
	}

	state.mu.Lock()
	empty := len(state.instances) == 0 &&
		len(state.draining) == 0 &&
		state.starting == 0 &&
		!state.startInFlight
	state.mu.Unlock()

	if !empty {
		return
	}

	s.removePool(specKey)
}

func cloneSpecRegistry(specs map[string]domain.ServerSpec) map[string]domain.ServerSpec {
	if len(specs) == 0 {
		return map[string]domain.ServerSpec{}
	}
	out := make(map[string]domain.ServerSpec, len(specs))
	for key, spec := range specs {
		out[key] = spec
	}
	return out
}

func (s *BasicScheduler) getPool(specKey string, spec domain.ServerSpec) *poolState {
	s.poolsMu.Lock()
	defer s.poolsMu.Unlock()

	state := s.pools[specKey]
	if state == nil {
		// Use the spec as-is, preserving the original Name for display
		state = &poolState{
			spec:    spec,
			specKey: specKey,
		}
		s.pools[specKey] = state
	}
	return state
}

func (s *BasicScheduler) snapshotPools() []poolEntry {
	s.poolsMu.RLock()
	defer s.poolsMu.RUnlock()

	entries := make([]poolEntry, 0, len(s.pools))
	for specKey, state := range s.pools {
		entries = append(entries, poolEntry{specKey: specKey, state: state})
	}
	return entries
}
