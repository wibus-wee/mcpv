package controlplane

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/app/runtime"
	"mcpv/internal/domain"
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
	initManager := bootstrap.NewServerInitializationManager(scheduler, &prevState, zap.NewNop())
	runtimeState := runtime.NewStateFromSpecKeys(prevState.Summary.ServerSpecKeys)
	state := NewState(context.Background(), runtimeState, scheduler, initManager, nil, &prevState, zap.NewNop())
	registry := NewClientRegistry(state)

	_, err := registry.RegisterClient(context.Background(), "client-1", 1, nil, "")
	require.NoError(t, err)
	scheduler.minReadyCalls = nil

	manager := NewReloadManager(nil, state, registry, scheduler, initManager, nil, nil, nil, nil, nil, nil, zap.NewNop())
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
	require.Equal(t, 1, registry.specCounts[nextSpecKey])
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
	state := NewState(context.Background(), runtimeState, scheduler, nil, nil, &prevState, zap.NewNop())
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

type reloadMinReadyCall struct {
	specKey  string
	minReady int
}

type schedulerStub struct {
	applyCalls    int
	lastDiff      domain.CatalogDiff
	lastRegistry  map[string]domain.ServerSpec
	minReadyCalls []reloadMinReadyCall
	stopCalls     []string
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
