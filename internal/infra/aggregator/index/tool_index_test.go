package index

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/mcpcodec"
)

func TestToolIndex_SnapshotPrefixedTool(t *testing.T) {
	ctx := context.Background()
	router := &fakeRouter{
		tools: []*mcp.Tool{
			{
				Name:        "echo",
				Description: "echo input",
				InputSchema: map[string]any{"type": "object"},
			},
		},
		callResult: &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		},
	}

	specs := map[string]domain.ServerSpec{
		"echo": {Name: "echo"},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{
		ExposeTools:           true,
		ToolNamespaceStrategy: domain.ToolNamespaceStrategyPrefix,
		ToolRefreshSeconds:    0,
	}

	index := NewToolIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Tools, 1)
	require.Equal(t, "echo.echo", snapshot.Tools[0].Name)
	require.Equal(t, "echo input", snapshot.Tools[0].Description)
	require.Equal(t, map[string]any{"type": "object"}, snapshot.Tools[0].InputSchema)
	require.Equal(t, "spec-echo", snapshot.Tools[0].SpecKey)
	require.Equal(t, "echo", snapshot.Tools[0].ServerName)

	resultRaw, err := index.CallTool(ctx, "echo.echo", json.RawMessage(`{}`), "")
	require.NoError(t, err)

	var result mcp.CallToolResult
	require.NoError(t, json.Unmarshal(resultRaw, &result))
	require.Len(t, result.Content, 1)
	require.Equal(t, "ok", result.Content[0].(*mcp.TextContent).Text)

	lastMethod, lastServerType := router.last()
	require.Equal(t, "tools/call", lastMethod)
	require.Equal(t, "echo", lastServerType)
}

func TestToolIndex_SnapshotForServer(t *testing.T) {
	ctx := context.Background()
	router := &fakeRouter{
		tools: []*mcp.Tool{
			{
				Name:        "echo",
				Description: "echo input",
				InputSchema: map[string]any{"type": "object"},
			},
		},
		callResult: &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		},
	}

	specs := map[string]domain.ServerSpec{
		"echo": {Name: "echo"},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{
		ExposeTools:           true,
		ToolNamespaceStrategy: domain.ToolNamespaceStrategyPrefix,
		ToolRefreshSeconds:    0,
	}

	index := NewToolIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot, ok := index.SnapshotForServer("echo")
	require.True(t, ok)
	require.Len(t, snapshot.Tools, 1)
	require.Equal(t, "echo", snapshot.Tools[0].Name)
	require.Equal(t, "spec-echo", snapshot.Tools[0].SpecKey)
	require.Equal(t, "echo", snapshot.Tools[0].ServerName)

	resultRaw, err := index.CallToolForServer(ctx, "echo", "echo", json.RawMessage(`{}`), "")
	require.NoError(t, err)

	var result mcp.CallToolResult
	require.NoError(t, json.Unmarshal(resultRaw, &result))
	require.Len(t, result.Content, 1)
	require.Equal(t, "ok", result.Content[0].(*mcp.TextContent).Text)

	lastMethod, lastServerType := router.last()
	require.Equal(t, "tools/call", lastMethod)
	require.Equal(t, "echo", lastServerType)
}

func TestToolIndex_RespectsExposeToolsAllowlist(t *testing.T) {
	ctx := context.Background()
	router := &fakeRouter{
		tools: []*mcp.Tool{
			{Name: "echo", InputSchema: map[string]any{"type": "object"}},
			{Name: "skip", InputSchema: map[string]any{"type": "object"}},
		},
		callResult: &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}},
	}

	specs := map[string]domain.ServerSpec{
		"echo": {
			Name:        "echo",
			ExposeTools: []string{"echo"},
		},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{ExposeTools: true, ToolNamespaceStrategy: domain.ToolNamespaceStrategyPrefix}

	index := NewToolIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Tools, 1)
	require.Equal(t, "echo.echo", snapshot.Tools[0].Name)
}

