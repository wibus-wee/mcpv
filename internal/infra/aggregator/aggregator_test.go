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
	cfg := domain.RuntimeConfig{
		ExposeTools:           true,
		ToolNamespaceStrategy: "prefix",
		ToolRefreshSeconds:    0,
	}

	index := NewToolIndex(router, specs, cfg, zap.NewNop())
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Tools, 1)
	require.Equal(t, "echo.echo", snapshot.Tools[0].Name)

	var tool mcp.Tool
	require.NoError(t, json.Unmarshal(snapshot.Tools[0].ToolJSON, &tool))
	require.Equal(t, "echo.echo", tool.Name)

	resultRaw, err := index.CallTool(ctx, "echo.echo", json.RawMessage(`{}`), "")
	require.NoError(t, err)

	var result mcp.CallToolResult
	require.NoError(t, json.Unmarshal(resultRaw, &result))
	require.Len(t, result.Content, 1)
	require.Equal(t, "ok", result.Content[0].(*mcp.TextContent).Text)

	require.Equal(t, "tools/call", router.lastMethod)
	require.Equal(t, "echo", router.lastServerType)
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
	cfg := domain.RuntimeConfig{ExposeTools: true, ToolNamespaceStrategy: "prefix"}

	index := NewToolIndex(router, specs, cfg, zap.NewNop())
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Tools, 1)
	require.Equal(t, "echo.echo", snapshot.Tools[0].Name)
}

func TestToolIndex_CallToolNotFound(t *testing.T) {
	ctx := context.Background()
	index := NewToolIndex(&fakeRouter{}, map[string]domain.ServerSpec{}, domain.RuntimeConfig{}, zap.NewNop())

	_, err := index.CallTool(ctx, "missing", nil, "")
	require.ErrorIs(t, err, domain.ErrToolNotFound)
}

type fakeRouter struct {
	tools          []*mcp.Tool
	callResult     *mcp.CallToolResult
	lastMethod     string
	lastServerType string
}

func (f *fakeRouter) Route(ctx context.Context, serverType, routingKey string, payload json.RawMessage) (json.RawMessage, error) {
	msg, err := jsonrpc.DecodeMessage(payload)
	if err != nil {
		return nil, err
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok {
		return nil, errors.New("invalid jsonrpc request")
	}
	f.lastMethod = req.Method
	f.lastServerType = serverType

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
