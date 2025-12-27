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

	index := NewPromptIndex(router, specs, specKeys, cfg, zap.NewNop(), nil)
	index.Start(ctx)
	defer index.Stop()

	snapshot := index.Snapshot()
	require.Len(t, snapshot.Prompts, 1)
	require.Equal(t, "echo.echo", snapshot.Prompts[0].Name)

	resultRaw, err := index.GetPrompt(ctx, "echo.echo", json.RawMessage(`{"name":"Pat"}`))
	require.NoError(t, err)

	var result mcp.GetPromptResult
	require.NoError(t, json.Unmarshal(resultRaw, &result))
	require.Len(t, result.Messages, 1)
	require.Equal(t, "ok", result.Messages[0].Content.(*mcp.TextContent).Text)
	require.Equal(t, "prompts/get", router.lastMethod)
	require.Equal(t, "echo", router.lastPromptName)
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
