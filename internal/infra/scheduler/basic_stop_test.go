package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

func TestBasicScheduler_StopSpecStopsInstances(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := newTestSpec("svc")

	s := newScheduler(t, lc, map[string]domain.ServerSpec{"svc": spec}, Options{})

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)

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
	spec := newTestSpec("svc")
	spec.DrainTimeoutSeconds = 1

	s := newScheduler(t, lc, map[string]domain.ServerSpec{"svc": spec}, Options{
		Logger: zap.NewNop(),
	})

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.Equal(t, 1, inst.BusyCount())

	require.NoError(t, s.StopSpec(context.Background(), "svc", "caller inactive"))
	require.Equal(t, domain.InstanceStateDraining, inst.State())

	state := s.getPool("svc", spec)
	state.mu.Lock()
	require.Len(t, state.instances, 0)
	require.Len(t, state.draining, 1)
	state.mu.Unlock()

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
	spec := newTestSpec("svc")

	s := newScheduler(t, lc, map[string]domain.ServerSpec{"svc": spec}, Options{})

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
	spec := newTestSpec("svc")
	spec.DrainTimeoutSeconds = 1

	s := newScheduler(t, lc, map[string]domain.ServerSpec{"svc": spec}, Options{
		Logger: zap.NewNop(),
	})

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

func TestBasicScheduler_ReleaseClosesDrainDone(t *testing.T) {
	lc := &blockingLifecycle{}
	spec := newTestSpec("svc")
	spec.DrainTimeoutSeconds = 1

	s := newScheduler(t, lc, map[string]domain.ServerSpec{"svc": spec}, Options{})

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)

	require.NoError(t, s.StopSpec(context.Background(), "svc", "drain"))

	state := s.getPool("svc", spec)
	state.mu.Lock()
	require.Len(t, state.draining, 1)
	tracked := state.draining[0]
	drainDone := tracked.drainDone
	state.mu.Unlock()

	require.NotNil(t, drainDone)
	require.NoError(t, s.Release(context.Background(), inst))

	select {
	case <-drainDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected drainDone to be closed after release")
	}

	require.Eventually(t, func() bool {
		return lc.stops() == 1
	}, 500*time.Millisecond, 20*time.Millisecond)
}

func TestTrackedInstance_CloseDrainDoneConcurrent(t *testing.T) {
	inst := &trackedInstance{
		drainDone: make(chan struct{}),
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		inst.closeDrainDone()
	}()
	go func() {
		defer wg.Done()
		inst.closeDrainDone()
	}()
	wg.Wait()

	select {
	case <-inst.drainDone:
	default:
		t.Fatal("expected drainDone to be closed")
	}
}

func TestStopSpec_SerialStopPerformance(t *testing.T) {
	const (
		instanceCount = 5
		stopDelay     = 50 * time.Millisecond
	)

	lc := &slowStopLifecycle{stopDelay: stopDelay}
	spec := newTestSpec("svc")
	spec.MaxConcurrent = 1

	s := newScheduler(t, lc, map[string]domain.ServerSpec{"svc": spec}, Options{})

	instances := make([]*domain.Instance, instanceCount)
	for i := 0; i < instanceCount; i++ {
		inst, err := s.Acquire(context.Background(), "svc", "")
		require.NoError(t, err)
		instances[i] = inst
	}

	state := s.getPool("svc", spec)
	state.mu.Lock()
	actualInstances := len(state.instances)
	state.mu.Unlock()
	require.Equal(t, instanceCount, actualInstances, "should have created %d separate instances", instanceCount)

	for _, inst := range instances {
		require.NoError(t, s.Release(context.Background(), inst))
	}

	start := time.Now()
	require.NoError(t, s.StopSpec(context.Background(), "svc", "test"))
	elapsed := time.Since(start)

	require.Equal(t, instanceCount, lc.stops())

	serialThreshold := time.Duration(instanceCount-1) * stopDelay
	if elapsed < serialThreshold {
		t.Logf("StopSpec completed in %v (parallel behavior achieved)", elapsed)
	} else {
		t.Logf("StopSpec completed in %v (serial behavior, expected ~%v)", elapsed, time.Duration(instanceCount)*stopDelay)
	}
}
