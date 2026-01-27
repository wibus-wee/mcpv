package lifecycle

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/transport"
)

func TestManager_StartInstance_StreamableHTTP(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "remote",
		Version: "0.1.0",
	}, nil)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})

	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	launcher := &fakeLauncher{}
	httpTransport := transport.NewStreamableHTTPTransport(transport.StreamableHTTPTransportOptions{})
	manager := NewManager(context.Background(), launcher, httpTransport, zap.NewNop())

	spec := domain.ServerSpec{
		Name:            "remote",
		Transport:       domain.TransportStreamableHTTP,
		ProtocolVersion: domain.DefaultStreamableHTTPProtocolVersion,
		HTTP: &domain.StreamableHTTPConfig{
			Endpoint:   httpServer.URL,
			MaxRetries: 1,
		},
		MaxConcurrent: 1,
	}

	inst, err := manager.StartInstance(context.Background(), "spec-remote", spec)
	require.NoError(t, err)
	require.Equal(t, domain.InstanceStateReady, inst.State())
	require.Nil(t, launcher.startCtx)
}
