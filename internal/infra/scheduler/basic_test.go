package scheduler

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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	inst1, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.Equal(t, 1, inst1.BusyCount())

	inst2, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.Same(t, inst1, inst2)
	require.Equal(t, 2, inst1.BusyCount())

	require.NoError(t, s.Release(context.Background(), inst1))
	require.Equal(t, domain.InstanceStateBusy, inst1.State())
	require.NoError(t, s.Release(context.Background(), inst1))
	require.Equal(t, domain.InstanceStateReady, inst1.State())
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
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
	s, err := NewBasicScheduler(&fakeLifecycle{}, map[string]domain.ServerSpec{}, Options{})
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	_, err = s.AcquireReady(context.Background(), "svc", "")
	require.ErrorIs(t, err, domain.ErrNoReadyInstance)
	require.Equal(t, 0, lc.counter)
}

func TestBasicScheduler_StatelessSelection(t *testing.T) {
	t.Run("round_robin_cycle", func(t *testing.T) {
		lc := &countingLifecycle{}
		spec := domain.ServerSpec{
			Name:            "svc",
			Cmd:             []string{"./svc"},
			MaxConcurrent:   2,
			Strategy:        domain.StrategyStateless,
			ProtocolVersion: domain.DefaultProtocolVersion,
		}
		s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
		require.NoError(t, err)
		require.NoError(t, s.SetDesiredMinReady(context.Background(), "svc", 3))

		state := s.getPool("svc", spec)
		state.mu.Lock()
		require.Len(t, state.instances, 3)
		instA := state.instances[0].instance
		instB := state.instances[1].instance
		instC := state.instances[2].instance
		state.mu.Unlock()

		cases := []struct {
			name string
			want *domain.Instance
		}{
			{name: "first", want: instA},
			{name: "second", want: instB},
			{name: "third", want: instC},
			{name: "wrap", want: instA},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				selected, err := s.AcquireReady(context.Background(), "svc", "")
				require.NoError(t, err)
				require.Equal(t, tc.want.ID(), selected.ID())
				require.NoError(t, s.Release(context.Background(), selected))
			})
		}
	})

	t.Run("least_loaded_prefers_lower_busy", func(t *testing.T) {
		lc := &countingLifecycle{}
		spec := domain.ServerSpec{
			Name:            "svc",
			Cmd:             []string{"./svc"},
			MaxConcurrent:   10,
			Strategy:        domain.StrategyStateless,
			ProtocolVersion: domain.DefaultProtocolVersion,
		}
		s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
		require.NoError(t, err)
		require.NoError(t, s.SetDesiredMinReady(context.Background(), "svc", 3))

		state := s.getPool("svc", spec)
		state.mu.Lock()
		require.Len(t, state.instances, 3)
		instA := state.instances[0].instance
		instB := state.instances[1].instance
		instC := state.instances[2].instance

		instA.SetBusyCount(2)
		instA.SetState(domain.InstanceStateBusy)
		instB.SetBusyCount(1)
		instB.SetState(domain.InstanceStateBusy)
		instC.SetBusyCount(0)
		instC.SetState(domain.InstanceStateReady)
		state.mu.Unlock()

		selected, err := s.AcquireReady(context.Background(), "svc", "")
		require.NoError(t, err)
		require.Equal(t, instC.ID(), selected.ID())
	})

	t.Run("skip_unroutable_and_full", func(t *testing.T) {
		lc := &countingLifecycle{}
		spec := domain.ServerSpec{
			Name:            "svc",
			Cmd:             []string{"./svc"},
			MaxConcurrent:   2,
			Strategy:        domain.StrategyStateless,
			ProtocolVersion: domain.DefaultProtocolVersion,
		}
		s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
		require.NoError(t, err)
		require.NoError(t, s.SetDesiredMinReady(context.Background(), "svc", 3))

		state := s.getPool("svc", spec)
		state.mu.Lock()
		require.Len(t, state.instances, 3)
		instA := state.instances[0].instance
		instB := state.instances[1].instance
		instC := state.instances[2].instance

		instA.SetState(domain.InstanceStateFailed)
		instB.SetBusyCount(spec.MaxConcurrent)
		instB.SetState(domain.InstanceStateBusy)
		instC.SetBusyCount(0)
		instC.SetState(domain.InstanceStateReady)
		state.mu.Unlock()

		selected, err := s.AcquireReady(context.Background(), "svc", "")
		require.NoError(t, err)
		require.Equal(t, instC.ID(), selected.ID())
	})
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)
	require.NoError(t, s.SetDesiredMinReady(context.Background(), "svc", 1))

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.NoError(t, s.Release(context.Background(), inst))

	s.reapIdle()
	require.Equal(t, domain.InstanceStateReady, inst.State())
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	s.reapIdle()
	require.Equal(t, domain.InstanceStateStopped, inst.State())
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "rk")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	// Instance should not be reaped because it has an active binding
	s.reapIdle()
	require.Equal(t, domain.InstanceStateReady, inst.State())
}

