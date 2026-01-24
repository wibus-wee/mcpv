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
	cp := newTestControlPlane(context.Background(), domain.Catalog{
		Specs:   map[string]domain.ServerSpec{},
		Runtime: domain.RuntimeConfig{},
	}, &fakeScheduler{})

	_, err := cp.ListTools(context.Background(), "client")
	require.ErrorIs(t, err, domain.ErrClientNotRegistered)
}

func TestControlPlane_RegisterUnregister(t *testing.T) {
	spec := domain.ServerSpec{
		Name:            "spec-a",
		Cmd:             []string{"/bin/true"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	specKey, err := domain.SpecFingerprint(spec)
	require.NoError(t, err)

	sched := &fakeScheduler{}
	cp := newTestControlPlane(context.Background(), domain.Catalog{
		Specs:   map[string]domain.ServerSpec{spec.Name: spec},
		Runtime: domain.RuntimeConfig{},
	}, sched)

	registration, err := cp.RegisterClient(context.Background(), "client", 1234, nil)
	require.NoError(t, err)
	require.Equal(t, "client", registration.Client)
	require.Equal(t, []minReadyCall{{specKey: specKey, minReady: 1}}, sched.minReadyCalls)

	require.NoError(t, cp.UnregisterClient(context.Background(), "client"))
	require.Equal(t, []stopCall{{specKey: specKey, reason: "client inactive"}}, sched.stopCalls)
}

func TestControlPlane_ReapDeadClients_Heartbeat(t *testing.T) {
	runtime := domain.RuntimeConfig{ClientCheckSeconds: 1, ClientInactiveSeconds: 60}
	cp := newTestControlPlane(context.Background(), domain.Catalog{
		Specs:   map[string]domain.ServerSpec{},
		Runtime: runtime,
	}, &fakeScheduler{})

	cp.registry.mu.Lock()
	cp.registry.activeClients["client"] = clientState{
		pid:           -1,
		lastHeartbeat: time.Now(),
	}
	cp.registry.mu.Unlock()

	cp.registry.reapDeadClients(context.Background())
	_, err := cp.registry.resolveClientTags("client")
	require.NoError(t, err)

	cp.registry.mu.Lock()
	cp.registry.activeClients["client"] = clientState{
		pid:           -1,
		lastHeartbeat: time.Now().Add(-time.Minute),
	}
	cp.registry.mu.Unlock()

	cp.registry.reapDeadClients(context.Background())
	_, err = cp.registry.resolveClientTags("client")
	require.ErrorIs(t, err, domain.ErrClientNotRegistered)
}

func TestControlPlane_ReapDeadClients_TTL(t *testing.T) {
	runtime := domain.RuntimeConfig{ClientCheckSeconds: 1, ClientInactiveSeconds: 1}
	cp := newTestControlPlane(context.Background(), domain.Catalog{
		Specs:   map[string]domain.ServerSpec{},
		Runtime: runtime,
	}, &fakeScheduler{})

	cp.registry.mu.Lock()
	cp.registry.activeClients["client"] = clientState{
		pid:           os.Getpid(),
		lastHeartbeat: time.Now().Add(-2 * time.Second),
	}
	cp.registry.mu.Unlock()

	cp.registry.reapDeadClients(context.Background())
	_, err := cp.registry.resolveClientTags("client")
	require.ErrorIs(t, err, domain.ErrClientNotRegistered)
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
	catalog domain.Catalog,
	scheduler domain.Scheduler,
) *ControlPlane {
	state, err := domain.NewCatalogState(catalog, 1, time.Now())
	if err != nil {
		panic(err)
	}
	runtime := &runtimeState{
		specKeys: copySpecKeyMap(state.Summary.ServerSpecKeys),
	}
	controlState := newControlPlaneState(ctx, runtime, scheduler, nil, nil, &state, zap.NewNop())
	registry := newClientRegistry(controlState)
	discovery := newDiscoveryService(controlState, registry)
	observability := newObservabilityService(controlState, registry, nil)
	automation := newAutomationService(controlState, registry, discovery)
	return NewControlPlane(controlState, registry, discovery, observability, automation)
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

func (f *fakeScheduler) ApplyCatalogDiff(ctx context.Context, diff domain.CatalogDiff, registry map[string]domain.ServerSpec) error {
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
