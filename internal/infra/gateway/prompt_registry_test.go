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

func TestPromptRegistry_ApplySnapshotRegistersAndRemovesPrompts(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "gateway", Version: "0.1.0"}, &mcp.ServerOptions{HasPrompts: true})

	registry := newPromptRegistry(server, func(name string) mcp.PromptHandler {
		return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: name}},
				},
			}, nil
		}
	}, zap.NewNop())

	prompt := &mcp.Prompt{
		Name:        "echo",
		Description: "echo prompt",
	}
	raw, err := json.Marshal(prompt)
	require.NoError(t, err)

	registry.ApplySnapshot(&controlv1.PromptsSnapshot{
		Etag: "v1",
		Prompts: []*controlv1.PromptDefinition{
			{Name: "echo", PromptJson: raw},
		},
	})

	_, session := connectClient(t, ctx, server)
	defer session.Close()

	prompts, err := session.ListPrompts(ctx, &mcp.ListPromptsParams{})
	require.NoError(t, err)
	require.Len(t, prompts.Prompts, 1)
	require.Equal(t, "echo", prompts.Prompts[0].Name)

	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{Name: "echo"})
	require.NoError(t, err)
	require.Len(t, result.Messages, 1)
	require.Equal(t, "echo", result.Messages[0].Content.(*mcp.TextContent).Text)

	registry.ApplySnapshot(&controlv1.PromptsSnapshot{Etag: "v2"})

	prompts, err = session.ListPrompts(ctx, &mcp.ListPromptsParams{})
	require.NoError(t, err)
	require.Len(t, prompts.Prompts, 0)
}