func TestBasicScheduler_IdleReapIgnoresIdleSecondsWhenMinReadyZero(t *testing.T) {
	// When minReady=0, instances should be reaped immediately regardless of IdleSeconds.
	// This is important for on-demand servers after bootstrap completes.
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     3600, // 1 hour, normally would not be reaped
		MinReady:        0,
		Strategy:        domain.StrategyStateless,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	// Even with IdleSeconds=3600, instance should be reaped because minReady=0
	s.reapIdle()
	require.Equal(t, domain.InstanceStateStopped, inst.State())
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{
		Logger: zap.NewNop(),
	})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "rk")
	require.NoError(t, err)
	require.Equal(t, "rk", inst.StickyKey())
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
	require.Equal(t, "", inst.StickyKey())
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
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
	require.Equal(t, "rk", inst.StickyKey())
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
			s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
			require.NoError(t, err)

			inst, err := s.Acquire(context.Background(), "svc", "")
			require.NoError(t, err)
			require.NoError(t, s.Release(context.Background(), inst))

			s.reapIdle()
			require.Equal(t, domain.InstanceStateReady, inst.State())

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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{
		Probe:  &fakeProbe{err: errors.New("ping failed")},
		Logger: zap.NewNop(),
	})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	s.probeInstances()

	require.Equal(t, domain.InstanceStateStopped, inst.State())
	specKey := domain.SpecFingerprint(spec)
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

	specKeyA := domain.SpecFingerprint(specA)
	specKeyB := domain.SpecFingerprint(specB)
	require.Equal(t, specKeyA, specKeyB)

	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{
		specKeyA: specA,
	}, Options{})
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)

	// Release instance first so it's idle (BusyCount == 0)
	require.NoError(t, s.Release(context.Background(), inst))

	require.NoError(t, s.StopSpec(context.Background(), "svc", "caller inactive"))
	require.Equal(t, domain.InstanceStateStopped, inst.State())

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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{
		Logger: zap.NewNop(),
	})
	require.NoError(t, err)

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.Equal(t, 1, inst.BusyCount())

	// StopSpec should mark busy instance as draining, not stopped
	require.NoError(t, s.StopSpec(context.Background(), "svc", "caller inactive"))
	require.Equal(t, domain.InstanceStateDraining, inst.State())

	state := s.getPool("svc", spec)
	state.mu.Lock()
	require.Len(t, state.instances, 0)
	require.Len(t, state.draining, 1)
	state.mu.Unlock()

	// Release triggers drain completion
	require.NoError(t, s.Release(context.Background(), inst))
	require.Eventually(t, func() bool {
		state.mu.Lock()
		defer state.mu.Unlock()
		return inst.State() == domain.InstanceStateStopped && len(state.draining) == 0
	}, 2*time.Second, 10*time.Millisecond)
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{
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
	inst.SetState(domain.InstanceStateDraining)
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
	require.Equal(t, domain.InstanceStateStopped, inst.State())
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
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

func TestBasicScheduler_SingletonBusyWaitsInsteadOfStarting(t *testing.T) {
	started := make(chan struct{}, 2)
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
		Strategy:        domain.StrategySingleton,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	instACh := make(chan *domain.Instance, 1)
	errACh := make(chan error, 1)
	go func() {
		inst, err := s.Acquire(context.Background(), "svc", "")
		instACh <- inst
		errACh <- err
	}()

	<-started
	close(release)

	require.NoError(t, <-errACh)
	instA := <-instACh

	ctxB, cancelB := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelB()
	instBCh := make(chan *domain.Instance, 1)
	errBCh := make(chan error, 1)
	go func() {
		inst, err := s.Acquire(ctxB, "svc", "")
		instBCh <- inst
		errBCh <- err
	}()

	select {
	case <-started:
		t.Fatal("unexpected start while singleton is busy")
	case <-time.After(50 * time.Millisecond):
	}

	require.NoError(t, s.Release(context.Background(), instA))
	require.NoError(t, <-errBCh)
	instB := <-instBCh
	require.Same(t, instA, instB)
	require.Equal(t, 1, lc.starts())
}

func TestBasicScheduler_WaitersWakeAfterStart(t *testing.T) {
	started := make(chan struct{}, 1)
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
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
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
	time.Sleep(20 * time.Millisecond)
	close(release)

	for i := 0; i < 3; i++ {
		require.NoError(t, <-errorsCh)
		require.NotNil(t, <-results)
	}
	require.Equal(t, 1, lc.starts())
}

func TestBasicScheduler_WaitersWakeAfterStartFailure(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	lc := &failOnceBlockingLifecycle{
		started: started,
		release: release,
	}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   3,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	results := make(chan *domain.Instance, 3)
	errorsCh := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			inst, err := s.Acquire(ctx, "svc", "")
			results <- inst
			errorsCh <- err
		}()
	}

	<-started
	close(release)

	successes := 0
	for i := 0; i < 3; i++ {
		err := <-errorsCh
		inst := <-results
		if err != nil {
			require.Nil(t, inst)
			continue
		}
		require.NotNil(t, inst)
		successes++
	}
	require.Equal(t, 2, lc.starts())
	require.Equal(t, 2, successes)
}

