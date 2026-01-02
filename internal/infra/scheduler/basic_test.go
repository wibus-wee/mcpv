package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

func TestBasicScheduler_StartsAndReusesInstance(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   2,
		IdleSeconds:     10,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	inst1, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.Equal(t, 1, inst1.BusyCount)

	inst2, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.Same(t, inst1, inst2)
	require.Equal(t, 2, inst1.BusyCount)

	require.NoError(t, s.Release(context.Background(), inst1))
	require.Equal(t, domain.InstanceStateBusy, inst1.State)
	require.NoError(t, s.Release(context.Background(), inst1))
	require.Equal(t, domain.InstanceStateReady, inst1.State)
}

func TestBasicScheduler_StickyBinding(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		Strategy:        domain.StrategyStateful,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	inst1, err := s.Acquire(context.Background(), "svc", "userA")
	require.NoError(t, err)

	inst2, err := s.Acquire(context.Background(), "svc", "userA")
	require.ErrorIs(t, err, ErrStickyBusy)
	require.Nil(t, inst2)

	_ = s.Release(context.Background(), inst1)

	inst3, err := s.Acquire(context.Background(), "svc", "userA")
	require.NoError(t, err)
	require.Same(t, inst1, inst3)
}

func TestBasicScheduler_UnknownServer(t *testing.T) {
	s, err := NewBasicScheduler(&fakeLifecycle{}, map[string]domain.ServerSpec{}, SchedulerOptions{})
	require.NoError(t, err)

	_, err = s.Acquire(context.Background(), "missing", "")
	require.ErrorIs(t, err, ErrUnknownSpecKey)
}

func TestBasicScheduler_AcquireReadyDoesNotStart(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	_, err = s.AcquireReady(context.Background(), "svc", "")
	require.ErrorIs(t, err, domain.ErrNoReadyInstance)
	require.Equal(t, 0, lc.counter)
}

func TestBasicScheduler_IdleReapRespectsMinReady(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        1,
		Strategy:        domain.StrategyStateless,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)
	require.NoError(t, s.SetDesiredMinReady(context.Background(), "svc", 1))

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.NoError(t, s.Release(context.Background(), inst))

	s.reapIdle()
	require.Equal(t, domain.InstanceStateReady, inst.State)
}

func TestBasicScheduler_IdleReapStopsWhenBelowMinReady(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		Strategy:        domain.StrategyStateless,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	s.reapIdle()
	require.Equal(t, domain.InstanceStateStopped, inst.State)
}

func TestBasicScheduler_StatefulWithBindingSkipsIdle(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:              "svc",
		Cmd:               []string{"./svc"},
		MaxConcurrent:     1,
		IdleSeconds:       0,
		Strategy:          domain.StrategyStateful,
		SessionTTLSeconds: 3600, // 1 hour, won't expire
		ProtocolVersion:   domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "rk")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	// Instance should not be reaped because it has an active binding
	s.reapIdle()
	require.Equal(t, domain.InstanceStateReady, inst.State)
}

func TestBasicScheduler_StatefulSessionTTLLimitsBindings(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:              "svc",
		Cmd:               []string{"./svc"},
		MaxConcurrent:     1,
		IdleSeconds:       0,
		Strategy:          domain.StrategyStateful,
		SessionTTLSeconds: 1,
		ProtocolVersion:   domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{
		Logger: zap.NewNop(),
	})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "rk")
	require.NoError(t, err)
	require.Equal(t, "rk", inst.StickyKey)
	require.NoError(t, s.Release(context.Background(), inst))

	state := s.getPool("svc", spec)
	state.mu.Lock()
	binding := state.sticky["rk"]
	require.NotNil(t, binding)
	binding.lastAccess = time.Now().Add(-2 * time.Second)
	state.mu.Unlock()

	s.reapStaleBindings()

	state.mu.Lock()
	_, exists := state.sticky["rk"]
	state.mu.Unlock()
	require.False(t, exists)
	require.Equal(t, "", inst.StickyKey)
}

func TestBasicScheduler_StatefulSessionTTLZeroKeepsBindings(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:              "svc",
		Cmd:               []string{"./svc"},
		MaxConcurrent:     1,
		IdleSeconds:       0,
		Strategy:          domain.StrategyStateful,
		SessionTTLSeconds: 0,
		ProtocolVersion:   domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "rk")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	state := s.getPool("svc", spec)
	state.mu.Lock()
	binding := state.sticky["rk"]
	require.NotNil(t, binding)
	binding.lastAccess = time.Now().Add(-2 * time.Second)
	state.mu.Unlock()

	s.reapStaleBindings()

	state.mu.Lock()
	_, exists := state.sticky["rk"]
	state.mu.Unlock()
	require.True(t, exists)
	require.Equal(t, "rk", inst.StickyKey)
}

