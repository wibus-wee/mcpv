package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

func TestReloadManager_ApplyUpdate_UpdatesRuntimeAndRegistry(t *testing.T) {
	runtime := domain.RuntimeConfig{
		ServerInitRetryBaseSeconds: 1,
		ServerInitRetryMaxSeconds:  1,
		ServerInitMaxRetries:       1,
	}
	prevSpec := serverSpec("svc", []string{"run"}, 2)
	nextSpec := serverSpec("svc", []string{"run", "v2"}, 2)

	prevState := newCatalogState(t, map[string]domain.Profile{
		domain.DefaultProfileName: profileWithSpec(domain.DefaultProfileName, "svc", prevSpec, runtime),
	}, nil)
	nextState := newCatalogState(t, map[string]domain.Profile{
		domain.DefaultProfileName: profileWithSpec(domain.DefaultProfileName, "svc", nextSpec, runtime),
	}, nil)

	prevRuntime := &profileRuntime{
		name:     domain.DefaultProfileName,
		specKeys: collectSpecKeys(prevState.Summary.Profiles[domain.DefaultProfileName].SpecKeys),
		active:   true,
	}
	profiles := map[string]*profileRuntime{
		domain.DefaultProfileName: prevRuntime,
	}

	scheduler := &schedulerStub{}
	initManager := NewServerInitializationManager(scheduler, &prevState, zap.NewNop())
	state := newControlPlaneState(context.Background(), profiles, scheduler, initManager, &prevState, zap.NewNop())
	registry := newCallerRegistry(state)
	registry.activeCallers["caller-1"] = callerState{
		pid:           1,
		profile:       domain.DefaultProfileName,
		lastHeartbeat: time.Now(),
	}

	manager := NewReloadManager(nil, state, registry, scheduler, initManager, nil, nil, nil, zap.NewNop())
	update := domain.CatalogUpdate{
		Snapshot: nextState,
		Diff:     domain.DiffCatalogStates(prevState, nextState),
		Source:   domain.CatalogUpdateSourceManual,
	}

	require.NoError(t, manager.applyUpdate(context.Background(), update))

	require.Equal(t, 1, scheduler.applyCalls)
	require.Equal(t, update.Diff, scheduler.lastDiff)
	require.Equal(t, nextState.Summary.SpecRegistry, scheduler.lastRegistry)

	updatedRuntime, ok := state.Profile(domain.DefaultProfileName)
	require.True(t, ok)
	require.Same(t, prevRuntime, updatedRuntime)
	require.Equal(t, collectSpecKeys(nextState.Summary.Profiles[domain.DefaultProfileName].SpecKeys), updatedRuntime.SpecKeys())

	require.Equal(t, 1, registry.profileCounts[domain.DefaultProfileName])
	specKey := collectSpecKeys(nextState.Summary.Profiles[domain.DefaultProfileName].SpecKeys)[0]
	require.Equal(t, 1, registry.specCounts[specKey])
	require.Len(t, scheduler.minReadyCalls, 1)
	require.Equal(t, reloadMinReadyCall{specKey: specKey, minReady: 2}, scheduler.minReadyCalls[0])
}

func TestReloadManager_ApplyUpdate_RemovesProfile(t *testing.T) {
	runtime := domain.RuntimeConfig{}
	defaultSpec := serverSpec("default", []string{"run"}, 1)
	extraSpec := serverSpec("extra", []string{"run", "extra"}, 1)

	prevState := newCatalogState(t, map[string]domain.Profile{
		domain.DefaultProfileName: profileWithSpec(domain.DefaultProfileName, "default", defaultSpec, runtime),
		"extra":                   profileWithSpec("extra", "extra", extraSpec, runtime),
	}, nil)
	nextState := newCatalogState(t, map[string]domain.Profile{
		domain.DefaultProfileName: profileWithSpec(domain.DefaultProfileName, "default", defaultSpec, runtime),
	}, nil)

	removedRuntime := &profileRuntime{
		name:     "extra",
		specKeys: collectSpecKeys(prevState.Summary.Profiles["extra"].SpecKeys),
		active:   true,
	}
	profiles := map[string]*profileRuntime{
		domain.DefaultProfileName: &profileRuntime{
			name:     domain.DefaultProfileName,
			specKeys: collectSpecKeys(prevState.Summary.Profiles[domain.DefaultProfileName].SpecKeys),
			active:   true,
		},
		"extra": removedRuntime,
	}

	scheduler := &schedulerStub{}
	state := newControlPlaneState(context.Background(), profiles, scheduler, nil, &prevState, zap.NewNop())
	registry := newCallerRegistry(state)
	manager := NewReloadManager(nil, state, registry, scheduler, nil, nil, nil, nil, zap.NewNop())
	update := domain.CatalogUpdate{
		Snapshot: nextState,
		Diff:     domain.DiffCatalogStates(prevState, nextState),
		Source:   domain.CatalogUpdateSourceManual,
	}

	require.NoError(t, manager.applyUpdate(context.Background(), update))

	_, ok := state.Profile("extra")
	require.False(t, ok)
	require.False(t, removedRuntime.active)
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

func (s *schedulerStub) Acquire(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	return nil, nil
}

func (s *schedulerStub) AcquireReady(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	return nil, nil
}

func (s *schedulerStub) Release(ctx context.Context, instance *domain.Instance) error {
	return nil
}

func (s *schedulerStub) SetDesiredMinReady(ctx context.Context, specKey string, minReady int) error {
	s.minReadyCalls = append(s.minReadyCalls, reloadMinReadyCall{specKey: specKey, minReady: minReady})
	return nil
}

func (s *schedulerStub) StopSpec(ctx context.Context, specKey, reason string) error {
	s.stopCalls = append(s.stopCalls, specKey)
	return nil
}

func (s *schedulerStub) ApplyCatalogDiff(ctx context.Context, diff domain.CatalogDiff, registry map[string]domain.ServerSpec) error {
	s.applyCalls++
	s.lastDiff = diff
	s.lastRegistry = copySpecRegistry(registry)
	return nil
}

func (s *schedulerStub) StartIdleManager(interval time.Duration) {}

func (s *schedulerStub) StopIdleManager() {}

func (s *schedulerStub) StartPingManager(interval time.Duration) {}

func (s *schedulerStub) StopPingManager() {}

func (s *schedulerStub) StopAll(ctx context.Context) {}

func (s *schedulerStub) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
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

func newCatalogState(t *testing.T, profiles map[string]domain.Profile, callers map[string]string) domain.CatalogState {
	t.Helper()

	store := domain.ProfileStore{
		Profiles: profiles,
		Callers:  callers,
	}
	state, err := domain.NewCatalogState(store, 0, time.Time{})
	require.NoError(t, err)
	return state
}

func profileWithSpec(profileName string, serverType string, spec domain.ServerSpec, runtime domain.RuntimeConfig) domain.Profile {
	return domain.Profile{
		Name: profileName,
		Catalog: domain.Catalog{
			Specs: map[string]domain.ServerSpec{
				serverType: spec,
			},
			Runtime: runtime,
		},
	}
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