func TestBasicScheduler_Acquire_DetachesStartFromCallerCancel(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	lc := &ctxBlockingLifecycle{
		started: started,
		release: release,
	}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan *domain.Instance, 1)
	errorsCh := make(chan error, 1)
	go func() {
		inst, err := s.Acquire(ctx, "svc", "")
		results <- inst
		errorsCh <- err
	}()

	<-started
	cancel()

	select {
	case <-lc.startCtx.Done():
		t.Fatal("start context should not be canceled when caller context is canceled")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-errorsCh)
	require.NotNil(t, <-results)
}

func TestBasicScheduler_SetDesiredMinReady_PanicDoesNotLeakStarting(t *testing.T) {
	lc := &panicLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	func() {
		defer func() {
			require.NotNil(t, recover())
		}()
		_ = s.SetDesiredMinReady(context.Background(), "svc", 1)
	}()

	require.NoError(t, s.SetDesiredMinReady(context.Background(), "svc", 1))
	require.Equal(t, 2, lc.starts())

	inst, err := s.AcquireReady(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NotNil(t, inst)
}

type fakeLifecycle struct {
	counter int
}

func (f *fakeLifecycle) StartInstance(_ context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	f.counter++
	return domain.NewInstance(domain.InstanceOptions{
		ID:         spec.Name + "-inst",
		Spec:       spec,
		SpecKey:    specKey,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}), nil
}

func (f *fakeLifecycle) StopInstance(_ context.Context, instance *domain.Instance, _ string) error {
	if instance != nil {
		instance.SetState(domain.InstanceStateStopped)
	}
	return nil
}

type countingLifecycle struct {
	mu    sync.Mutex
	count int
}

func (c *countingLifecycle) StartInstance(_ context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	c.mu.Lock()
	c.count++
	id := fmt.Sprintf("%s-%d", spec.Name, c.count)
	c.mu.Unlock()
	return domain.NewInstance(domain.InstanceOptions{
		ID:         id,
		Spec:       spec,
		SpecKey:    specKey,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}), nil
}

func (c *countingLifecycle) StopInstance(_ context.Context, instance *domain.Instance, _ string) error {
	if instance != nil {
		instance.SetState(domain.InstanceStateStopped)
	}
	return nil
}

type fakeProbe struct {
	err error
}

func (f *fakeProbe) Ping(_ context.Context, _ domain.Conn) error {
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

func (b *blockingLifecycle) StartInstance(_ context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	b.mu.Lock()
	b.count++
	b.mu.Unlock()
	if b.started != nil {
		b.started <- struct{}{}
	}
	if b.release != nil {
		<-b.release
	}
	return domain.NewInstance(domain.InstanceOptions{
		ID:         spec.Name + "-inst",
		Spec:       spec,
		SpecKey:    specKey,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}), nil
}

func (b *blockingLifecycle) StopInstance(_ context.Context, instance *domain.Instance, _ string) error {
	if instance != nil {
		instance.SetState(domain.InstanceStateStopped)
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

type ctxBlockingLifecycle struct {
	started  chan struct{}
	release  chan struct{}
	startCtx context.Context
}

func (c *ctxBlockingLifecycle) StartInstance(ctx context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	c.startCtx = ctx
	if c.started != nil {
		c.started <- struct{}{}
	}
	if c.release != nil {
		<-c.release
	}
	return domain.NewInstance(domain.InstanceOptions{
		ID:         spec.Name + "-inst",
		Spec:       spec,
		SpecKey:    specKey,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}), nil
}

func (c *ctxBlockingLifecycle) StopInstance(_ context.Context, instance *domain.Instance, _ string) error {
	if instance != nil {
		instance.SetState(domain.InstanceStateStopped)
	}
	return nil
}

type failOnceBlockingLifecycle struct {
	mu      sync.Mutex
	count   int
	started chan struct{}
	release chan struct{}
}

func (f *failOnceBlockingLifecycle) StartInstance(_ context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	f.mu.Lock()
	f.count++
	count := f.count
	f.mu.Unlock()

	if f.started != nil {
		f.started <- struct{}{}
	}
	if f.release != nil {
		<-f.release
	}

	if count == 1 {
		return nil, errors.New("start failed")
	}

	return domain.NewInstance(domain.InstanceOptions{
		ID:         spec.Name + "-inst",
		Spec:       spec,
		SpecKey:    specKey,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}), nil
}

func (f *failOnceBlockingLifecycle) StopInstance(_ context.Context, instance *domain.Instance, _ string) error {
	if instance != nil {
		instance.SetState(domain.InstanceStateStopped)
	}
	return nil
}

func (f *failOnceBlockingLifecycle) starts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count
}

type panicLifecycle struct {
	mu    sync.Mutex
	count int
}

func (p *panicLifecycle) StartInstance(_ context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	p.mu.Lock()
	p.count++
	count := p.count
	p.mu.Unlock()
	if count == 1 {
		panic("boom")
	}
	return domain.NewInstance(domain.InstanceOptions{
		ID:         spec.Name + "-inst",
		Spec:       spec,
		SpecKey:    specKey,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}), nil
}

func (p *panicLifecycle) StopInstance(_ context.Context, instance *domain.Instance, _ string) error {
	if instance != nil {
		instance.SetState(domain.InstanceStateStopped)
	}
	return nil
}

func (p *panicLifecycle) starts() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count
}

type trackingLifecycle struct {
	stopCh   chan struct{}
	stopOnce sync.Once
}

func (t *trackingLifecycle) StartInstance(_ context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	return domain.NewInstance(domain.InstanceOptions{
		ID:         spec.Name + "-inst",
		Spec:       spec,
		SpecKey:    specKey,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}), nil
}

func (t *trackingLifecycle) StopInstance(_ context.Context, instance *domain.Instance, _ string) error {
	if instance != nil {
		instance.SetState(domain.InstanceStateStopped)
	}
	t.stopOnce.Do(func() {
		close(t.stopCh)
	})
	return nil
}

// slowStopLifecycle simulates a slow process exit for testing stop performance.
type slowStopLifecycle struct {
	stopDelay time.Duration
	mu        sync.Mutex
	stopCount int
}

func (s *slowStopLifecycle) StartInstance(_ context.Context, specKey string, spec domain.ServerSpec) (*domain.Instance, error) {
	return domain.NewInstance(domain.InstanceOptions{
		ID:         fmt.Sprintf("%s-inst-%d", spec.Name, time.Now().UnixNano()),
		Spec:       spec,
		SpecKey:    specKey,
		State:      domain.InstanceStateReady,
		LastActive: time.Now(),
	}), nil
}

func (s *slowStopLifecycle) StopInstance(_ context.Context, instance *domain.Instance, _ string) error {
	time.Sleep(s.stopDelay)
	s.mu.Lock()
	s.stopCount++
	s.mu.Unlock()
	if instance != nil {
		instance.SetState(domain.InstanceStateStopped)
	}
	return nil
}

func (s *slowStopLifecycle) stops() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopCount
}

func TestStopSpec_SerialStopPerformance(t *testing.T) {
	// This test documents the current behavior: StopSpec stops idle instances serially.
	// With N instances each taking D time to stop, total time is N*D.
	// Ideally this should be closer to D (parallel) rather than N*D (serial).

	const (
		instanceCount = 5
		stopDelay     = 50 * time.Millisecond
	)

	lc := &slowStopLifecycle{stopDelay: stopDelay}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1, // Force new instance per acquire when busy
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s, err := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, Options{})
	require.NoError(t, err)

	// Start multiple instances by keeping them busy
	instances := make([]*domain.Instance, instanceCount)
	for i := 0; i < instanceCount; i++ {
		inst, err := s.Acquire(context.Background(), "svc", "")
		require.NoError(t, err)
		instances[i] = inst
	}

	// Verify we have multiple instances
	state := s.getPool("svc", spec)
	state.mu.Lock()
	actualInstances := len(state.instances)
	state.mu.Unlock()
	require.Equal(t, instanceCount, actualInstances, "should have created %d separate instances", instanceCount)

	// Release all instances so they become idle
	for _, inst := range instances {
		require.NoError(t, s.Release(context.Background(), inst))
	}

	// Measure StopSpec duration
	start := time.Now()
	require.NoError(t, s.StopSpec(context.Background(), "svc", "test"))
	elapsed := time.Since(start)

	require.Equal(t, instanceCount, lc.stops())

	// Current behavior: serial stop takes ~N*D
	// This assertion documents the current (suboptimal) behavior.
	// If parallelized, elapsed should be closer to stopDelay.
	serialThreshold := time.Duration(instanceCount-1) * stopDelay
	if elapsed < serialThreshold {
		t.Logf("StopSpec completed in %v (parallel behavior achieved)", elapsed)
	} else {
		t.Logf("StopSpec completed in %v (serial behavior, expected ~%v)", elapsed, time.Duration(instanceCount)*stopDelay)
	}
}