func TestBasicScheduler_IdleReapSkipsPersistentAndSingleton(t *testing.T) {
	cases := []struct {
		name     string
		strategy domain.InstanceStrategy
	}{
		{name: "persistent", strategy: domain.StrategyPersistent},
		{name: "singleton", strategy: domain.StrategySingleton},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lc := &fakeLifecycle{}
			spec := domain.ServerSpec{
				Name:            "svc",
				Cmd:             []string{"./svc"},
				MaxConcurrent:   1,
				IdleSeconds:     0,
				Strategy:        tc.strategy,
				ProtocolVersion: domain.DefaultProtocolVersion,
			}
			s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
			require.NoError(t, err)

			inst, err := s.Acquire(context.Background(), "svc", "")
			require.NoError(t, err)
			require.NoError(t, s.Release(context.Background(), inst))

			s.reapIdle()
			require.Equal(t, domain.InstanceStateReady, inst.State)

			state := s.getPool("svc", spec)
			state.mu.Lock()
			require.Len(t, state.instances, 1)
			state.mu.Unlock()
		})
	}
}

func TestBasicScheduler_PingFailureStopsInstance(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     10,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{
		Probe:  &fakeProbe{err: errors.New("ping failed")},
		Logger: zap.NewNop(),
	})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	s.probeInstances()

	require.Equal(t, domain.InstanceStateStopped, inst.State)
	specKey, err := domain.SpecFingerprint(spec)
	require.NoError(t, err)
	state := s.getPool(specKey, spec)
	state.mu.Lock()
	defer state.mu.Unlock()
	require.Len(t, state.instances, 0)
}

func TestBasicScheduler_SharedPool(t *testing.T) {
	lc := &fakeLifecycle{}
	specA := domain.ServerSpec{
		Name:            "svc-a",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   2,
		IdleSeconds:     10,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	specB := specA
	specB.Name = "svc-b"

	specKeyA, err := domain.SpecFingerprint(specA)
	require.NoError(t, err)
	specKeyB, err := domain.SpecFingerprint(specB)
	require.NoError(t, err)
	require.Equal(t, specKeyA, specKeyB)

	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{
		specKeyA: specA,
	}, SchedulerOptions{})
	require.NoError(t, err)

	instA, err := s.Acquire(context.Background(), specKeyA, "")
	require.NoError(t, err)
	instB, err := s.Acquire(context.Background(), specKeyB, "")
	require.NoError(t, err)
	require.Same(t, instA, instB)
}

func TestBasicScheduler_SetDesiredMinReadyStartsInstance(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	require.NoError(t, s.SetDesiredMinReady(context.Background(), "svc", 1))

	state := s.getPool("svc", spec)
	state.mu.Lock()
	defer state.mu.Unlock()
	require.Len(t, state.instances, 1)
}

func TestBasicScheduler_StopSpecStopsInstances(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)

	// Release instance first so it's idle (BusyCount == 0)
	require.NoError(t, s.Release(context.Background(), inst))

	require.NoError(t, s.StopSpec(context.Background(), "svc", "caller inactive"))
	require.Equal(t, domain.InstanceStateStopped, inst.State)

	state := s.getPool("svc", spec)
	state.mu.Lock()
	defer state.mu.Unlock()
	require.Len(t, state.instances, 0)
}

func TestBasicScheduler_StopSpecDrainsBusyInstances(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:                "svc",
		Cmd:                 []string{"./svc"},
		MaxConcurrent:       1,
		DrainTimeoutSeconds: 1,
		ProtocolVersion:     domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{
		Logger: zap.NewNop(),
	})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.Equal(t, 1, inst.BusyCount)

	// StopSpec should mark busy instance as draining, not stopped
	require.NoError(t, s.StopSpec(context.Background(), "svc", "caller inactive"))
	require.Equal(t, domain.InstanceStateDraining, inst.State)

	state := s.getPool("svc", spec)
	state.mu.Lock()
	require.Len(t, state.instances, 0)
	require.Len(t, state.draining, 1)
	state.mu.Unlock()

	// Release triggers drain completion
	require.NoError(t, s.Release(context.Background(), inst))

	// Wait for drain goroutine to complete
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, domain.InstanceStateStopped, inst.State)

	state.mu.Lock()
	defer state.mu.Unlock()
	require.Len(t, state.draining, 0)
}

