package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	controlv1 "mcpd/pkg/api/control/v1"
)

func TestToolRegistry_ApplySnapshotRegistersAndRemovesTools(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "gateway", Version: "0.1.0"}, &mcp.ServerOptions{HasTools: true})

	registry := newToolRegistry(server, func(name string) mcp.ToolHandler {
		return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: name}},
			}, nil
		}
	}, zap.NewNop())

	tool := &mcp.Tool{
		Name:        "echo.echo",
		Description: "echo input",
		InputSchema: map[string]any{"type": "object"},
	}
	raw, err := json.Marshal(tool)
	require.NoError(t, err)

	registry.ApplySnapshot(&controlv1.ToolsSnapshot{
		Etag: "v1",
		Tools: []*controlv1.ToolDefinition{
			{Name: "echo.echo", ToolJson: raw},
		},
	})

	_, session := connectClient(t, ctx, server)
	defer session.Close()

	res, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, res.Tools, 1)
	require.Equal(t, "echo.echo", res.Tools[0].Name)

	registry.ApplySnapshot(&controlv1.ToolsSnapshot{Etag: "v2"})

	res, err = session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, res.Tools, 0)
}

func connectClient(t *testing.T, ctx context.Context, server *mcp.Server) (*mcp.Client, *mcp.ClientSession) {
	t.Helper()
	ct, st := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	return client, session
}
