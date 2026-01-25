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

	"mcpd/internal/domain"
)

func TestPromptIndex_NamespacedPrompt(t *testing.T) {
	ctx := context.Background()
	router := &promptRouter{
		prompts: []*mcp.Prompt{
			{Name: "echo"},
		},
	}

	specs := map[string]domain.ServerSpec{
		"echo": {Name: "echo"},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{
		ToolNamespaceStrategy: "prefix",
		ToolRefreshSeconds:    0,
	}

	index := NewPromptIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Prompts, 1)
	require.Equal(t, "echo.echo", snapshot.Prompts[0].Name)
	require.Equal(t, "spec-echo", snapshot.Prompts[0].SpecKey)
	require.Equal(t, "echo", snapshot.Prompts[0].ServerName)

	resultRaw, err := index.GetPrompt(ctx, "echo.echo", json.RawMessage(`{"name":"Pat"}`))
	require.NoError(t, err)

	var result mcp.GetPromptResult
	require.NoError(t, json.Unmarshal(resultRaw, &result))
	require.Len(t, result.Messages, 1)
	require.Equal(t, "ok", result.Messages[0].Content.(*mcp.TextContent).Text)
	require.Equal(t, "prompts/get", router.lastMethod)
	require.Equal(t, "echo", router.lastPromptName)
}

func TestPromptIndex_SnapshotForServer(t *testing.T) {
	ctx := context.Background()
	router := &promptRouter{
		prompts: []*mcp.Prompt{
			{Name: "echo"},
		},
	}

	specs := map[string]domain.ServerSpec{
		"echo": {Name: "echo"},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{
		ToolNamespaceStrategy: "prefix",
		ToolRefreshSeconds:    0,
	}

	index := NewPromptIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot, ok := index.SnapshotForServer("echo")
	require.True(t, ok)
	require.Len(t, snapshot.Prompts, 1)
	require.Equal(t, "echo", snapshot.Prompts[0].Name)
	require.Equal(t, "spec-echo", snapshot.Prompts[0].SpecKey)
	require.Equal(t, "echo", snapshot.Prompts[0].ServerName)

	resultRaw, err := index.GetPromptForServer(ctx, "echo", "echo", json.RawMessage(`{"name":"Pat"}`))
	require.NoError(t, err)

	var result mcp.GetPromptResult
	require.NoError(t, json.Unmarshal(resultRaw, &result))
	require.Len(t, result.Messages, 1)
	require.Equal(t, "ok", result.Messages[0].Content.(*mcp.TextContent).Text)
	require.Equal(t, "prompts/get", router.lastMethod)
	require.Equal(t, "echo", router.lastPromptName)
}

func TestPromptIndex_UsesCachedPromptsWhenNoReadyInstance(t *testing.T) {
	ctx := context.Background()
	cache := domain.NewMetadataCache()
	cache.SetPrompts("spec-echo", []domain.PromptDefinition{
		{Name: "echo", Description: "cached"},
	}, "etag")

	router := &noReadyPromptRouter{}
	specs := map[string]domain.ServerSpec{
		"echo": {Name: "echo"},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{
		ToolNamespaceStrategy: "prefix",
		ToolRefreshSeconds:    0,
	}

	index := NewPromptIndex(router, specs, specKeys, cfg, cache, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Prompts, 1)
	require.Equal(t, "echo.echo", snapshot.Prompts[0].Name)
	require.Equal(t, "spec-echo", snapshot.Prompts[0].SpecKey)
	require.Equal(t, "echo", snapshot.Prompts[0].ServerName)
}

func TestPromptIndex_SetBootstrapWaiterDoesNotDeadlock(t *testing.T) {
	index := NewPromptIndex(nil, map[string]domain.ServerSpec{}, map[string]string{}, domain.RuntimeConfig{}, nil, zap.NewNop(), nil, nil, nil)

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

type promptRouter struct {
	prompts        []*mcp.Prompt
	lastMethod     string
	lastPromptName string
}

func (r *promptRouter) Route(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage) (json.RawMessage, error) {
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
	case "prompts/list":
		return encodeResponse(req.ID, &mcp.ListPromptsResult{Prompts: r.prompts})
	case "prompts/get":
		var params mcp.GetPromptParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, err
		}
		r.lastPromptName = params.Name
		return encodeResponse(req.ID, &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{Role: "user", Content: &mcp.TextContent{Text: "ok"}},
			},
		})
	default:
		return nil, nil
	}
}

func (r *promptRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, opts domain.RouteOptions) (json.RawMessage, error) {
	return r.Route(ctx, serverType, specKey, routingKey, payload)
}

type noReadyPromptRouter struct{}

func (r *noReadyPromptRouter) Route(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage) (json.RawMessage, error) {
	return nil, domain.ErrNoReadyInstance
}

func (r *noReadyPromptRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, opts domain.RouteOptions) (json.RawMessage, error) {
	return r.Route(ctx, serverType, specKey, routingKey, payload)
}