func TestBasicScheduler_StopSpecCancelsInFlightStart(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	lc := &blockingLifecycle{
		started: started,
		release: release,
	}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		_, err := s.Acquire(context.Background(), "svc", "")
		errCh <- err
	}()

	<-started
	require.NoError(t, s.StopSpec(context.Background(), "svc", "caller inactive"))
	close(release)

	require.ErrorIs(t, <-errCh, ErrNoCapacity)

	state := s.getPool("svc", spec)
	state.mu.Lock()
	defer state.mu.Unlock()
	require.Len(t, state.instances, 0)
	require.Equal(t, 1, lc.stops())
}

func TestBasicScheduler_StartDrainCompletesImmediatelyWhenIdle(t *testing.T) {
	stopCh := make(chan struct{})
	lc := &trackingLifecycle{stopCh: stopCh}
	spec := domain.ServerSpec{
		Name:                "svc",
		Cmd:                 []string{"./svc"},
		MaxConcurrent:       1,
		DrainTimeoutSeconds: 1,
		ProtocolVersion:     domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{
		Logger: zap.NewNop(),
	})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	state := s.getPool("svc", spec)
	state.mu.Lock()
	require.Len(t, state.instances, 1)
	tracked := state.instances[0]
	inst.State = domain.InstanceStateDraining
	state.instances = nil
	state.draining = append(state.draining, tracked)
	state.mu.Unlock()

	s.startDrain("svc", tracked, time.Second, "caller inactive")

	select {
	case <-stopCh:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected drain to complete before timeout")
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	require.Len(t, state.draining, 0)
	require.Equal(t, domain.InstanceStateStopped, inst.State)
}

func TestBasicScheduler_StartGateSingleflight(t *testing.T) {
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	lc := &blockingLifecycle{
		started: started,
		release: release,
	}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   3,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})
	require.NoError(t, err)

	results := make(chan *domain.Instance, 3)
	errorsCh := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			inst, err := s.Acquire(context.Background(), "svc", "")
			results <- inst
			errorsCh <- err
		}()
	}

	<-started
	close(release)

	var instances []*domain.Instance
	for i := 0; i < 3; i++ {
		require.NoError(t, <-errorsCh)
		instances = append(instances, <-results)
	}
	require.Equal(t, 1, lc.starts())
	require.Same(t, instances[0], instances[1])
	require.Same(t, instances[0], instances[2])
}

type fakeLifecycle struct {
	counter int
}

func (f *fakeLifecycle) StartInstance(ctx context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	f.counter++
	return &domain.Instance{
		ID:         spec.Name + "-inst",
		Spec:       spec,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}, nil
}

func (f *fakeLifecycle) StopInstance(ctx context.Context, instance *domain.Instance, reason string) error {
	if instance != nil {
		instance.State = domain.InstanceStateStopped
	}
	return nil
}

type fakeProbe struct {
	err error
}

func (f *fakeProbe) Ping(ctx context.Context, conn domain.Conn) error {
	return f.err
}

type blockingLifecycle struct {
	mu      sync.Mutex
	started chan struct{}
	release chan struct{}
	count   int
	stopMu  sync.Mutex
	stopped int
}

func (b *blockingLifecycle) StartInstance(ctx context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	b.mu.Lock()
	b.count++
	b.mu.Unlock()
	if b.started != nil {
		b.started <- struct{}{}
	}
	if b.release != nil {
		<-b.release
	}
	return &domain.Instance{
		ID:         spec.Name + "-inst",
		Spec:       spec,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}, nil
}

func (b *blockingLifecycle) StopInstance(ctx context.Context, instance *domain.Instance, reason string) error {
	if instance != nil {
		instance.State = domain.InstanceStateStopped
	}
	b.stopMu.Lock()
	b.stopped++
	b.stopMu.Unlock()
	return nil
}

func (b *blockingLifecycle) starts() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

func (b *blockingLifecycle) stops() int {
	b.stopMu.Lock()
	defer b.stopMu.Unlock()
	return b.stopped
}

type trackingLifecycle struct {
	stopCh   chan struct{}
	stopOnce sync.Once
}

func (t *trackingLifecycle) StartInstance(ctx context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	return &domain.Instance{
		ID:         spec.Name + "-inst",
		Spec:       spec,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}, nil
}

func (t *trackingLifecycle) StopInstance(ctx context.Context, instance *domain.Instance, reason string) error {
	if instance != nil {
		instance.State = domain.InstanceStateStopped
	}
	t.stopOnce.Do(func() {
		close(t.stopCh)
	})
	return nil
}
