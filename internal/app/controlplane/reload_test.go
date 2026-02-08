package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/app/bootstrap/serverinit"
	"mcpv/internal/app/runtime"
	"mcpv/internal/domain"
	pluginmanager "mcpv/internal/infra/plugin/manager"
)

func TestReloadManager_ApplyUpdate_UpdatesRuntimeAndRegistry(t *testing.T) {
	runtimeCfg := domain.RuntimeConfig{
		ServerInitRetryBaseSeconds: 1,
		ServerInitRetryMaxSeconds:  1,
		ServerInitMaxRetries:       1,
	}
	prevSpec := serverSpec("svc", []string{"run"}, 2)
	nextSpec := serverSpec("svc", []string{"run", "v2"}, 2)

	prevCatalog := domain.Catalog{
		Specs:   map[string]domain.ServerSpec{"svc": prevSpec},
		Runtime: runtimeCfg,
	}
	nextCatalog := domain.Catalog{
		Specs:   map[string]domain.ServerSpec{"svc": nextSpec},
		Runtime: runtimeCfg,
	}

	prevState := newCatalogState(t, prevCatalog)
	nextState := newCatalogState(t, nextCatalog)

	prevSpecKey := domain.SpecFingerprint(prevSpec)
	nextSpecKey := domain.SpecFingerprint(nextSpec)
	require.NotEqual(t, prevSpecKey, nextSpecKey)

	scheduler := &schedulerStub{}
	initManager := serverinit.NewManager(scheduler, &prevState, zap.NewNop())
	startup := bootstrap.NewServerStartupOrchestrator(initManager, nil, zap.NewNop())
	runtimeState := runtime.NewStateFromSpecKeys(prevState.Summary.ServerSpecKeys)
	state := NewState(context.Background(), runtimeState, scheduler, startup, &prevState, zap.NewNop())
	registry := NewClientRegistry(state)

	_, err := registry.RegisterClient(context.Background(), "client-1", 1, nil, "")
	require.NoError(t, err)
	scheduler.minReadyCalls = nil

	manager := NewReloadManager(nil, state, registry, scheduler, startup, nil, nil, nil, nil, nil, nil, zap.NewNop())
	update := domain.CatalogUpdate{
		Snapshot: nextState,
		Diff:     domain.DiffCatalogStates(prevState, nextState),
		Source:   domain.CatalogUpdateSourceManual,
	}

	require.NoError(t, manager.applyUpdate(context.Background(), update))

	require.Equal(t, 1, scheduler.applyCalls)
	require.Equal(t, update.Diff, scheduler.lastDiff)
	require.Equal(t, nextState.Summary.SpecRegistry, scheduler.lastRegistry)

	require.Contains(t, scheduler.minReadyCalls, reloadMinReadyCall{specKey: nextSpecKey, minReady: 2})
	visibleKeys, err := registry.ResolveVisibleSpecKeys("client-1")
	require.NoError(t, err)
	require.Contains(t, visibleKeys, nextSpecKey)
	require.NotContains(t, visibleKeys, prevSpecKey)
}

func TestReloadManager_ApplyUpdate_RemovesServer(t *testing.T) {
	prevSpec := serverSpec("default", []string{"run"}, 1)
	removedSpec := serverSpec("extra", []string{"run", "extra"}, 1)

	prevCatalog := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"default": prevSpec,
			"extra":   removedSpec,
		},
		Runtime: domain.RuntimeConfig{},
	}
	nextCatalog := domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			"default": prevSpec,
		},
		Runtime: domain.RuntimeConfig{},
	}

	prevState := newCatalogState(t, prevCatalog)
	nextState := newCatalogState(t, nextCatalog)

	removedSpecKey := domain.SpecFingerprint(removedSpec)

	scheduler := &schedulerStub{}
	runtimeState := runtime.NewStateFromSpecKeys(prevState.Summary.ServerSpecKeys)
	state := NewState(context.Background(), runtimeState, scheduler, nil, &prevState, zap.NewNop())
	registry := NewClientRegistry(state)

	_, err := registry.RegisterClient(context.Background(), "client-1", 1, nil, "")
	require.NoError(t, err)
	scheduler.stopCalls = nil

	manager := NewReloadManager(nil, state, registry, scheduler, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	update := domain.CatalogUpdate{
		Snapshot: nextState,
		Diff:     domain.DiffCatalogStates(prevState, nextState),
		Source:   domain.CatalogUpdateSourceManual,
	}

	require.NoError(t, manager.applyUpdate(context.Background(), update))
	require.Contains(t, scheduler.stopCalls, removedSpecKey)
	require.Equal(t, 1, scheduler.applyCalls)
}

