package controlplane

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/app/runtime"
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
	specKey := domain.SpecFingerprint(spec)

	sched := &fakeScheduler{}
	cp := newTestControlPlane(context.Background(), domain.Catalog{
		Specs:   map[string]domain.ServerSpec{spec.Name: spec},
		Runtime: domain.RuntimeConfig{},
	}, sched)

	registration, err := cp.RegisterClient(context.Background(), "client", 1234, nil, "")
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
	runtimeState := runtime.NewStateFromSpecKeys(state.Summary.ServerSpecKeys)
	controlState := NewState(ctx, runtimeState, scheduler, nil, nil, &state, zap.NewNop())
	registry := NewClientRegistry(controlState)
	discovery := NewDiscoveryService(controlState, registry)
	observability := NewObservabilityService(controlState, registry, nil)
	automation := NewAutomationService(controlState, registry, discovery)
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

func (f *fakeScheduler) Acquire(_ context.Context, _, _ string) (*domain.Instance, error) {
	return nil, nil
}

func (f *fakeScheduler) AcquireReady(_ context.Context, _, _ string) (*domain.Instance, error) {
	return nil, nil
}

func (f *fakeScheduler) Release(_ context.Context, _ *domain.Instance) error {
	return nil
}

func (f *fakeScheduler) ApplyCatalogDiff(_ context.Context, _ domain.CatalogDiff, _ map[string]domain.ServerSpec) error {
	return nil
}

func (f *fakeScheduler) SetDesiredMinReady(_ context.Context, specKey string, minReady int) error {
	f.minReadyCalls = append(f.minReadyCalls, minReadyCall{specKey: specKey, minReady: minReady})
	return nil
}

func (f *fakeScheduler) StopSpec(_ context.Context, specKey, reason string) error {
	f.stopCalls = append(f.stopCalls, stopCall{specKey: specKey, reason: reason})
	return nil
}

func (f *fakeScheduler) StartIdleManager(_ time.Duration) {}
func (f *fakeScheduler) StopIdleManager()                 {}
func (f *fakeScheduler) StartPingManager(_ time.Duration) {}
func (f *fakeScheduler) StopPingManager()                 {}
func (f *fakeScheduler) StopAll(_ context.Context)        {}

func (f *fakeScheduler) GetPoolStatus(_ context.Context) ([]domain.PoolInfo, error) {
	return nil, nil
}

func TestControlPlane_SharedServerNotKilled(t *testing.T) {
	spec := domain.ServerSpec{
		Name:            "git-server",
		Cmd:             []string{"/bin/git-server"},
		Tags:            []string{"git"},
		MaxConcurrent:   10,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	specKey := domain.SpecFingerprint(spec)

	sched := &fakeScheduler{}
	cp := newTestControlPlane(context.Background(), domain.Catalog{
		Specs:   map[string]domain.ServerSpec{spec.Name: spec},
		Runtime: domain.RuntimeConfig{},
	}, sched)

	// A Client A registers with tag: ["git"]
	_, err := cp.RegisterClient(context.Background(), "clientA", 1001, []string{"git"}, "")
	require.NoError(t, err)
	require.Equal(t, 1, len(sched.minReadyCalls))
	require.Equal(t, specKey, sched.minReadyCalls[0].specKey)
	require.Equal(t, 1, sched.minReadyCalls[0].minReady)

	// Client B registers with tag: ["git"] (sharing the same server)
	_, err = cp.RegisterClient(context.Background(), "clientB", 1002, []string{"git"}, "")
	require.NoError(t, err)
	// Key assertion: server should not be started again
	require.Equal(t, 1, len(sched.minReadyCalls), "should not start server again")

	// Verify specCounts = 2
	cp.registry.mu.Lock()
	require.Equal(t, 2, cp.registry.specCounts[specKey], "both clients should reference the server")
	cp.registry.mu.Unlock()

	// Client B unregisters
	require.NoError(t, cp.UnregisterClient(context.Background(), "clientB"))

	// Server should NOT be stopped because clientA is still active
	require.Equal(t, 0, len(sched.stopCalls), "server should NOT be stopped while clientA is still active")

	// Verify specCounts = 1
	cp.registry.mu.Lock()
	require.Equal(t, 1, cp.registry.specCounts[specKey], "clientA should still reference the server")
	cp.registry.mu.Unlock()

	// Client A unregisters
	require.NoError(t, cp.UnregisterClient(context.Background(), "clientA"))

	require.Equal(t, 1, len(sched.stopCalls), "server should be stopped after all clients exit")
	require.Equal(t, specKey, sched.stopCalls[0].specKey)
	require.Equal(t, "client inactive", sched.stopCalls[0].reason)

	// Verify specCounts is removed
	cp.registry.mu.Lock()
	_, exists := cp.registry.specCounts[specKey]
	require.False(t, exists, "spec should be removed from specCounts")
	cp.registry.mu.Unlock()
}

func TestControlPlane_SharedServerMultipleTags(t *testing.T) {
	// Create two servers, one with the "git" tag and one with the "docker" tag
	gitSpec := domain.ServerSpec{
		Name:            "git-server",
		Cmd:             []string{"/bin/git"},
		Tags:            []string{"git"},
		MaxConcurrent:   10,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	dockerSpec := domain.ServerSpec{
		Name:            "docker-server",
		Cmd:             []string{"/bin/docker"},
		Tags:            []string{"docker"},
		MaxConcurrent:   10,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	gitKey := domain.SpecFingerprint(gitSpec)
	dockerKey := domain.SpecFingerprint(dockerSpec)

	sched := &fakeScheduler{}
	cp := newTestControlPlane(context.Background(), domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			gitSpec.Name:    gitSpec,
			dockerSpec.Name: dockerSpec,
		},
		Runtime: domain.RuntimeConfig{},
	}, sched)

	// Client A registers with tags: ["git", "docker"]
	_, err := cp.RegisterClient(context.Background(), "clientA", 1001, []string{"git", "docker"}, "")
	require.NoError(t, err)
	require.Equal(t, 2, len(sched.minReadyCalls), "should start both servers")

	// Client B registers with tag: ["git"] (sharing the same git-server)
	_, err = cp.RegisterClient(context.Background(), "clientB", 1002, []string{"git"}, "")
	require.NoError(t, err)

	// Verify specCounts
	cp.registry.mu.Lock()
	require.Equal(t, 2, cp.registry.specCounts[gitKey], "git server shared by 2 clients")
	require.Equal(t, 1, cp.registry.specCounts[dockerKey], "docker server used by 1 client")
	cp.registry.mu.Unlock()

	// Client A unregisters
	require.NoError(t, cp.UnregisterClient(context.Background(), "clientA"))

	// docker-server should be stopped, but git-server should not
	require.Equal(t, 1, len(sched.stopCalls), "only docker server should be stopped")
	require.Equal(t, dockerKey, sched.stopCalls[0].specKey)

	// Verify specCounts
	cp.registry.mu.Lock()
	require.Equal(t, 1, cp.registry.specCounts[gitKey], "git server still used by clientB")
	_, exists := cp.registry.specCounts[dockerKey]
	require.False(t, exists, "docker server should be removed")
	cp.registry.mu.Unlock()

	// Client B unregisters
	require.NoError(t, cp.UnregisterClient(context.Background(), "clientB"))

	// Now git-server should also be stopped
	require.Equal(t, 2, len(sched.stopCalls), "git server should now be stopped")
	require.Equal(t, gitKey, sched.stopCalls[1].specKey)
}