func TestToolIndex_UsesCachedToolsWhenNoReadyInstance(t *testing.T) {
	ctx := context.Background()
	cache := domain.NewMetadataCache()
	cache.SetTools("spec-echo", []domain.ToolDefinition{
		{Name: "echo", Description: "cached", InputSchema: map[string]any{"type": "object"}},
	}, "etag")

	router := &failingRouter{err: domain.ErrNoReadyInstance}
	specs := map[string]domain.ServerSpec{
		"echo": {Name: "echo"},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{
		ExposeTools:           true,
		ToolNamespaceStrategy: domain.ToolNamespaceStrategyPrefix,
		ToolRefreshSeconds:    0,
	}

	index := NewToolIndex(router, specs, specKeys, cfg, cache, zap.NewNop(), nil, nil, nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Tools, 1)
	require.Equal(t, "echo.echo", snapshot.Tools[0].Name)
	require.Equal(t, "spec-echo", snapshot.Tools[0].SpecKey)
	require.Equal(t, "echo", snapshot.Tools[0].ServerName)
}

func TestToolIndex_CachedSnapshot(t *testing.T) {
	cache := domain.NewMetadataCache()
	cache.SetTools("spec-echo", []domain.ToolDefinition{
		{
			Name:        "echo",
			Description: "cached",
			InputSchema: map[string]any{"type": "object"},
		},
	}, "etag")

	router := &failingRouter{err: domain.ErrNoReadyInstance}
	specs := map[string]domain.ServerSpec{
		"echo": {Name: "echo"},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{
		ExposeTools:           true,
		ToolNamespaceStrategy: domain.ToolNamespaceStrategyPrefix,
	}

	index := NewToolIndex(router, specs, specKeys, cfg, cache, zap.NewNop(), nil, nil, nil)

	snapshot := index.CachedSnapshot()
	require.Len(t, snapshot.Tools, 1)
	require.Equal(t, "echo.echo", snapshot.Tools[0].Name)
	require.Equal(t, "cached", snapshot.Tools[0].Description)
	require.Equal(t, "spec-echo", snapshot.Tools[0].SpecKey)
	require.Equal(t, "echo", snapshot.Tools[0].ServerName)
}

func TestToolIndex_CallToolNotFound(t *testing.T) {
	ctx := context.Background()
	index := NewToolIndex(&fakeRouter{}, map[string]domain.ServerSpec{}, map[string]string{}, domain.RuntimeConfig{}, nil, zap.NewNop(), nil, nil, nil)

	_, err := index.CallTool(ctx, "missing", nil, "")
	require.ErrorIs(t, err, domain.ErrToolNotFound)
}

func TestToolIndex_RefreshConcurrentFetches(t *testing.T) {
	ctx := context.Background()
	slowBlock := make(chan struct{})
	router := &blockingRouter{
		responses: map[string]toolListResponse{
			"fast": {tools: []*mcp.Tool{
				{Name: "fast", InputSchema: map[string]any{"type": "object"}},
			}},
			"slow": {
				tools: []*mcp.Tool{{Name: "slow", InputSchema: map[string]any{"type": "object"}}},
				block: slowBlock,
			},
		},
	}
	specs := map[string]domain.ServerSpec{
		"fast": {Name: "fast"},
		"slow": {Name: "slow"},
	}
	specKeys := map[string]string{
		"fast": "spec-fast",
		"slow": "spec-slow",
	}
	cfg := domain.RuntimeConfig{ExposeTools: true, ToolNamespaceStrategy: domain.ToolNamespaceStrategyPrefix}

	index := NewToolIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)

	done := make(chan error, 1)
	go func() {
		done <- index.refresh(ctx)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
		t.Fatalf("refresh completed before slow server released")
	case <-time.After(200 * time.Millisecond):
	}

	snapshot := index.Snapshot()
	require.Empty(t, snapshot.Tools, "snapshot should not update until all servers refresh")

	close(slowBlock)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatalf("refresh did not complete after releasing slow server")
	}

	snapshot = index.Snapshot()
	require.Len(t, snapshot.Tools, 2)
	require.Equal(t, "fast.fast", snapshot.Tools[0].Name)
	require.Equal(t, "slow.slow", snapshot.Tools[1].Name)
}

