package transport

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

type fakeConn struct {
	readCh  chan jsonrpc.Message
	writeCh chan jsonrpc.Message
	closed  chan struct{}
}

func newFakeConn() *fakeConn {
	return &fakeConn{
		readCh:  make(chan jsonrpc.Message, 1),
		writeCh: make(chan jsonrpc.Message, 1),
		closed:  make(chan struct{}),
	}
}

func (f *fakeConn) Read(ctx context.Context) (jsonrpc.Message, error) {
	select {
	case msg := <-f.readCh:
		return msg, nil
	case <-f.closed:
		return nil, mcp.ErrConnectionClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *fakeConn) Write(ctx context.Context, msg jsonrpc.Message) error {
	select {
	case f.writeCh <- msg:
		return nil
	case <-f.closed:
		return mcp.ErrConnectionClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakeConn) Close() error {
	select {
	case <-f.closed:
		return nil
	default:
		close(f.closed)
		return nil
	}
}

func (f *fakeConn) SessionID() string { return "" }

type samplingStub struct {
	result *domain.SamplingResult
	err    error
}

func (s *samplingStub) CreateMessage(ctx context.Context, params *domain.SamplingRequest) (*domain.SamplingResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type elicitationStub struct {
	result *domain.ElicitationResult
	err    error
}

func (e *elicitationStub) Elicit(ctx context.Context, params *domain.ElicitationRequest) (*domain.ElicitationResult, error) {
	if e.err != nil {
		return nil, e.err
	}
	return e.result, nil
}

func TestConnectionSamplingRequest(t *testing.T) {
	conn := newFakeConn()
	handler := &samplingStub{
		result: &domain.SamplingResult{
			Role: "assistant",
			Content: domain.SamplingContent{
				Type: "text",
				Text: "hello",
			},
		},
	}
	client := newClientConn(conn, clientConnOptions{
		Logger:          zap.NewNop(),
		SamplingHandler: handler,
	})
	t.Cleanup(func() { _ = client.Close() })

	params := domain.SamplingRequest{
		Messages: []domain.SamplingMessage{
			{
				Role: "user",
				Content: domain.SamplingContent{
					Type: "text",
					Text: "ping",
				},
			},
		},
	}
	rawParams, err := json.Marshal(params)
	require.NoError(t, err)

	id, err := jsonrpc.MakeID("1")
	require.NoError(t, err)
	conn.readCh <- &jsonrpc.Request{
		ID:     id,
		Method: "sampling/createMessage",
		Params: rawParams,
	}

	select {
	case msg := <-conn.writeCh:
		resp, ok := msg.(*jsonrpc.Response)
		require.True(t, ok)
		require.Nil(t, resp.Error)

		var result domain.SamplingResult
		require.NoError(t, json.Unmarshal(resp.Result, &result))
		require.Equal(t, "assistant", result.Role)
		require.Equal(t, "hello", result.Content.Text)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for sampling response")
	}
}

func TestConnectionElicitationRequest(t *testing.T) {
	conn := newFakeConn()
	handler := &elicitationStub{
		result: &domain.ElicitationResult{
			Action: "decline",
		},
	}
	client := newClientConn(conn, clientConnOptions{
		Logger:             zap.NewNop(),
		ElicitationHandler: handler,
	})
	t.Cleanup(func() { _ = client.Close() })

	params := domain.ElicitationRequest{
		Message: "Need info",
		Mode:    "form",
	}
	rawParams, err := json.Marshal(params)
	require.NoError(t, err)

	id, err := jsonrpc.MakeID("elicit-1")
	require.NoError(t, err)
	conn.readCh <- &jsonrpc.Request{
		ID:     id,
		Method: "elicitation/create",
		Params: rawParams,
	}

	select {
	case msg := <-conn.writeCh:
		resp, ok := msg.(*jsonrpc.Response)
		require.True(t, ok)
		require.Nil(t, resp.Error)

		var result domain.ElicitationResult
		require.NoError(t, json.Unmarshal(resp.Result, &result))
		require.Equal(t, "decline", result.Action)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for elicitation response")
	}
}

func TestConnectionUnsupportedMethod(t *testing.T) {
	conn := newFakeConn()
	client := newClientConn(conn, clientConnOptions{
		Logger: zap.NewNop(),
	})
	t.Cleanup(func() { _ = client.Close() })

	id, err := jsonrpc.MakeID("99")
	require.NoError(t, err)
	conn.readCh <- &jsonrpc.Request{
		ID:     id,
		Method: "tasks/get",
		Params: json.RawMessage(`{}`),
	}

	select {
	case msg := <-conn.writeCh:
		resp, ok := msg.(*jsonrpc.Response)
		require.True(t, ok)
		require.Error(t, resp.Error)
		require.Contains(t, resp.Error.Error(), "method not supported")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for unsupported method response")
	}
}
