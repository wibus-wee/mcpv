package runtime

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/aggregator"
	"mcpv/internal/infra/notifications"
	"mcpv/internal/infra/router"
	"mcpv/internal/infra/telemetry"
)

// State tracks runtime indexes and metadata caches.
type State struct {
	specKeys      map[string]string
	metadataCache *domain.MetadataCache
	baseRouter    *router.BasicRouter
	tools         *aggregator.ToolIndex
	resources     *aggregator.ResourceIndex
	prompts       *aggregator.PromptIndex

	mu     sync.RWMutex
	active bool
}

// NewState constructs runtime indexes for a catalog snapshot.
func NewState(
	state *domain.CatalogState,
	scheduler domain.Scheduler,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	metadataCache *domain.MetadataCache,
	listChanges *notifications.ListChangeHub,
	logger *zap.Logger,
) *State {
	if logger == nil {
		logger = zap.NewNop()
	}
	refreshGate := aggregator.NewRefreshGate()
	baseRouter := router.NewBasicRouter(scheduler, router.Options{
		Timeout: state.Summary.Runtime.RouteTimeout(),
		Logger:  logger,
	})
	rt := router.NewMetricRouter(baseRouter, metrics)
	toolIndex := aggregator.NewToolIndex(rt, state.Catalog.Specs, state.Summary.ServerSpecKeys, state.Summary.Runtime, metadataCache, logger, health, refreshGate, listChanges)
	resourceIndex := aggregator.NewResourceIndex(rt, state.Catalog.Specs, state.Summary.ServerSpecKeys, state.Summary.Runtime, metadataCache, logger, health, refreshGate, listChanges)
	promptIndex := aggregator.NewPromptIndex(rt, state.Catalog.Specs, state.Summary.ServerSpecKeys, state.Summary.Runtime, metadataCache, logger, health, refreshGate, listChanges)
	return &State{
		specKeys:      copySpecKeyMap(state.Summary.ServerSpecKeys),
		metadataCache: metadataCache,
		baseRouter:    baseRouter,
		tools:         toolIndex,
		resources:     resourceIndex,
		prompts:       promptIndex,
	}
}

// NewStateFromSpecKeys constructs a minimal runtime state for tests.
func NewStateFromSpecKeys(specKeys map[string]string) *State {
	return &State{specKeys: copySpecKeyMap(specKeys)}
}

// Activate starts indexes for the runtime state.
func (r *State) Activate(ctx context.Context) {
	r.mu.Lock()
	if r.active {
		r.mu.Unlock()
		return
	}
	r.active = true
	r.mu.Unlock()

	if r.tools != nil {
		r.tools.Start(ctx)
	}
	if r.resources != nil {
		r.resources.Start(ctx)
	}
	if r.prompts != nil {
		r.prompts.Start(ctx)
	}
}

// Deactivate stops indexes for the runtime state.
func (r *State) Deactivate() {
	r.mu.Lock()
	if !r.active {
		r.mu.Unlock()
		return
	}
	r.active = false
	r.mu.Unlock()

	if r.tools != nil {
		r.tools.Stop()
	}
	if r.resources != nil {
		r.resources.Stop()
	}
	if r.prompts != nil {
		r.prompts.Stop()
	}
}

// SpecKeys returns a copy of the server spec keys.
func (r *State) SpecKeys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.specKeys) == 0 {
		return nil
	}
	return collectSpecKeys(r.specKeys)
}

// UpdateCatalog refreshes runtime state from the catalog.
func (r *State) UpdateCatalog(catalog domain.Catalog, specKeys map[string]string, runtime domain.RuntimeConfig) {
	r.mu.Lock()
	r.specKeys = copySpecKeyMap(specKeys)
	r.mu.Unlock()

	if r.tools != nil {
		r.tools.UpdateSpecs(catalog.Specs, specKeys, runtime)
	}
	if r.resources != nil {
		r.resources.UpdateSpecs(catalog.Specs, specKeys, runtime)
	}
	if r.prompts != nil {
		r.prompts.UpdateSpecs(catalog.Specs, specKeys, runtime)
	}
}

// ApplyRuntimeConfig updates runtime-dependent settings without rebuilding indexes.
func (r *State) ApplyRuntimeConfig(_ context.Context, prev, next domain.RuntimeConfig) error {
	if r == nil {
		return nil
	}
	if r.baseRouter != nil && prev.RouteTimeoutSeconds != next.RouteTimeoutSeconds {
		r.baseRouter.SetTimeout(next.RouteTimeout())
	}
	if r.tools != nil {
		r.tools.ApplyRuntimeConfig(next)
	}
	if r.resources != nil {
		r.resources.ApplyRuntimeConfig(next)
	}
	if r.prompts != nil {
		r.prompts.ApplyRuntimeConfig(next)
	}
	return nil
}

// SetBootstrapWaiter attaches a bootstrap waiter to indexes.
func (r *State) SetBootstrapWaiter(waiter func(context.Context) error) {
	if r.tools != nil {
		r.tools.SetBootstrapWaiter(waiter)
	}
	if r.resources != nil {
		r.resources.SetBootstrapWaiter(waiter)
	}
	if r.prompts != nil {
		r.prompts.SetBootstrapWaiter(waiter)
	}
}

// Tools returns the tool index.
func (r *State) Tools() *aggregator.ToolIndex {
	return r.tools
}

// Resources returns the resource index.
func (r *State) Resources() *aggregator.ResourceIndex {
	return r.resources
}

// Prompts returns the prompt index.
func (r *State) Prompts() *aggregator.PromptIndex {
	return r.prompts
}

// MetadataCache returns the runtime metadata cache.
func (r *State) MetadataCache() *domain.MetadataCache {
	return r.metadataCache
}

func copySpecKeyMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
