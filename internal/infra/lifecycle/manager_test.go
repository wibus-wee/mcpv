package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

func TestManager_StartInstance_Success(t *testing.T) {
	ft := &fakeTransport{
		conn: &fakeConn{
			recvPayload: json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"srv"},"capabilities":{}}}`),
		},
		stop: func(ctx context.Context) error { return nil },
	}
	mgr := NewManager(ft, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	inst, err := mgr.StartInstance(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, domain.InstanceStateReady, inst.State)
	require.Equal(t, 0, inst.BusyCount)
	require.WithinDuration(t, time.Now(), inst.LastActive, time.Second)

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	require.NotNil(t, mgr.conns[inst.ID])
	require.NotNil(t, mgr.stops[inst.ID])
}

func TestManager_StartInstance_ProtocolMismatch(t *testing.T) {
	ft := &fakeTransport{
		conn: &fakeConn{
			recvPayload: json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2024-01-01","serverInfo":{"name":"srv"},"capabilities":{}}}`),
		},
		stop: func(ctx context.Context) error { return nil },
	}
	mgr := NewManager(ft, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: "2024-01-01",
	}

	_, err := mgr.StartInstance(context.Background(), spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported protocol version")
}

func TestManager_StartInstance_TransportError(t *testing.T) {
	ft := &fakeTransport{
		err: errors.New("boom"),
	}
	mgr := NewManager(ft, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	_, err := mgr.StartInstance(context.Background(), spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "start transport")
}

func TestManager_StartInstance_InitializeFail(t *testing.T) {
	ft := &fakeTransport{
		conn: &fakeConn{sendErr: errors.New("send fail")},
		stop: func(ctx context.Context) error { return nil },
	}
	mgr := NewManager(ft, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	_, err := mgr.StartInstance(context.Background(), spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "initialize")
}

func TestManager_StopInstance_Success(t *testing.T) {
	stopped := false
	ft := &fakeTransport{
		conn: &fakeConn{
			recvPayload: json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"srv"},"capabilities":{}}}`),
		},
		stop: func(ctx context.Context) error {
			stopped = true
			return nil
		},
	}
	mgr := NewManager(ft, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		IdleSeconds:     0,
		MinReady:        0,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	inst, err := mgr.StartInstance(context.Background(), spec)
	require.NoError(t, err)

	err = mgr.StopInstance(context.Background(), inst, "test")
	require.NoError(t, err)
	require.Equal(t, domain.InstanceStateStopped, inst.State)
	require.True(t, stopped)
}

func TestManager_StopInstance_Unknown(t *testing.T) {
	mgr := NewManager(&fakeTransport{}, zap.NewNop())
	inst := &domain.Instance{ID: "missing", Spec: domain.ServerSpec{Name: "svc"}}

	err := mgr.StopInstance(context.Background(), inst, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown instance")
}

type fakeTransport struct {
	conn domain.Conn
	stop domain.StopFn
	err  error
}

func (f *fakeTransport) Start(ctx context.Context, spec domain.ServerSpec) (domain.Conn, domain.StopFn, error) {
	return f.conn, f.stop, f.err
}

func (f *fakeConn) Send(ctx context.Context, msg json.RawMessage) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	return nil
}
func (f *fakeConn) Recv(ctx context.Context) (json.RawMessage, error) {
	if f.recvPayload != nil {
		return f.recvPayload, nil
	}
	return nil, f.recvErr
}
func (f *fakeConn) Close() error { return f.closeErr }

type fakeConn struct {
	sendErr     error
	recvErr     error
	closeErr    error
	recvPayload json.RawMessage
}

func TestManager_InitializeMissingCapabilities(t *testing.T) {
	ft := &fakeTransport{
		conn: &fakeConn{
			recvPayload: json.RawMessage(`{"jsonrpc":"2.0","id":"mcpd-init","result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"srv"}}}`),
		},
	}
	mgr := NewManager(ft, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "svc",
		Cmd:             []string{"./svc"},
		MaxConcurrent:   1,
		ProtocolVersion: domain.DefaultProtocolVersion,
	}

	_, err := mgr.StartInstance(context.Background(), spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "capabilities")
}
