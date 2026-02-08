package serverinit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

func TestManager_Ready(t *testing.T) {
	spec := domain.ServerSpec{Name: "alpha", MinReady: 1, ActivationMode: domain.ActivationAlwaysOn}
	specKey := specKeyFor(t, spec)
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 1},
		},
	})

	manager := NewManager(scheduler, newTestState(map[string]domain.ServerSpec{spec.Name: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)

	status := waitForStatus(t, manager, specKey, domain.ServerInitReady, 1)
	require.Equal(t, 0, status.Failed)
	require.Equal(t, "", status.LastError)
}

func TestManager_DegradedThenReady(t *testing.T) {
	spec := domain.ServerSpec{Name: "beta", MinReady: 2, ActivationMode: domain.ActivationAlwaysOn}
	specKey := specKeyFor(t, spec)
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 1, failed: 1, err: errors.New("initialization error")},
			{ready: 2, failed: 1},
		},
	})

	manager := NewManager(scheduler, newTestState(map[string]domain.ServerSpec{spec.Name: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)

	status := waitForStatus(t, manager, specKey, domain.ServerInitReady, 2)
	require.Equal(t, 1, status.Failed)
	require.Empty(t, status.LastError)
}

func TestManager_Cancelled(t *testing.T) {
	spec := domain.ServerSpec{Name: "gamma", MinReady: 1, ActivationMode: domain.ActivationAlwaysOn}
	specKey := specKeyFor(t, spec)
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 0, failed: 0, err: errors.New("start failed")},
		},
	})

	manager := NewManager(scheduler, newTestState(map[string]domain.ServerSpec{spec.Name: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())

	manager.Start(ctx)
	cancel()

	status := waitForStatus(t, manager, specKey, domain.ServerInitFailed, 0)
	require.Contains(t, status.LastError, "context canceled")
}

func TestManager_SuspendsAfterRetries(t *testing.T) {
	spec := domain.ServerSpec{Name: "delta", MinReady: 1, ActivationMode: domain.ActivationAlwaysOn}
	specKey := specKeyFor(t, spec)
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 0, failed: 0, err: errors.New("start failed")},
			{ready: 0, failed: 0, err: errors.New("start failed")},
		},
	})

	manager := NewManager(scheduler, newTestState(map[string]domain.ServerSpec{spec.Name: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)

	status := waitForStatus(t, manager, specKey, domain.ServerInitSuspended, 0)
	require.Equal(t, 2, status.RetryCount)
	require.Contains(t, status.LastError, "retry limit reached")
}

func TestManager_RetrySpecResets(t *testing.T) {
	spec := domain.ServerSpec{Name: "epsilon", MinReady: 1, ActivationMode: domain.ActivationAlwaysOn}
	specKey := specKeyFor(t, spec)
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 0, failed: 0, err: errors.New("start failed")},
			{ready: 0, failed: 0, err: errors.New("start failed")},
			{ready: 1, failed: 0},
		},
	})

	manager := NewManager(scheduler, newTestState(map[string]domain.ServerSpec{spec.Name: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)

	status := waitForStatus(t, manager, specKey, domain.ServerInitSuspended, 0)
	require.Equal(t, 2, status.RetryCount)

	require.NoError(t, manager.RetrySpec(specKey))

	status = waitForStatus(t, manager, specKey, domain.ServerInitReady, 1)
	require.Equal(t, 0, status.RetryCount)
}

func TestManager_OnDemandReadyWithoutInstances(t *testing.T) {
	// On-demand server with minReady=0 should report ready immediately
	// because the spec/metadata is loaded successfully, no instances needed.
	spec := domain.ServerSpec{Name: "ondemand", MinReady: 0, ActivationMode: domain.ActivationOnDemand}
	specKey := specKeyFor(t, spec)
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 0, failed: 0}, // No instances, no error
		},
	})

	runtime := initRuntimeConfig(2)
	runtime.DefaultActivationMode = domain.ActivationOnDemand
	manager := NewManager(scheduler, newTestState(map[string]domain.ServerSpec{spec.Name: spec}, runtime), zap.NewNop())
	ctx := t.Context()

	manager.Start(ctx)

	// Should be ready with 0 instances since target is 0 (on-demand not activated)
	status := waitForStatus(t, manager, specKey, domain.ServerInitReady, 0)
	require.Equal(t, 0, status.MinReady)
	require.Equal(t, 0, status.Failed)
	require.Empty(t, status.LastError)
}

