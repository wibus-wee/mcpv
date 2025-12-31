package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

func TestControlPlane_RequiresRegistration(t *testing.T) {
	cp := newTestControlPlane(
		context.Background(),
		map[string]*profileRuntime{
			domain.DefaultProfileName: {name: domain.DefaultProfileName},
		},
		map[string]string{},
		map[string]domain.ServerSpec{},
		&fakeScheduler{},
		domain.RuntimeConfig{},
	)

	_, err := cp.ListTools(context.Background(), "caller")
	require.ErrorIs(t, err, domain.ErrCallerNotRegistered)
}

func TestControlPlane_RegisterUnregister(t *testing.T) {
	specKey := "spec-a"
	spec := domain.ServerSpec{
		Name:            specKey,
		Cmd:             []string{"/bin/true"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	runtime := &profileRuntime{
		name:     domain.DefaultProfileName,
		specKeys: []string{specKey},
	}
	sched := &fakeScheduler{}
	cp := newTestControlPlane(
		context.Background(),
		map[string]*profileRuntime{domain.DefaultProfileName: runtime},
		map[string]string{"caller": domain.DefaultProfileName},
		map[string]domain.ServerSpec{specKey: spec},
		sched,
		domain.RuntimeConfig{},
	)

	profile, err := cp.RegisterCaller(context.Background(), "caller", 1234)
	require.NoError(t, err)
	require.Equal(t, domain.DefaultProfileName, profile)
	require.True(t, runtime.active)
	require.Equal(t, []minReadyCall{{specKey: specKey, minReady: 1}}, sched.minReadyCalls)

	require.NoError(t, cp.UnregisterCaller(context.Background(), "caller"))
	require.False(t, runtime.active)
	require.Equal(t, []stopCall{{specKey: specKey, reason: "caller inactive"}}, sched.stopCalls)
}

func TestControlPlane_ReapDeadCallers_Heartbeat(t *testing.T) {
	runtime := domain.RuntimeConfig{CallerCheckSeconds: 1, CallerInactiveSeconds: 60}
	cp := newTestControlPlane(
		context.Background(),
		map[string]*profileRuntime{
			domain.DefaultProfileName: {name: domain.DefaultProfileName},
		},
		map[string]string{},
		map[string]domain.ServerSpec{},
		&fakeScheduler{},
		runtime,
	)

	cp.registry.mu.Lock()
	cp.registry.activeCallers["caller"] = callerState{
		pid:           -1,
		profile:       domain.DefaultProfileName,
		lastHeartbeat: time.Now(),
	}
	cp.registry.profileCounts[domain.DefaultProfileName] = 1
	cp.registry.mu.Unlock()

	cp.registry.reapDeadCallers(context.Background())
	_, err := cp.registry.resolveProfile("caller")
	require.NoError(t, err)

	cp.registry.mu.Lock()
	cp.registry.activeCallers["caller"] = callerState{
		pid:           -1,
		profile:       domain.DefaultProfileName,
		lastHeartbeat: time.Now().Add(-time.Minute),
	}
	cp.registry.profileCounts[domain.DefaultProfileName] = 1
	cp.registry.mu.Unlock()

	cp.registry.reapDeadCallers(context.Background())
	_, err = cp.registry.resolveProfile("caller")
	require.ErrorIs(t, err, domain.ErrCallerNotRegistered)
}

func TestControlPlane_ReapDeadCallers_TTL(t *testing.T) {
	runtime := domain.RuntimeConfig{CallerCheckSeconds: 1, CallerInactiveSeconds: 1}
	cp := newTestControlPlane(
		context.Background(),
		map[string]*profileRuntime{
			domain.DefaultProfileName: {name: domain.DefaultProfileName},
		},
		map[string]string{},
		map[string]domain.ServerSpec{},
		&fakeScheduler{},
		runtime,
	)

	cp.registry.mu.Lock()
	cp.registry.activeCallers["caller"] = callerState{
		pid:           os.Getpid(),
		profile:       domain.DefaultProfileName,
		lastHeartbeat: time.Now().Add(-2 * time.Second),
	}
	cp.registry.profileCounts[domain.DefaultProfileName] = 1
	cp.registry.mu.Unlock()

	cp.registry.reapDeadCallers(context.Background())
	_, err := cp.registry.resolveProfile("caller")
	require.ErrorIs(t, err, domain.ErrCallerNotRegistered)
}

func TestPaginateResources_InvalidCursor(t *testing.T) {
	snapshot := domain.ResourceSnapshot{
		ETag: "v1",
		Resources: []domain.ResourceDefinition{
			{URI: "a"},
			{URI: "b"},
		},
	}
	_, err := paginateResources(snapshot, "missing")
	require.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestPaginatePrompts_InvalidCursor(t *testing.T) {
	snapshot := domain.PromptSnapshot{
		ETag: "v1",
		Prompts: []domain.PromptDefinition{
			{Name: "a"},
			{Name: "b"},
		},
	}
	_, err := paginatePrompts(snapshot, "missing")
	require.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func newTestControlPlane(
	ctx context.Context,
	profiles map[string]*profileRuntime,
	callers map[string]string,
	specRegistry map[string]domain.ServerSpec,
	scheduler domain.Scheduler,
	runtime domain.RuntimeConfig,
) *ControlPlane {
	store := domain.ProfileStore{
		Profiles: map[string]domain.Profile{},
		Callers:  callers,
	}
	summary := profileSummary{
		configs:        map[string]profileConfig{},
		specRegistry:   specRegistry,
		defaultRuntime: runtime,
	}
	state := newControlPlaneState(ctx, profiles, scheduler, nil, store, summary, zap.NewNop())
	registry := newCallerRegistry(state)
	discovery := newDiscoveryService(state, registry)
	observability := newObservabilityService(state, registry, nil)
	automation := newAutomationService(state, registry, discovery)
	return NewControlPlane(state, registry, discovery, observability, automation)
}

type minReadyCall struct {
	specKey  string
	minReady int
}

type stopCall struct {
	specKey string
	reason  string
}

type fakeScheduler struct {
	minReadyCalls []minReadyCall
	stopCalls     []stopCall
}

func (f *fakeScheduler) Acquire(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	return nil, nil
}

func (f *fakeScheduler) AcquireReady(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	return nil, nil
}

func (f *fakeScheduler) Release(ctx context.Context, instance *domain.Instance) error {
	return nil
}

func (f *fakeScheduler) SetDesiredMinReady(ctx context.Context, specKey string, minReady int) error {
	f.minReadyCalls = append(f.minReadyCalls, minReadyCall{specKey: specKey, minReady: minReady})
	return nil
}

func (f *fakeScheduler) StopSpec(ctx context.Context, specKey, reason string) error {
	f.stopCalls = append(f.stopCalls, stopCall{specKey: specKey, reason: reason})
	return nil
}

func (f *fakeScheduler) StartIdleManager(interval time.Duration) {}
func (f *fakeScheduler) StopIdleManager()                        {}
func (f *fakeScheduler) StartPingManager(interval time.Duration) {}
func (f *fakeScheduler) StopPingManager()                        {}
func (f *fakeScheduler) StopAll(ctx context.Context)             {}

func (f *fakeScheduler) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	return nil, nil
}
