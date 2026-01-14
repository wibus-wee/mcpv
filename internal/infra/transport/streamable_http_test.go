package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
