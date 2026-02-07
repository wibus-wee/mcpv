package registry

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/domain"
)

type fakeState struct {
	catalog        domain.Catalog
	serverSpecKeys map[string]string
	specRegistry   map[string]domain.ServerSpec
	runtime        domain.RuntimeConfig
	scheduler      domain.Scheduler
	logger         *zap.Logger
	ctx            context.Context
}

func newFakeState(ctx context.Context, catalog domain.Catalog, scheduler domain.Scheduler) *fakeState {
	serverSpecKeys := make(map[string]string)
	specRegistry := make(map[string]domain.ServerSpec)
	for name, spec := range catalog.Specs {
		specKey := domain.SpecFingerprint(spec)
		serverSpecKeys[name] = specKey
		specRegistry[specKey] = spec
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return &fakeState{
		catalog:        catalog,
		serverSpecKeys: serverSpecKeys,
		specRegistry:   specRegistry,
		runtime:        catalog.Runtime,
		scheduler:      scheduler,
		logger:         zap.NewNop(),
		ctx:            ctx,
	}
}

func (f *fakeState) Catalog() domain.Catalog {
	return f.catalog
}

func (f *fakeState) ServerSpecKeys() map[string]string {
	return f.serverSpecKeys
}

func (f *fakeState) SpecRegistry() map[string]domain.ServerSpec {
	return f.specRegistry
}

func (f *fakeState) Runtime() domain.RuntimeConfig {
	return f.runtime
}

func (f *fakeState) Logger() *zap.Logger {
	return f.logger
}

func (f *fakeState) Context() context.Context {
	return f.ctx
}

func (f *fakeState) Scheduler() domain.Scheduler {
	return f.scheduler
}

func (f *fakeState) InitManager() *bootstrap.ServerInitializationManager {
	return nil
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

func TestRegistry_ReapDeadClients_Heartbeat(t *testing.T) {
	runtime := domain.RuntimeConfig{ClientCheckSeconds: 1, ClientInactiveSeconds: 60}
	sched := &fakeScheduler{}
	state := newFakeState(context.Background(), domain.Catalog{
		Specs:   map[string]domain.ServerSpec{},
		Runtime: runtime,
	}, sched)
	reg := NewClientRegistry(state)

	reg.mu.Lock()
	reg.activeClients["client"] = clientState{
		pid:           -1,
		lastHeartbeat: time.Now(),
	}
	reg.mu.Unlock()

	reg.reapDeadClients(context.Background())
	_, err := reg.ResolveClientTags("client")
	require.NoError(t, err)

	reg.mu.Lock()
	reg.activeClients["client"] = clientState{
		pid:           -1,
		lastHeartbeat: time.Now().Add(-time.Minute),
	}
	reg.mu.Unlock()

	reg.reapDeadClients(context.Background())
	_, err = reg.ResolveClientTags("client")
	require.ErrorIs(t, err, domain.ErrClientNotRegistered)
}

func TestRegistry_ReapDeadClients_TTL(t *testing.T) {
	runtime := domain.RuntimeConfig{ClientCheckSeconds: 1, ClientInactiveSeconds: 1}
	sched := &fakeScheduler{}
	state := newFakeState(context.Background(), domain.Catalog{
		Specs:   map[string]domain.ServerSpec{},
		Runtime: runtime,
	}, sched)
	reg := NewClientRegistry(state)

	reg.mu.Lock()
	reg.activeClients["client"] = clientState{
		pid:           os.Getpid(),
		lastHeartbeat: time.Now().Add(-2 * time.Second),
	}
	reg.mu.Unlock()

	reg.reapDeadClients(context.Background())
	_, err := reg.ResolveClientTags("client")
	require.ErrorIs(t, err, domain.ErrClientNotRegistered)
}

func TestRegistry_SharedServerNotKilled(t *testing.T) {
	spec := domain.ServerSpec{
		Name:            "git-server",
		Cmd:             []string{"/bin/git-server"},
		Tags:            []string{"git"},
		MaxConcurrent:   10,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	specKey := domain.SpecFingerprint(spec)

	sched := &fakeScheduler{}
	state := newFakeState(context.Background(), domain.Catalog{
		Specs:   map[string]domain.ServerSpec{spec.Name: spec},
		Runtime: domain.RuntimeConfig{},
	}, sched)
	reg := NewClientRegistry(state)

	_, err := reg.RegisterClient(context.Background(), "clientA", 1001, []string{"git"}, "")
	require.NoError(t, err)
	require.Equal(t, 1, len(sched.minReadyCalls))
	require.Equal(t, specKey, sched.minReadyCalls[0].specKey)
	require.Equal(t, 1, sched.minReadyCalls[0].minReady)

	_, err = reg.RegisterClient(context.Background(), "clientB", 1002, []string{"git"}, "")
	require.NoError(t, err)
	require.Equal(t, 1, len(sched.minReadyCalls), "should not start server again")

	reg.mu.Lock()
	require.Equal(t, 2, reg.specCounts[specKey], "both clients should reference the server")
	reg.mu.Unlock()

	require.NoError(t, reg.UnregisterClient(context.Background(), "clientB"))
	require.Equal(t, 0, len(sched.stopCalls), "server should NOT be stopped while clientA is still active")

	reg.mu.Lock()
	require.Equal(t, 1, reg.specCounts[specKey], "clientA should still reference the server")
	reg.mu.Unlock()

	require.NoError(t, reg.UnregisterClient(context.Background(), "clientA"))
	require.Equal(t, 1, len(sched.stopCalls), "server should be stopped after all clients exit")
	require.Equal(t, specKey, sched.stopCalls[0].specKey)
	require.Equal(t, "client inactive", sched.stopCalls[0].reason)

	reg.mu.Lock()
	_, exists := reg.specCounts[specKey]
	require.False(t, exists, "spec should be removed from specCounts")
	reg.mu.Unlock()
}

func TestRegistry_SharedServerMultipleTags(t *testing.T) {
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
	state := newFakeState(context.Background(), domain.Catalog{
		Specs: map[string]domain.ServerSpec{
			gitSpec.Name:    gitSpec,
			dockerSpec.Name: dockerSpec,
		},
		Runtime: domain.RuntimeConfig{},
	}, sched)
	reg := NewClientRegistry(state)

	_, err := reg.RegisterClient(context.Background(), "clientA", 1001, []string{"git", "docker"}, "")
	require.NoError(t, err)
	require.Equal(t, 2, len(sched.minReadyCalls), "should start both servers")

	_, err = reg.RegisterClient(context.Background(), "clientB", 1002, []string{"git"}, "")
	require.NoError(t, err)

	reg.mu.Lock()
	require.Equal(t, 2, reg.specCounts[gitKey], "git server shared by 2 clients")
	require.Equal(t, 1, reg.specCounts[dockerKey], "docker server used by 1 client")
	reg.mu.Unlock()

	require.NoError(t, reg.UnregisterClient(context.Background(), "clientA"))
	require.Equal(t, 1, len(sched.stopCalls), "only docker server should be stopped")
	require.Equal(t, dockerKey, sched.stopCalls[0].specKey)

	reg.mu.Lock()
	require.Equal(t, 1, reg.specCounts[gitKey], "git server still used by clientB")
	_, exists := reg.specCounts[dockerKey]
	require.False(t, exists, "docker server should be removed")
	reg.mu.Unlock()

	require.NoError(t, reg.UnregisterClient(context.Background(), "clientB"))
	require.Equal(t, 2, len(sched.stopCalls), "git server should now be stopped")
	require.Equal(t, gitKey, sched.stopCalls[1].specKey)
}