func waitForStatus(t *testing.T, manager *Manager, specKey string, state domain.ServerInitState, ready int) domain.ServerInitStatus {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, status := range manager.Statuses(context.Background()) {
			if status.SpecKey != specKey {
				continue
			}
			if status.State == state && status.Ready == ready {
				return status
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("status not reached for %s: expected state=%s ready=%d", specKey, state, ready)
	return domain.ServerInitStatus{}
}

func initRuntimeConfig(maxRetries int) domain.RuntimeConfig {
	return domain.RuntimeConfig{
		ServerInitRetryBaseSeconds: 1,
		ServerInitRetryMaxSeconds:  1,
		ServerInitMaxRetries:       maxRetries,
		DefaultActivationMode:      domain.ActivationAlwaysOn,
	}
}

func newTestState(specs map[string]domain.ServerSpec, runtime domain.RuntimeConfig) *domain.CatalogState {
	state, err := domain.NewCatalogState(domain.Catalog{
		Specs:   specs,
		Runtime: runtime,
	}, 0, time.Time{})
	if err != nil {
		panic(err)
	}
	return &state
}

func specKeyFor(t *testing.T, spec domain.ServerSpec) string {
	t.Helper()
	return domain.SpecFingerprint(spec)
}

type setResult struct {
	ready  int
	failed int
	err    error
}

type initSchedulerStub struct {
	results map[string][]setResult

	mu     sync.Mutex
	ready  map[string]int
	failed map[string]int
	min    map[string]int
}

func newInitSchedulerStub(results map[string][]setResult) *initSchedulerStub {
	return &initSchedulerStub{
		results: results,
		ready:   make(map[string]int),
		failed:  make(map[string]int),
		min:     make(map[string]int),
	}
}

func (s *initSchedulerStub) Acquire(_ context.Context, _, _ string) (*domain.Instance, error) {
	return nil, nil
}

func (s *initSchedulerStub) AcquireReady(_ context.Context, _, _ string) (*domain.Instance, error) {
	return nil, nil
}

func (s *initSchedulerStub) Release(_ context.Context, _ *domain.Instance) error {
	return nil
}

func (s *initSchedulerStub) SetDesiredMinReady(_ context.Context, specKey string, minReady int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := setResult{ready: minReady}
	if list, ok := s.results[specKey]; ok && len(list) > 0 {
		result = list[0]
		s.results[specKey] = list[1:]
	}

	if result.ready < 0 {
		result.ready = 0
	}
	if result.failed < 0 {
		result.failed = 0
	}

	s.ready[specKey] = result.ready
	s.failed[specKey] = result.failed
	s.min[specKey] = minReady
	return result.err
}

func (s *initSchedulerStub) StopSpec(_ context.Context, _, _ string) error {
	return nil
}

func (s *initSchedulerStub) ApplyCatalogDiff(_ context.Context, _ domain.CatalogDiff, _ map[string]domain.ServerSpec) error {
	return nil
}

func (s *initSchedulerStub) StartIdleManager(_ time.Duration) {}
func (s *initSchedulerStub) StopIdleManager()                 {}
func (s *initSchedulerStub) StartPingManager(_ time.Duration) {}
func (s *initSchedulerStub) StopPingManager()                 {}
func (s *initSchedulerStub) StopAll(_ context.Context)        {}

func (s *initSchedulerStub) GetPoolStatus(_ context.Context) ([]domain.PoolInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pools := make([]domain.PoolInfo, 0, len(s.ready))
	for specKey, ready := range s.ready {
		failed := s.failed[specKey]
		minReady := s.min[specKey]
		instances := make([]domain.InstanceInfo, 0, ready+failed)
		for i := 0; i < ready; i++ {
			instances = append(instances, domain.InstanceInfo{
				ID:    fmt.Sprintf("%s-ready-%d", specKey, i),
				State: domain.InstanceStateReady,
			})
		}
		for i := 0; i < failed; i++ {
			instances = append(instances, domain.InstanceInfo{
				ID:    fmt.Sprintf("%s-failed-%d", specKey, i),
				State: domain.InstanceStateFailed,
			})
		}
		pools = append(pools, domain.PoolInfo{
			SpecKey:    specKey,
			ServerName: specKey,
			MinReady:   minReady,
			Instances:  instances,
		})
	}
	return pools, nil
}
