package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

func TestManager_StartInstance_Success(t *testing.T) {
	streams, stop := newTestStreams()
	launcher := &fakeLauncher{streams: streams, stop: stop}
	transport := &fakeTransport{
		conn: &fakeConn{
			resp: json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"srv"},"capabilities":{}}}`),
		},
	}
	mgr := NewManager(context.Background(), launcher, transport, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	inst, err := mgr.StartInstance(context.Background(), "spec-key", spec)
	require.NoError(t, err)
	require.Equal(t, domain.InstanceStateReady, inst.State())
	require.Equal(t, 0, inst.BusyCount())
	require.WithinDuration(t, time.Now(), inst.LastActive(), time.Second)

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	require.NotNil(t, mgr.conns[inst.ID()])
	require.NotNil(t, mgr.stops[inst.ID()])
}

func TestManager_StartInstance_DetachesFromCallerContext(t *testing.T) {
	streams, stop := newTestStreams()
	launcher := &fakeLauncher{streams: streams, stop: stop}
	transport := &fakeTransport{
		conn: &fakeConn{
			resp: json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"srv"},"capabilities":{}}}`),
		},
	}
	mgr := NewManager(context.Background(), launcher, transport, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	ctx, cancel := context.WithCancel(context.Background())
	inst, err := mgr.StartInstance(ctx, "spec-key", spec)
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.NotNil(t, launcher.startCtx)

	cancel()

	select {
	case <-launcher.startCtx.Done():
		t.Fatal("start context should not be canceled when caller context is canceled after startup")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestManager_StartInstance_ProtocolMismatch(t *testing.T) {
	streams, stop := newTestStreams()
	launcher := &fakeLauncher{streams: streams, stop: stop}
	transport := &fakeTransport{
		conn: &fakeConn{
			resp: json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2024-01-01","serverInfo":{"name":"srv"},"capabilities":{}}}`),
		},
	}
	mgr := NewManager(context.Background(), launcher, transport, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: "2024-01-01",
	}

	_, err := mgr.StartInstance(context.Background(), "spec-key", spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported protocol version")
}

func TestManager_StartInstance_LauncherError(t *testing.T) {
	streams, stop := newTestStreams()
	launcher := &fakeLauncher{streams: streams, stop: stop, err: errors.New("boom")}
	mgr := NewManager(context.Background(), launcher, &fakeTransport{}, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	_, err := mgr.StartInstance(context.Background(), "spec-key", spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "start launcher")
}

func TestManager_StartInstance_InitializeFail(t *testing.T) {
	streams, stop := newTestStreams()
	launcher := &fakeLauncher{streams: streams, stop: stop}
	transport := &fakeTransport{
		conn: &fakeConn{callErr: errors.New("call fail")},
	}
	mgr := NewManager(context.Background(), launcher, transport, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	_, err := mgr.StartInstance(context.Background(), "spec-key", spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "initialize")
}

func TestManager_StartInstance_InitializeRetry(t *testing.T) {
	streams, stop := newTestStreams()
	launcher := &fakeLauncher{streams: streams, stop: stop}
	conn := &retryConn{}
	transport := &fakeTransport{conn: conn}
	mgr := NewManager(context.Background(), launcher, transport, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	inst, err := mgr.StartInstance(context.Background(), "spec-key", spec)
	require.NoError(t, err)
	require.NotNil(t, inst)
	require.Equal(t, initializeRetryCount+1, conn.callCount)
}

func TestManager_StopInstance_Success(t *testing.T) {
	var stopped atomic.Bool
	streams, stop := newTestStreamsWithStop(func(ctx context.Context) error {
		stopped.Store(true)
		return nil
	})
	launcher := &fakeLauncher{streams: streams, stop: stop}
	transport := &fakeTransport{
		conn: &fakeConn{
			resp: json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"srv"},"capabilities":{}}}`),
		},
	}
	mgr := NewManager(context.Background(), launcher, transport, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	inst, err := mgr.StartInstance(context.Background(), "spec-key", spec)
	require.NoError(t, err)

	err = mgr.StopInstance(context.Background(), inst, "test")
	require.NoError(t, err)
	require.Equal(t, domain.InstanceStateStopped, inst.State())
	require.True(t, stopped.Load())
}

func TestManager_StopInstance_Unknown(t *testing.T) {
	streams, stop := newTestStreams()
	launcher := &fakeLauncher{streams: streams, stop: stop}
	mgr := NewManager(context.Background(), launcher, &fakeTransport{}, zap.NewNop())
	inst := domain.NewInstance(domain.InstanceOptions{
		ID:   "missing",
		Spec: domain.ServerSpec{Name: "svc"},
	})

	err := mgr.StopInstance(context.Background(), inst, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown instance")
}

func TestManager_InitializeMissingCapabilities(t *testing.T) {
	streams, stop := newTestStreams()
	launcher := &fakeLauncher{streams: streams, stop: stop}
	transport := &fakeTransport{
		conn: &fakeConn{
			resp: json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"srv"}}}`),
		},
	}
	mgr := NewManager(context.Background(), launcher, transport, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	_, err := mgr.StartInstance(context.Background(), "spec-key", spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "capabilities")
}

type fakeLauncher struct {
	streams  domain.IOStreams
	stop     domain.StopFn
	err      error
	startCtx context.Context
}

func (f *fakeLauncher) Start(ctx context.Context, specKey string, spec domain.ServerSpec) (domain.IOStreams, domain.StopFn, error) {
	f.startCtx = ctx
	return f.streams, f.stop, f.err
}

type fakeTransport struct {
	conn       domain.Conn
	err        error
	connectCtx context.Context
}

func (f *fakeTransport) Connect(ctx context.Context, specKey string, spec domain.ServerSpec, streams domain.IOStreams) (domain.Conn, error) {
	f.connectCtx = ctx
	return f.conn, f.err
}

type fakeConn struct {
	callErr  error
	resp     json.RawMessage
	closeErr error
}

func (f *fakeConn) Call(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	if f.callErr != nil {
		return nil, f.callErr
	}
	return f.resp, nil
}

func (f *fakeConn) Close() error { return f.closeErr }

type retryConn struct {
	callCount int
}

func (r *retryConn) Call(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	r.callCount++
	if r.callCount <= initializeRetryCount {
		return nil, errors.New("call fail")
	}
	return json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"srv"},"capabilities":{}}}`), nil
}

func (r *retryConn) Close() error { return nil }

func newTestStreams() (domain.IOStreams, domain.StopFn) {
	return newTestStreamsWithStop(func(ctx context.Context) error { return nil })
}

func newTestStreamsWithStop(stopFn domain.StopFn) (domain.IOStreams, domain.StopFn) {
	reader, writer := io.Pipe()
	streams := domain.IOStreams{Reader: reader, Writer: writer}
	stop := func(ctx context.Context) error {
		_ = reader.Close()
		_ = writer.Close()
		if stopFn != nil {
			return stopFn(ctx)
		}
		return nil
	}
	return streams, stop
}
