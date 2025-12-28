package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

func TestResourceIndex_SnapshotAndRead(t *testing.T) {
	ctx := context.Background()
	router := &resourceRouter{
		resources: []*mcp.Resource{
			{URI: "file:///a"},
			{URI: "file:///b"},
		},
	}

	specs := map[string]domain.ServerSpec{
		"files": {Name: "files"},
	}
	specKeys := map[string]string{
		"files": "spec-files",
	}
	cfg := domain.RuntimeConfig{
		ToolRefreshSeconds: 0,
	}

	index := NewResourceIndex(router, specs, specKeys, cfg, zap.NewNop(), nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Resources, 2)
	require.Equal(t, "file:///a", snapshot.Resources[0].URI)

	resultRaw, err := index.ReadResource(ctx, "file:///a")
	require.NoError(t, err)

	var result mcp.ReadResourceResult
	require.NoError(t, json.Unmarshal(resultRaw, &result))
	require.Len(t, result.Contents, 1)
	require.Equal(t, "file:///a", result.Contents[0].URI)
	require.Equal(t, "resources/read", router.lastMethod)
	require.Equal(t, "file:///a", router.lastURI)
}

type resourceRouter struct {
	resources  []*mcp.Resource
	readResult *mcp.ReadResourceResult
	lastMethod string
	lastURI    string
}

func (r *resourceRouter) Route(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage) (json.RawMessage, error) {
	msg, err := jsonrpc.DecodeMessage(payload)
	if err != nil {
		return nil, err
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok {
		return nil, errors.New("invalid jsonrpc request")
	}
	r.lastMethod = req.Method

	switch req.Method {
	case "resources/list":
		return encodeResponse(req.ID, &mcp.ListResourcesResult{Resources: r.resources})
	case "resources/read":
		var params mcp.ReadResourceParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, err
		}
		r.lastURI = params.URI
		if r.readResult == nil {
			r.readResult = &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: params.URI, Text: "ok"}},
			}
		}
		return encodeResponse(req.ID, r.readResult)
	default:
		return nil, nil
	}
}

func (r *resourceRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, opts domain.RouteOptions) (json.RawMessage, error) {
	return r.Route(ctx, serverType, specKey, routingKey, payload)
}
