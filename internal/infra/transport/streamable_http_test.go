package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"mcpd/internal/domain"
)

func TestStreamableHTTPTransport_ConnectAndPing(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "remote",
		Version: "0.1.0",
	}, nil)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})

	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	transport := NewStreamableHTTPTransport(StreamableHTTPTransportOptions{})
	spec := domain.ServerSpec{
		Name:            "remote",
		Transport:       domain.TransportStreamableHTTP,
		ProtocolVersion: domain.DefaultStreamableHTTPProtocolVersion,
		HTTP: &domain.StreamableHTTPConfig{
			Endpoint:   httpServer.URL,
			MaxRetries: 1,
		},
	}

	conn, err := transport.Connect(context.Background(), "spec-remote", spec, domain.IOStreams{})
	require.NoError(t, err)
	defer conn.Close()

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`)
	resp, err := conn.Call(context.Background(), msg)
	require.NoError(t, err)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":1,"result":{}}`, string(resp))
}

func TestStreamableHTTPTransport_MissingConfig(t *testing.T) {
	transport := NewStreamableHTTPTransport(StreamableHTTPTransportOptions{})
	spec := domain.ServerSpec{
		Name:      "remote",
		Transport: domain.TransportStreamableHTTP,
	}

	_, err := transport.Connect(context.Background(), "spec-remote", spec, domain.IOStreams{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "remote")
}

func TestStreamableHTTPTransport_EmptyEndpoint(t *testing.T) {
	transport := NewStreamableHTTPTransport(StreamableHTTPTransportOptions{})
	spec := domain.ServerSpec{
		Name:      "remote",
		Transport: domain.TransportStreamableHTTP,
		HTTP: &domain.StreamableHTTPConfig{
			Endpoint: "",
		},
	}

	_, err := transport.Connect(context.Background(), "spec-remote", spec, domain.IOStreams{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint")
}

func TestStreamableHTTPTransport_ConnectionFailure(t *testing.T) {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcp.NewServer(&mcp.Implementation{Name: "remote", Version: "0.1.0"}, nil)
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	httpServer.Close()

	transport := NewStreamableHTTPTransport(StreamableHTTPTransportOptions{})
	spec := domain.ServerSpec{
		Name:            "remote",
		Transport:       domain.TransportStreamableHTTP,
		ProtocolVersion: domain.DefaultStreamableHTTPProtocolVersion,
		HTTP: &domain.StreamableHTTPConfig{
			Endpoint:   httpServer.URL,
			MaxRetries: -1,
		},
	}

	conn, err := transport.Connect(context.Background(), "spec-remote", spec, domain.IOStreams{})
	require.NoError(t, err)
	defer conn.Close()

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":3,"method":"ping","params":{}}`)
	_, err = conn.Call(context.Background(), msg)
	require.Error(t, err)
}

func TestStreamableHTTPTransport_CustomHeaders(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "remote",
		Version: "0.1.0",
	}, nil)
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})

	var sawHeader atomic.Bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer token" {
			sawHeader.Store(true)
		}
		streamable.ServeHTTP(w, r)
	})

	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	transport := NewStreamableHTTPTransport(StreamableHTTPTransportOptions{})
	spec := domain.ServerSpec{
		Name:            "remote",
		Transport:       domain.TransportStreamableHTTP,
		ProtocolVersion: domain.DefaultStreamableHTTPProtocolVersion,
		HTTP: &domain.StreamableHTTPConfig{
			Endpoint: httpServer.URL,
			Headers: map[string]string{
				"Authorization": "Bearer token",
			},
		},
	}

	conn, err := transport.Connect(context.Background(), "spec-remote", spec, domain.IOStreams{})
	require.NoError(t, err)
	defer conn.Close()

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"ping","params":{}}`)
	_, err = conn.Call(context.Background(), msg)
	require.NoError(t, err)
	require.True(t, sawHeader.Load())
}

func TestEffectiveMaxRetries(t *testing.T) {
	require.Equal(t, domain.DefaultStreamableHTTPMaxRetries, effectiveMaxRetries(0))
	require.Equal(t, 3, effectiveMaxRetries(3))
	require.Equal(t, -1, effectiveMaxRetries(-1))
}
