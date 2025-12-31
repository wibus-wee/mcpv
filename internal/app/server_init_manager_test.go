package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

func TestServerInitializationManager_Ready(t *testing.T) {
	specKey := "alpha"
	spec := domain.ServerSpec{Name: "alpha", MinReady: 1}
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 1},
		},
	})

	manager := NewServerInitializationManager(scheduler, newTestSnapshot(map[string]domain.ServerSpec{specKey: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)

	status := waitForStatus(t, manager, specKey, domain.ServerInitReady, 1)
	require.Equal(t, 0, status.Failed)
	require.Equal(t, "", status.LastError)
}

func TestServerInitializationManager_DegradedThenReady(t *testing.T) {
	specKey := "beta"
	spec := domain.ServerSpec{Name: "beta", MinReady: 2}
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 1, failed: 1, err: errors.New("initialization error")},
			{ready: 2, failed: 1},
		},
	})

	manager := NewServerInitializationManager(scheduler, newTestSnapshot(map[string]domain.ServerSpec{specKey: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)

	status := waitForStatus(t, manager, specKey, domain.ServerInitReady, 2)
	require.Equal(t, 1, status.Failed)
	require.Empty(t, status.LastError)
}

func TestServerInitializationManager_Cancelled(t *testing.T) {
	specKey := "gamma"
	spec := domain.ServerSpec{Name: "gamma", MinReady: 1}
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 0, failed: 0, err: errors.New("start failed")},
		},
	})

	manager := NewServerInitializationManager(scheduler, newTestSnapshot(map[string]domain.ServerSpec{specKey: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())

	manager.Start(ctx)
	cancel()

	status := waitForStatus(t, manager, specKey, domain.ServerInitFailed, 0)
	require.Contains(t, status.LastError, "context canceled")
}

func TestServerInitializationManager_SuspendsAfterRetries(t *testing.T) {
	specKey := "delta"
	spec := domain.ServerSpec{Name: "delta", MinReady: 1}
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 0, failed: 0, err: errors.New("start failed")},
			{ready: 0, failed: 0, err: errors.New("start failed")},
		},
	})

	manager := NewServerInitializationManager(scheduler, newTestSnapshot(map[string]domain.ServerSpec{specKey: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)

	status := waitForStatus(t, manager, specKey, domain.ServerInitSuspended, 0)
	require.Equal(t, 2, status.RetryCount)
	require.Contains(t, status.LastError, "retry limit reached")
}

func TestServerInitializationManager_RetrySpecResets(t *testing.T) {
	specKey := "epsilon"
	spec := domain.ServerSpec{Name: "epsilon", MinReady: 1}
	scheduler := newInitSchedulerStub(map[string][]setResult{
		specKey: {
			{ready: 0, failed: 0, err: errors.New("start failed")},
			{ready: 0, failed: 0, err: errors.New("start failed")},
			{ready: 1, failed: 0},
		},
	})

	manager := NewServerInitializationManager(scheduler, newTestSnapshot(map[string]domain.ServerSpec{specKey: spec}, initRuntimeConfig(2)), zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)

	status := waitForStatus(t, manager, specKey, domain.ServerInitSuspended, 0)
	require.Equal(t, 2, status.RetryCount)

	require.NoError(t, manager.RetrySpec(specKey))

	status = waitForStatus(t, manager, specKey, domain.ServerInitReady, 1)
	require.Equal(t, 0, status.RetryCount)
}

func waitForStatus(t *testing.T, manager *ServerInitializationManager, specKey string, state domain.ServerInitState, ready int) domain.ServerInitStatus {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, status := range manager.Statuses() {
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
	}
}

func newTestSummary(specs map[string]domain.ServerSpec, runtime domain.RuntimeConfig) profileSummary {
	return profileSummary{
		configs:        map[string]profileConfig{},
		specRegistry:   specs,
		defaultRuntime: runtime,
	}
}

func newTestSnapshot(specs map[string]domain.ServerSpec, runtime domain.RuntimeConfig) *CatalogSnapshot {
	return &CatalogSnapshot{
		store:   domain.ProfileStore{},
		summary: newTestSummary(specs, runtime),
	}
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
}

func newInitSchedulerStub(results map[string][]setResult) *initSchedulerStub {
	return &initSchedulerStub{
		results: results,
		ready:   make(map[string]int),
		failed:  make(map[string]int),
	}
}

func (s *initSchedulerStub) Acquire(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	return nil, nil
}

func (s *initSchedulerStub) AcquireReady(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	return nil, nil
}

func (s *initSchedulerStub) Release(ctx context.Context, instance *domain.Instance) error {
	return nil
}

func (s *initSchedulerStub) SetDesiredMinReady(ctx context.Context, specKey string, minReady int) error {
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
	return result.err
}

func (s *initSchedulerStub) StopSpec(ctx context.Context, specKey, reason string) error {
	return nil
}

func (s *initSchedulerStub) StartIdleManager(interval time.Duration) {}
func (s *initSchedulerStub) StopIdleManager()                        {}
func (s *initSchedulerStub) StartPingManager(interval time.Duration) {}
func (s *initSchedulerStub) StopPingManager()                        {}
func (s *initSchedulerStub) StopAll(ctx context.Context)             {}

func (s *initSchedulerStub) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pools := make([]domain.PoolInfo, 0, len(s.ready))
	for specKey, ready := range s.ready {
		failed := s.failed[specKey]
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
			MinReady:   ready,
			Instances:  instances,
		})
	}
	return pools, nil
}
