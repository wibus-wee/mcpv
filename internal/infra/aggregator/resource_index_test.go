package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
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

	index := NewResourceIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Resources, 2)
	require.Equal(t, "file:///a", snapshot.Resources[0].URI)
	require.Equal(t, "spec-files", snapshot.Resources[0].SpecKey)
	require.Equal(t, "files", snapshot.Resources[0].ServerName)

	resultRaw, err := index.ReadResource(ctx, "file:///a")
	require.NoError(t, err)

	var result mcp.ReadResourceResult
	require.NoError(t, json.Unmarshal(resultRaw, &result))
	require.Len(t, result.Contents, 1)
	require.Equal(t, "file:///a", result.Contents[0].URI)
	require.Equal(t, "resources/read", router.lastMethod)
	require.Equal(t, "file:///a", router.lastURI)
}

func TestResourceIndex_SnapshotForServer(t *testing.T) {
	ctx := context.Background()
	router := &resourceRouter{
		resources: []*mcp.Resource{
			{URI: "file:///a"},
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

	index := NewResourceIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot, ok := index.SnapshotForServer("files")
	require.True(t, ok)
	require.Len(t, snapshot.Resources, 1)
	require.Equal(t, "file:///a", snapshot.Resources[0].URI)
	require.Equal(t, "spec-files", snapshot.Resources[0].SpecKey)
	require.Equal(t, "files", snapshot.Resources[0].ServerName)

	resultRaw, err := index.ReadResourceForServer(ctx, "files", "file:///a")
	require.NoError(t, err)

	var result mcp.ReadResourceResult
	require.NoError(t, json.Unmarshal(resultRaw, &result))
	require.Len(t, result.Contents, 1)
	require.Equal(t, "file:///a", result.Contents[0].URI)
	require.Equal(t, "resources/read", router.lastMethod)
	require.Equal(t, "file:///a", router.lastURI)
}

func TestResourceIndex_UsesCachedResourcesWhenNoReadyInstance(t *testing.T) {
	ctx := context.Background()
	cache := domain.NewMetadataCache()
	cache.SetResources("spec-files", []domain.ResourceDefinition{
		{URI: "file:///cached", Name: "cached"},
	}, "etag")

	router := &noReadyResourceRouter{}
	specs := map[string]domain.ServerSpec{
		"files": {Name: "files"},
	}
	specKeys := map[string]string{
		"files": "spec-files",
	}
	cfg := domain.RuntimeConfig{
		ToolRefreshSeconds: 0,
	}

	index := NewResourceIndex(router, specs, specKeys, cfg, cache, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Resources, 1)
	require.Equal(t, "file:///cached", snapshot.Resources[0].URI)
	require.Equal(t, "spec-files", snapshot.Resources[0].SpecKey)
	require.Equal(t, "files", snapshot.Resources[0].ServerName)
}

func TestResourceIndex_SetBootstrapWaiterDoesNotDeadlock(t *testing.T) {
	index := NewResourceIndex(nil, map[string]domain.ServerSpec{}, map[string]string{}, domain.RuntimeConfig{}, nil, zap.NewNop(), nil, nil, nil)

	done := make(chan struct{})
	go func() {
		index.SetBootstrapWaiter(func(context.Context) error { return nil })
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("SetBootstrapWaiter should not block")
	}
}

type resourceRouter struct {
	resources  []*mcp.Resource
	readResult *mcp.ReadResourceResult
	lastMethod string
	lastURI    string
}

func (r *resourceRouter) Route(_ context.Context, _, _, _ string, payload json.RawMessage) (json.RawMessage, error) {
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

func (r *resourceRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, _ domain.RouteOptions) (json.RawMessage, error) {
	return r.Route(ctx, serverType, specKey, routingKey, payload)
}

type noReadyResourceRouter struct{}

func (r *noReadyResourceRouter) Route(_ context.Context, _, _, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return nil, domain.ErrNoReadyInstance
}

func (r *noReadyResourceRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, _ domain.RouteOptions) (json.RawMessage, error) {
	return r.Route(ctx, serverType, specKey, routingKey, payload)
}
