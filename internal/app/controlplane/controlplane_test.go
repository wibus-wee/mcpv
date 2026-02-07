package controlplane

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/app/runtime"
	"mcpv/internal/domain"
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
	tools := NewToolDiscoveryService(controlState, registry)
	resources := NewResourceDiscoveryService(controlState, registry)
	prompts := NewPromptDiscoveryService(controlState, registry)
	observability := NewObservabilityService(controlState, registry, nil)
	automation := NewAutomationService(controlState, registry, tools)
	return NewControlPlane(controlState, registry, tools, resources, prompts, observability, automation)
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
