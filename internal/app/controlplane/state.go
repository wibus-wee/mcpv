package controlplane

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/app/runtime"
	"mcpv/internal/domain"
)

type State struct {
	mu sync.RWMutex

	info             domain.ControlPlaneInfo
	runtimeState     *runtime.State
	specRegistry     map[string]domain.ServerSpec
	serverSpecKeys   map[string]string
	scheduler        domain.Scheduler
	initManager      *bootstrap.ServerInitializationManager
	bootstrapManager *bootstrap.Manager
	runtime          domain.RuntimeConfig
	catalog          domain.Catalog
	logger           *zap.Logger
	ctx              context.Context
}

func NewState(
	ctx context.Context,
	runtimeState *runtime.State,
	scheduler domain.Scheduler,
	initManager *bootstrap.ServerInitializationManager,
	bootstrapManager *bootstrap.Manager,
	state *domain.CatalogState,
	logger *zap.Logger,
) *State {
	if logger == nil {
		logger = zap.NewNop()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	summary := state.Summary
	specRegistry := copySpecRegistryMap(summary.SpecRegistry)
	serverSpecKeys := copySpecKeyMap(summary.ServerSpecKeys)

	return &State{
		info:             defaultControlPlaneInfo(),
		runtimeState:     runtimeState,
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
	server        string
	specKeys      []string
	lastHeartbeat time.Time
}

func defaultControlPlaneInfo() domain.ControlPlaneInfo {
	return domain.ControlPlaneInfo{
		Name:    "mcpv",
		Version: Version,
		Build:   Build,
	}
}

// UpdateCatalog replaces the control plane state with a new catalog.
func (s *State) UpdateCatalog(state *domain.CatalogState, runtimeState *runtime.State) {
	s.mu.Lock()
	s.catalog = state.Catalog
	s.runtime = state.Summary.Runtime
	s.specRegistry = copySpecRegistryMap(state.Summary.SpecRegistry)
	s.serverSpecKeys = copySpecKeyMap(state.Summary.ServerSpecKeys)
	s.runtimeState = runtimeState
	s.mu.Unlock()
}

// Catalog returns the current catalog.
func (s *State) Catalog() domain.Catalog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.catalog
}

// RuntimeState returns the runtime index state.
func (s *State) RuntimeState() *runtime.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtimeState
}

// SpecRegistry returns a copy of the current spec registry.
func (s *State) SpecRegistry() map[string]domain.ServerSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySpecRegistryMap(s.specRegistry)
}

// ServerSpecKeys returns a copy of the current server spec keys.
func (s *State) ServerSpecKeys() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySpecKeyMap(s.serverSpecKeys)
}

// Runtime returns the current runtime config.
func (s *State) Runtime() domain.RuntimeConfig {
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
