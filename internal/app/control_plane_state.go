package app

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
)

type controlPlaneState struct {
	mu sync.RWMutex

	info             domain.ControlPlaneInfo
	runtimeState     *runtimeState
	specRegistry     map[string]domain.ServerSpec
	serverSpecKeys   map[string]string
	scheduler        domain.Scheduler
	initManager      *ServerInitializationManager
	bootstrapManager *BootstrapManager
	runtime          domain.RuntimeConfig
	catalog          domain.Catalog
	logger           *zap.Logger
	ctx              context.Context
}

func newControlPlaneState(
	ctx context.Context,
	runtime *runtimeState,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
	bootstrapManager *BootstrapManager,
	state *domain.CatalogState,
	logger *zap.Logger,
) *controlPlaneState {
	if logger == nil {
		logger = zap.NewNop()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	summary := state.Summary
	specRegistry := copySpecRegistryMap(summary.SpecRegistry)
	serverSpecKeys := copySpecKeyMap(summary.ServerSpecKeys)

	return &controlPlaneState{
		info:             defaultControlPlaneInfo(),
		runtimeState:     runtime,
		specRegistry:     specRegistry,
		serverSpecKeys:   serverSpecKeys,
		scheduler:        scheduler,
		initManager:      initManager,
		bootstrapManager: bootstrapManager,
		runtime:          summary.Runtime,
		catalog:          state.Catalog,
		logger:           logger.Named("control_plane"),
		ctx:              ctx,
	}
}

type clientState struct {
	pid           int
	tags          []string
	specKeys      []string
	lastHeartbeat time.Time
}

type runtimeState struct {
	specKeys      map[string]string
	metadataCache *domain.MetadataCache
	tools         *aggregator.ToolIndex
	resources     *aggregator.ResourceIndex
	prompts       *aggregator.PromptIndex

	mu     sync.RWMutex
	active bool
}

// Activate starts indexes for the runtime state.
func (r *runtimeState) Activate(ctx context.Context) {
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
func (r *runtimeState) Deactivate() {
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
func (r *runtimeState) SpecKeys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.specKeys) == 0 {
		return nil
	}
	return collectSpecKeys(r.specKeys)
}

// UpdateCatalog refreshes runtime state from the catalog.
func (r *runtimeState) UpdateCatalog(catalog domain.Catalog, specKeys map[string]string, runtime domain.RuntimeConfig) {
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

func defaultControlPlaneInfo() domain.ControlPlaneInfo {
	return domain.ControlPlaneInfo{
		Name:    "mcpd",
		Version: Version,
		Build:   Build,
	}
}

// UpdateCatalog replaces the control plane state with a new catalog.
func (s *controlPlaneState) UpdateCatalog(state *domain.CatalogState, runtime *runtimeState) {
	s.mu.Lock()
	s.catalog = state.Catalog
	s.runtime = state.Summary.Runtime
	s.specRegistry = copySpecRegistryMap(state.Summary.SpecRegistry)
	s.serverSpecKeys = copySpecKeyMap(state.Summary.ServerSpecKeys)
	s.runtimeState = runtime
	s.mu.Unlock()
}

// Catalog returns the current catalog.
func (s *controlPlaneState) Catalog() domain.Catalog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.catalog
}

// RuntimeState returns the runtime index state.
func (s *controlPlaneState) RuntimeState() *runtimeState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtimeState
}

// SpecRegistry returns a copy of the current spec registry.
func (s *controlPlaneState) SpecRegistry() map[string]domain.ServerSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySpecRegistryMap(s.specRegistry)
}

// ServerSpecKeys returns a copy of the current server spec keys.
func (s *controlPlaneState) ServerSpecKeys() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySpecKeyMap(s.serverSpecKeys)
}

// Runtime returns the current runtime config.
func (s *controlPlaneState) Runtime() domain.RuntimeConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtime
}

func copySpecRegistryMap(src map[string]domain.ServerSpec) map[string]domain.ServerSpec {
	if src == nil {
		return nil
	}
	dst := make(map[string]domain.ServerSpec, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
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