func TestApplyCatalogDiff_PoolsMapNotCleaned(t *testing.T) {
	// This test documents the current behavior: ApplyCatalogDiff removes instances
	// but does NOT remove the poolState from the pools map.
	// This causes memory to grow unbounded when specs are frequently added/removed.

	lc := &fakeLifecycle{}
	initialSpecs := map[string]domain.ServerSpec{
		"svc-a": {Name: "svc-a", Cmd: []string{"./a"}, MaxConcurrent: 1, ProtocolVersion: domain.DefaultProtocolVersion},
		"svc-b": {Name: "svc-b", Cmd: []string{"./b"}, MaxConcurrent: 1, ProtocolVersion: domain.DefaultProtocolVersion},
	}
	s, err := NewBasicScheduler(lc, initialSpecs, Options{})
	require.NoError(t, err)

	// Acquire instances to create pool states
	instA, err := s.Acquire(context.Background(), "svc-a", "")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), instA))

	instB, err := s.Acquire(context.Background(), "svc-b", "")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), instB))

	// Verify pools exist
	s.poolsMu.RLock()
	poolCountBefore := len(s.pools)
	s.poolsMu.RUnlock()
	require.Equal(t, 2, poolCountBefore)

	// Apply diff that removes svc-a
	diff := domain.CatalogDiff{
		RemovedSpecKeys: []string{"svc-a"},
	}
	newRegistry := map[string]domain.ServerSpec{
		"svc-b": initialSpecs["svc-b"],
	}
	require.NoError(t, s.ApplyCatalogDiff(context.Background(), diff, newRegistry))

	// Verify instance was stopped
	require.Equal(t, domain.InstanceStateStopped, instA.State())

	// Check pools map - this documents the current (leaky) behavior
	s.poolsMu.RLock()
	poolCountAfter := len(s.pools)
	_, svcAPoolExists := s.pools["svc-a"]
	s.poolsMu.RUnlock()

	// Current behavior: pool still exists even after spec removed
	// This is a memory leak - poolState for "svc-a" is never cleaned up.
	if svcAPoolExists {
		t.Logf("pools map still contains removed spec 'svc-a' (memory leak)")
		require.Equal(t, poolCountBefore, poolCountAfter, "pool count unchanged - poolState not cleaned")
	} else {
		t.Logf("pools map correctly cleaned up removed spec 'svc-a'")
		require.Equal(t, poolCountBefore-1, poolCountAfter)
	}
}

