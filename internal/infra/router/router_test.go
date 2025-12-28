package router

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mcpd/internal/domain"
)

func TestBasicRouter_RouteSuccess(t *testing.T) {
	respPayload := json.RawMessage(`{"ok":true}`)
	sched := &fakeScheduler{
		instance: &domain.Instance{
			ID:   "inst1",
			Conn: &fakeConn{resp: respPayload},
		},
	}
	r := NewBasicRouter(sched, RouterOptions{})

	resp, err := r.Route(context.Background(), "svc", "spec", "rk", json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	require.NoError(t, err)
	require.JSONEq(t, string(respPayload), string(resp))
	require.True(t, sched.released)
}

func TestBasicRouter_AcquireError(t *testing.T) {
	sched := &fakeScheduler{acquireErr: errors.New("busy")}
	r := NewBasicRouter(sched, RouterOptions{})

	_, err := r.Route(context.Background(), "svc", "spec", "", json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	require.Error(t, err)
}

func TestBasicRouter_NoConn(t *testing.T) {
	sched := &fakeScheduler{
		instance: &domain.Instance{ID: "x"},
	}
	r := NewBasicRouter(sched, RouterOptions{})

	_, err := r.Route(context.Background(), "svc", "spec", "", json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	require.Error(t, err)
}

func TestBasicRouter_MethodNotAllowed(t *testing.T) {
	sched := &fakeScheduler{
		instance: &domain.Instance{
			ID:   "inst1",
			Conn: &fakeConn{resp: json.RawMessage(`{}`)},
		},
	}
	r := NewBasicRouter(sched, RouterOptions{})

	_, err := r.Route(context.Background(), "svc", "spec", "", json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	require.Error(t, err)
}

type fakeScheduler struct {
	instance   *domain.Instance
	acquireErr error
	released   bool
}

func (f *fakeScheduler) Acquire(ctx context.Context, specKey, routingKey string) (*domain.Instance, error) {
	return f.instance, f.acquireErr
}

func (f *fakeScheduler) Release(ctx context.Context, instance *domain.Instance) error {
	f.released = true
	return nil
}

func (f *fakeScheduler) SetDesiredMinReady(ctx context.Context, specKey string, minReady int) error {
	return nil
}

func (f *fakeScheduler) StopSpec(ctx context.Context, specKey, reason string) error {
	return nil
}

func (f *fakeScheduler) StartIdleManager(interval time.Duration) {}
func (f *fakeScheduler) StopIdleManager()                        {}
func (f *fakeScheduler) StartPingManager(interval time.Duration) {}
func (f *fakeScheduler) StopPingManager()                        {}
func (f *fakeScheduler) StopAll(ctx context.Context)             {}

func (f *fakeScheduler) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	return nil, nil
}

type fakeConn struct {
	req  json.RawMessage
	resp json.RawMessage
	err  error
}

func (f *fakeConn) Send(ctx context.Context, msg json.RawMessage) error {
	f.req = msg
	return f.err
}

func (f *fakeConn) Recv(ctx context.Context) (json.RawMessage, error) {
	return f.resp, f.err
}

func (f *fakeConn) Close() error { return nil }
