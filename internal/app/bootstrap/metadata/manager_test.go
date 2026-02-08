package metadata

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

func TestBootstrapManager_InitialState(t *testing.T) {
	cache := domain.NewMetadataCache()
	manager := NewManager(Options{
		Scheduler:   &minimalSchedulerStub{},
		Lifecycle:   &minimalLifecycleStub{},
		Specs:       map[string]domain.ServerSpec{},
		SpecKeys:    map[string]string{},
		Runtime:     domain.RuntimeConfig{},
		Cache:       cache,
		Logger:      zap.NewNop(),
		Concurrency: 3,
		Timeout:     30 * time.Second,
		Mode:        domain.BootstrapModeMetadata,
	})

	// Initial state should be pending
	require.False(t, manager.IsCompleted())
	progress := manager.GetProgress()
	require.Equal(t, domain.BootstrapPending, progress.State)
	require.Equal(t, 0, progress.Total)
	require.Equal(t, 0, progress.Completed)
	require.Equal(t, 0, progress.Failed)
}

func TestBootstrapManager_WaitForCompletion_AlreadyCompleted(t *testing.T) {
	cache := domain.NewMetadataCache()
	manager := NewManager(Options{
		Scheduler:   &minimalSchedulerStub{},
		Lifecycle:   &minimalLifecycleStub{},
		Specs:       map[string]domain.ServerSpec{},
		SpecKeys:    map[string]string{},
		Runtime:     domain.RuntimeConfig{},
		Cache:       cache,
		Logger:      zap.NewNop(),
		Concurrency: 1,
		Timeout:     1 * time.Second,
		Mode:        domain.BootstrapModeMetadata,
	})

	// Bootstrap with no servers should complete immediately
	manager.Bootstrap(context.Background())

	// Should be able to wait multiple times
	ctx := context.Background()
	require.NoError(t, manager.WaitForCompletion(ctx))
	require.NoError(t, manager.WaitForCompletion(ctx))

	require.True(t, manager.IsCompleted())
}

func TestBootstrapManager_GetProgress_ThreadSafe(t *testing.T) {
	cache := domain.NewMetadataCache()
	manager := NewManager(Options{
		Scheduler:   &minimalSchedulerStub{},
		Lifecycle:   &minimalLifecycleStub{},
		Specs:       map[string]domain.ServerSpec{},
		SpecKeys:    map[string]string{},
		Runtime:     domain.RuntimeConfig{},
		Cache:       cache,
		Logger:      zap.NewNop(),
		Concurrency: 1,
		Timeout:     1 * time.Second,
		Mode:        domain.BootstrapModeMetadata,
	})

	// Start bootstrap
	manager.Bootstrap(context.Background())

	// Should be able to call GetProgress concurrently without races
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = manager.GetProgress()
				_ = manager.IsCompleted()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	require.NotNil(t, manager)
}

// Minimal test stubs

type minimalSchedulerStub struct{}

func (s *minimalSchedulerStub) Acquire(_ context.Context, _, _ string) (*domain.Instance, error) {
	return nil, nil
}

func (s *minimalSchedulerStub) AcquireReady(_ context.Context, _, _ string) (*domain.Instance, error) {
	return nil, domain.ErrNoReadyInstance
}

func (s *minimalSchedulerStub) Release(_ context.Context, _ *domain.Instance) error {
	return nil
}

func (s *minimalSchedulerStub) SetDesiredMinReady(_ context.Context, _ string, _ int) error {
	return nil
}

func (s *minimalSchedulerStub) StopSpec(_ context.Context, _, _ string) error {
	return nil
}

func (s *minimalSchedulerStub) ApplyCatalogDiff(_ context.Context, _ domain.CatalogDiff, _ map[string]domain.ServerSpec) error {
	return nil
}

func (s *minimalSchedulerStub) StartIdleManager(_ time.Duration) {}
func (s *minimalSchedulerStub) StopIdleManager()                 {}
func (s *minimalSchedulerStub) StartPingManager(_ time.Duration) {}
func (s *minimalSchedulerStub) StopPingManager()                 {}
func (s *minimalSchedulerStub) StopAll(_ context.Context)        {}

func (s *minimalSchedulerStub) GetPoolStatus(_ context.Context) ([]domain.PoolInfo, error) {
	return nil, nil
}

type minimalLifecycleStub struct{}

func (l *minimalLifecycleStub) StartInstance(_ context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	return domain.NewInstance(domain.InstanceOptions{
		SpecKey: specKey,
		Spec:    spec,
	}), nil
}

func (l *minimalLifecycleStub) StopInstance(_ context.Context, _ *domain.Instance, _ string) error {
	return nil
}