func TestToolIndex_SetBootstrapWaiterDoesNotDeadlock(t *testing.T) {
	index := NewToolIndex(nil, map[string]domain.ServerSpec{}, map[string]string{}, domain.RuntimeConfig{}, nil, zap.NewNop(), nil, nil, nil)

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

func TestIsObjectSchema(t *testing.T) {
	require.True(t, mcpcodec.IsObjectSchema(map[string]any{"type": "object"}))
	require.True(t, mcpcodec.IsObjectSchema(json.RawMessage(`{"type":["null","object"]}`)))
	require.False(t, mcpcodec.IsObjectSchema(json.RawMessage(`{"type":"string"}`)))
	require.False(t, mcpcodec.IsObjectSchema(nil))
	require.False(t, mcpcodec.IsObjectSchema("not json"))
}

func TestToolIndex_FlatNamespaceConflictsRename(t *testing.T) {
	ctx := context.Background()
	router := &blockingRouter{
		responses: map[string]toolListResponse{
			"a": {tools: []*mcp.Tool{
				{Name: "dup", InputSchema: map[string]any{"type": "object"}},
			}},
			"b": {tools: []*mcp.Tool{
				{Name: "dup", InputSchema: map[string]any{"type": "object"}},
			}},
		},
	}
	specs := map[string]domain.ServerSpec{
		"a": {Name: "a"},
		"b": {Name: "b"},
	}
	specKeys := map[string]string{
		"a": "spec-a",
		"b": "spec-b",
	}
	cfg := domain.RuntimeConfig{ExposeTools: true, ToolNamespaceStrategy: domain.ToolNamespaceStrategyFlat}

	index := NewToolIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)

	require.NoError(t, index.refresh(ctx))

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Tools, 2)
	require.Equal(t, "dup", snapshot.Tools[0].Name)
	require.Equal(t, "dup_b", snapshot.Tools[1].Name)
}

func TestToolIndex_CallToolPropagatesRouteError(t *testing.T) {
	ctx := context.Background()
	router := &callErrorRouter{
		tools: []*mcp.Tool{{Name: "echo", InputSchema: map[string]any{"type": "object"}}},
		err:   context.DeadlineExceeded,
	}
	specs := map[string]domain.ServerSpec{
		"echo": {Name: "echo"},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{ExposeTools: true, ToolNamespaceStrategy: domain.ToolNamespaceStrategyPrefix}

	index := NewToolIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)
	require.NoError(t, index.Refresh(ctx))

	_, err := index.CallTool(ctx, "echo.echo", json.RawMessage(`{}`), "")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestToolIndex_RefreshFailureOpensCircuitBreaker(t *testing.T) {
	ctx := context.Background()
	router := &flakyRouter{
		okTools: []*mcp.Tool{{Name: "echo", InputSchema: map[string]any{"type": "object"}}},
	}
	specs := map[string]domain.ServerSpec{
		"echo": {Name: "echo"},
	}
	specKeys := map[string]string{
		"echo": "spec-echo",
	}
	cfg := domain.RuntimeConfig{ExposeTools: true, ToolNamespaceStrategy: domain.ToolNamespaceStrategyPrefix}

	index := NewToolIndex(router, specs, specKeys, cfg, nil, zap.NewNop(), nil, nil, nil)
	require.NoError(t, index.refresh(ctx))

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Tools, 1)

	for i := 0; i < domain.DefaultRefreshFailureThreshold; i++ {
		_ = index.refresh(ctx)
	}

	snapshot = index.Snapshot()
	require.Empty(t, snapshot.Tools)
}

type fakeRouter struct {
	tools          []*mcp.Tool
	callResult     *mcp.CallToolResult
	mu             sync.Mutex
	lastMethod     string
	lastServerType string
}