func TestApplyCatalogDiff_ContextTimeoutLeavesPartialState(t *testing.T) {
	// This test documents the risk: if context times out during ApplyCatalogDiff,
	// some specs may not be stopped, leading to orphaned processes.

	const (
		stopDelay = 100 * time.Millisecond
	)

	lc := &slowStopLifecycle{stopDelay: stopDelay}
	specs := map[string]domain.ServerSpec{
		"svc-a": {Name: "svc-a", Cmd: []string{"./a"}, MaxConcurrent: 1, ProtocolVersion: domain.DefaultProtocolVersion},
		"svc-b": {Name: "svc-b", Cmd: []string{"./b"}, MaxConcurrent: 1, ProtocolVersion: domain.DefaultProtocolVersion},
		"svc-c": {Name: "svc-c", Cmd: []string{"./c"}, MaxConcurrent: 1, ProtocolVersion: domain.DefaultProtocolVersion},
	}
	s, err := NewBasicScheduler(lc, specs, Options{})
	require.NoError(t, err)

	// Create instances for all specs
	instances := make(map[string]*domain.Instance)
	for key := range specs {
		inst, err := s.Acquire(context.Background(), key, "")
		require.NoError(t, err)
		require.NoError(t, s.Release(context.Background(), inst))
		instances[key] = inst
	}

	// Apply diff with a very short timeout (less than time to stop all)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	diff := domain.CatalogDiff{
		RemovedSpecKeys: []string{"svc-a", "svc-b", "svc-c"},
	}
	err = s.ApplyCatalogDiff(ctx, diff, map[string]domain.ServerSpec{})

	// Count how many were actually stopped
	stoppedCount := 0
	for _, inst := range instances {
		if inst.State() == domain.InstanceStateStopped {
			stoppedCount++
		}
	}

	// Document behavior: with serial stops and short timeout,
	// not all instances may be stopped
	t.Logf("stopped %d/%d instances before timeout/completion", stoppedCount, len(instances))
	t.Logf("ApplyCatalogDiff returned: %v", err)
}
