package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/app"
	controlv1 "mcpv/pkg/api/control/v1"
)

func TestResourceRegistry_ApplySnapshotRegistersAndRemovesResources(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "gateway", Version: app.Version}, &mcp.ServerOptions{HasResources: true})

	registry := newResourceRegistry(server, func(uri string) mcp.ResourceHandler {
		return func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: uri, Text: "ok"}},
			}, nil
		}
	}, zap.NewNop())

	resource := &mcp.Resource{
		URI:  "file:///a",
		Name: "a",
	}
	raw, err := json.Marshal(resource)
	require.NoError(t, err)

	registry.ApplySnapshot(&controlv1.ResourcesSnapshot{
		Etag: "v1",
		Resources: []*controlv1.ResourceDefinition{
			{Uri: "file:///a", ResourceJson: raw},
		},
	})

	_, session := connectClient(ctx, t, server)
	defer session.Close()

	resources, err := session.ListResources(ctx, &mcp.ListResourcesParams{})
	require.NoError(t, err)
	require.Len(t, resources.Resources, 1)
	require.Equal(t, "file:///a", resources.Resources[0].URI)

	read, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "file:///a"})
	require.NoError(t, err)
	require.Len(t, read.Contents, 1)
	require.Equal(t, "ok", read.Contents[0].Text)

	registry.ApplySnapshot(&controlv1.ResourcesSnapshot{Etag: "v2"})

	resources, err = session.ListResources(ctx, &mcp.ListResourcesParams{})
	require.NoError(t, err)
	require.Len(t, resources.Resources, 0)
}