func (f *fakeRouter) Route(_ context.Context, serverType, _, _ string, payload json.RawMessage) (json.RawMessage, error) {
	msg, err := jsonrpc.DecodeMessage(payload)
	if err != nil {
		return nil, err
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok {
		return nil, errors.New("invalid jsonrpc request")
	}
	f.mu.Lock()
	f.lastMethod = req.Method
	f.lastServerType = serverType
	f.mu.Unlock()

	switch req.Method {
	case "tools/list":
		return encodeResponse(req.ID, &mcp.ListToolsResult{Tools: f.tools})
	case "tools/call":
		if f.callResult == nil {
			f.callResult = &mcp.CallToolResult{}
		}
		return encodeResponse(req.ID, f.callResult)
	default:
		return nil, nil
	}
}

func (f *fakeRouter) last() (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastMethod, f.lastServerType
}

func (f *fakeRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, _ domain.RouteOptions) (json.RawMessage, error) {
	return f.Route(ctx, serverType, specKey, routingKey, payload)
}

type toolListResponse struct {
	tools []*mcp.Tool
	block <-chan struct{}
}

type blockingRouter struct {
	responses map[string]toolListResponse
}

func (b *blockingRouter) Route(ctx context.Context, serverType, _, _ string, payload json.RawMessage) (json.RawMessage, error) {
	msg, err := jsonrpc.DecodeMessage(payload)
	if err != nil {
		return nil, err
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok {
		return nil, errors.New("invalid jsonrpc request")
	}
	if req.Method != "tools/list" {
		return nil, errors.New("unsupported method")
	}
	resp, ok := b.responses[serverType]
	if !ok {
		return nil, fmt.Errorf("unknown server type: %s", serverType)
	}
	if resp.block != nil {
		select {
		case <-resp.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return encodeResponse(req.ID, &mcp.ListToolsResult{Tools: resp.tools})
}

func (b *blockingRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, _ domain.RouteOptions) (json.RawMessage, error) {
	return b.Route(ctx, serverType, specKey, routingKey, payload)
}

type failingRouter struct {
	err error
}

func (f *failingRouter) Route(_ context.Context, _, _, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return nil, f.err
}

func (f *failingRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, _ domain.RouteOptions) (json.RawMessage, error) {
	return f.Route(ctx, serverType, specKey, routingKey, payload)
}

type callErrorRouter struct {
	tools []*mcp.Tool
	err   error
}

func (c *callErrorRouter) Route(_ context.Context, _, _, _ string, payload json.RawMessage) (json.RawMessage, error) {
	msg, err := jsonrpc.DecodeMessage(payload)
	if err != nil {
		return nil, err
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok {
		return nil, errors.New("invalid jsonrpc request")
	}
	switch req.Method {
	case "tools/list":
		return encodeResponse(req.ID, &mcp.ListToolsResult{Tools: c.tools})
	case "tools/call":
		return nil, c.err
	default:
		return nil, errors.New("unsupported method")
	}
}

func (c *callErrorRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, _ domain.RouteOptions) (json.RawMessage, error) {
	return c.Route(ctx, serverType, specKey, routingKey, payload)
}

type flakyRouter struct {
	okTools []*mcp.Tool
	calls   int
}

func (f *flakyRouter) Route(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage) (json.RawMessage, error) {
	return f.RouteWithOptions(ctx, serverType, specKey, routingKey, payload, domain.RouteOptions{})
}

func (f *flakyRouter) RouteWithOptions(_ context.Context, _, _, _ string, payload json.RawMessage, _ domain.RouteOptions) (json.RawMessage, error) {
	msg, err := jsonrpc.DecodeMessage(payload)
	if err != nil {
		return nil, err
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok {
		return nil, errors.New("invalid jsonrpc request")
	}
	if req.Method != "tools/list" {
		return nil, nil
	}
	f.calls++
	if f.calls > 1 {
		return nil, errors.New("boom")
	}
	return encodeResponse(req.ID, &mcp.ListToolsResult{Tools: f.okTools})
}

func encodeResponse(id jsonrpc.ID, result any) (json.RawMessage, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	resp := &jsonrpc.Response{ID: id, Result: raw}
	wire, err := jsonrpc.EncodeMessage(resp)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(wire), nil
}
