package scheduler

import (
	"context"
	"errors"
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
	s := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})

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
		Sticky:          true,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})

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
	s := NewBasicScheduler(&fakeLifecycle{}, map[string]domain.ServerSpec{}, SchedulerOptions{})
	_, err := s.Acquire(context.Background(), "missing", "")
	require.ErrorIs(t, err, ErrUnknownServerType)
}

func TestBasicScheduler_IdleReapRespectsMinReadyAndPersistent(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        1,
		Persistent:      false,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.NoError(t, s.Release(context.Background(), inst))

	s.reapIdle()
	require.Equal(t, domain.InstanceStateReady, inst.State)

	spec.MinReady = 0
	s.specs["svc"] = spec
	s.reapIdle()
	require.Equal(t, domain.InstanceStateStopped, inst.State)
}

func TestBasicScheduler_StickySkipIdle(t *testing.T) {
	lc := &fakeLifecycle{}
	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		Sticky:          true,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}
	s := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{})

	inst, err := s.Acquire(context.Background(), "svc", "rk")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	s.reapIdle()
	require.Equal(t, domain.InstanceStateReady, inst.State)
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
	s := NewBasicScheduler(lc, map[string]domain.ServerSpec{"svc": spec}, SchedulerOptions{
		Probe:  &fakeProbe{err: errors.New("ping failed")},
		Logger: zap.NewNop(),
	})

	inst, err := s.Acquire(context.Background(), "svc", "")
	require.NoError(t, err)
	require.NoError(t, s.Release(context.Background(), inst))

	s.probeInstances()

	require.Equal(t, domain.InstanceStateStopped, inst.State)
	state := s.getServerState("svc")
	require.Len(t, state.instances, 0)
}

type fakeLifecycle struct {
	counter int
}

func (f *fakeLifecycle) StartInstance(ctx context.Context, spec domain.ServerSpec) (*domain.Instance, error) {
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