func TestReloadManager_ApplyUpdate_PluginApplyFailureRetainsState(t *testing.T) {
	runtimeCfg := domain.RuntimeConfig{}
	prevSpec := serverSpec("svc", []string{"run"}, 1)

	prevCatalog := domain.Catalog{
		Specs:   map[string]domain.ServerSpec{"svc": prevSpec},
		Runtime: runtimeCfg,
	}
	brokenPlugin := domain.PluginSpec{
		Name:     "broken-plugin",
		Category: domain.PluginCategoryAuthentication,
		Required: true,
		Cmd:      []string{"/does-not-exist"},
	}
	nextCatalog := domain.Catalog{
		Specs:   prevCatalog.Specs,
		Plugins: []domain.PluginSpec{brokenPlugin},
		Runtime: runtimeCfg,
	}

	prevState := newCatalogState(t, prevCatalog)
	nextState := newCatalogState(t, nextCatalog)

	scheduler := &schedulerStub{}
	state := NewState(context.Background(), runtime.NewStateFromSpecKeys(prevState.Summary.ServerSpecKeys), scheduler, nil, &prevState, zap.NewNop())
	registry := NewClientRegistry(state)

	pluginManager, err := pluginmanager.NewManager(pluginmanager.Options{Logger: zap.NewNop(), RootDir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { pluginManager.Stop(context.Background()) })

	manager := NewReloadManager(nil, state, registry, scheduler, nil, pluginManager, nil, nil, nil, nil, nil, zap.NewNop())
	update := domain.CatalogUpdate{
		Snapshot: nextState,
		Diff:     domain.DiffCatalogStates(prevState, nextState),
		Source:   domain.CatalogUpdateSourceManual,
	}

	err = manager.applyUpdate(context.Background(), update)
	require.Error(t, err)
	require.Equal(t, prevCatalog, manager.state.Catalog())
}

func TestReloadManager_ApplyUpdate_SchedulerErrorDoesNotAdvanceState(t *testing.T) {
	prevSpec := serverSpec("svc", []string{"run"}, 1)
	nextSpec := serverSpec("svc", []string{"run", "v2"}, 1)

	prevCatalog := domain.Catalog{
		Specs:   map[string]domain.ServerSpec{"svc": prevSpec},
		Runtime: domain.RuntimeConfig{},
	}
	nextCatalog := domain.Catalog{
		Specs:   map[string]domain.ServerSpec{"svc": nextSpec},
		Runtime: domain.RuntimeConfig{},
	}

	prevState := newCatalogState(t, prevCatalog)
	nextState := newCatalogState(t, nextCatalog)

	scheduler := &schedulerStub{applyErr: errors.New("apply failed")}
	runtimeState := runtime.NewStateFromSpecKeys(prevState.Summary.ServerSpecKeys)
	state := NewState(context.Background(), runtimeState, scheduler, nil, &prevState, zap.NewNop())
	registry := NewClientRegistry(state)

	manager := NewReloadManager(nil, state, registry, scheduler, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	update := domain.CatalogUpdate{
		Snapshot: nextState,
		Diff:     domain.DiffCatalogStates(prevState, nextState),
		Source:   domain.CatalogUpdateSourceManual,
	}

	err := manager.applyUpdate(context.Background(), update)
	require.Error(t, err)
	require.Equal(t, prevCatalog, state.Catalog())
}

func TestReloadManager_ApplyUpdate_RegistryErrorRollsBackState(t *testing.T) {
	prevSpec := serverSpec("svc", []string{"run"}, 1)
	nextSpec := serverSpec("svc", []string{"run", "v2"}, 1)

	prevCatalog := domain.Catalog{
		Specs:   map[string]domain.ServerSpec{"svc": prevSpec},
		Runtime: domain.RuntimeConfig{},
	}
	nextCatalog := domain.Catalog{
		Specs:   map[string]domain.ServerSpec{"svc": nextSpec},
		Runtime: domain.RuntimeConfig{},
	}

	prevState := newCatalogState(t, prevCatalog)
	nextState := newCatalogState(t, nextCatalog)

	scheduler := &schedulerStub{}
	runtimeState := runtime.NewStateFromSpecKeys(prevState.Summary.ServerSpecKeys)
	state := NewState(context.Background(), runtimeState, scheduler, nil, &prevState, zap.NewNop())
	registry := NewClientRegistry(state)

	_, err := registry.RegisterClient(context.Background(), "client-1", 1, nil, "")
	require.NoError(t, err)
	scheduler.setMinReadyErr = errors.New("min ready failed")

	manager := NewReloadManager(nil, state, registry, scheduler, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	update := domain.CatalogUpdate{
		Snapshot: nextState,
		Diff:     domain.DiffCatalogStates(prevState, nextState),
		Source:   domain.CatalogUpdateSourceManual,
	}

	err = manager.applyUpdate(context.Background(), update)
	require.Error(t, err)
	require.Equal(t, prevCatalog, state.Catalog())
	require.Equal(t, 2, scheduler.applyCalls)
}

func TestReloadManager_HandleApplyError_StrictPanics(t *testing.T) {
	runtimeCfg := domain.RuntimeConfig{ReloadMode: domain.ReloadModeStrict}
	catalog := domain.Catalog{
		Specs:   map[string]domain.ServerSpec{},
		Runtime: runtimeCfg,
	}
	state := newCatalogState(t, catalog)

	update := domain.CatalogUpdate{
		Snapshot: state,
		Diff:     domain.CatalogDiff{},
		Source:   domain.CatalogUpdateSourceManual,
	}

	logger := zap.New(zapcore.NewNopCore(), zap.WithFatalHook(zapcore.WriteThenPanic))
	manager := NewReloadManager(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	manager.coreLogger = logger
	manager.observer.SetCoreLogger(logger)

	require.Panics(t, func() {
		manager.observer.HandleApplyError(update, errors.New("apply failed"), 10*time.Millisecond)
	})
}

func TestReloadManager_HandleApplyError_LenientDoesNotPanic(t *testing.T) {
	runtimeCfg := domain.RuntimeConfig{ReloadMode: domain.ReloadModeLenient}
	catalog := domain.Catalog{
		Specs:   map[string]domain.ServerSpec{},
		Runtime: runtimeCfg,
	}
	state := newCatalogState(t, catalog)

	update := domain.CatalogUpdate{
		Snapshot: state,
		Diff:     domain.CatalogDiff{},
		Source:   domain.CatalogUpdateSourceManual,
	}

	logger := zap.New(zapcore.NewNopCore(), zap.WithFatalHook(zapcore.WriteThenPanic))
	manager := NewReloadManager(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	manager.coreLogger = logger
	manager.observer.SetCoreLogger(logger)

	require.NotPanics(t, func() {
		manager.observer.HandleApplyError(update, errors.New("apply failed"), 10*time.Millisecond)
	})
}

type reloadMinReadyCall struct {
	specKey  string
	minReady int
}

type schedulerStub struct {
	applyCalls     int
	lastDiff       domain.CatalogDiff
	lastRegistry   map[string]domain.ServerSpec
	minReadyCalls  []reloadMinReadyCall
	stopCalls      []string
	applyErr       error
	setMinReadyErr error
}

func (s *schedulerStub) Acquire(_ context.Context, _, _ string) (*domain.Instance, error) {
	return nil, nil
}

func (s *schedulerStub) AcquireReady(_ context.Context, _, _ string) (*domain.Instance, error) {
	return nil, nil
}

func (s *schedulerStub) Release(_ context.Context, _ *domain.Instance) error {
	return nil
}

func (s *schedulerStub) SetDesiredMinReady(_ context.Context, specKey string, minReady int) error {
	if s.setMinReadyErr != nil {
		return s.setMinReadyErr
	}
	s.minReadyCalls = append(s.minReadyCalls, reloadMinReadyCall{specKey: specKey, minReady: minReady})
	return nil
}

func (s *schedulerStub) StopSpec(_ context.Context, specKey, _ string) error {
	s.stopCalls = append(s.stopCalls, specKey)
	return nil
}

func (s *schedulerStub) ApplyCatalogDiff(_ context.Context, diff domain.CatalogDiff, registry map[string]domain.ServerSpec) error {
	s.applyCalls++
	s.lastDiff = diff
	s.lastRegistry = copySpecRegistry(registry)
	if s.applyErr != nil {
		return s.applyErr
	}
	return nil
}

func (s *schedulerStub) StartIdleManager(_ time.Duration) {}

func (s *schedulerStub) StopIdleManager() {}

func (s *schedulerStub) StartPingManager(_ time.Duration) {}

func (s *schedulerStub) StopPingManager() {}

func (s *schedulerStub) StopAll(_ context.Context) {}

func (s *schedulerStub) GetPoolStatus(_ context.Context) ([]domain.PoolInfo, error) {
	return nil, nil
}

func copySpecRegistry(registry map[string]domain.ServerSpec) map[string]domain.ServerSpec {
	if len(registry) == 0 {
		return map[string]domain.ServerSpec{}
	}
	clone := make(map[string]domain.ServerSpec, len(registry))
	for key, spec := range registry {
		clone[key] = spec
	}
	return clone
}

func newCatalogState(t *testing.T, catalog domain.Catalog) domain.CatalogState {
	t.Helper()

	state, err := domain.NewCatalogState(catalog, 0, time.Time{})
	require.NoError(t, err)
	return state
}

func serverSpec(name string, cmd []string, minReady int) domain.ServerSpec {
	return domain.ServerSpec{
		Name:            name,
		Cmd:             cmd,
		MaxConcurrent:   1,
		MinReady:        minReady,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
}
