package subagent

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AutomaticMCPTool returns the MCP tool definition for mcpv.automatic_mcp.
func AutomaticMCPTool() mcp.Tool {
	return mcp.Tool{
		Name:        "mcpv.automatic_mcp",
		Description: "Use this first-pass discovery helper whenever you need MCP tools for the current user task. Provide a natural-language query, reuse the same sessionId (any stable string, e.g. a conversation hash) to keep schema deduplication stable, and only flip forceRefresh or rotate the session when you have really lost context; example: query=\"Find Context7 docs\", sessionId=\"ctx7-run\", forceRefresh=false.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Description of the task you need to accomplish. Tools will be filtered based on relevance to this task.",
				},
				"sessionId": map[string]any{
					"type":        "string",
					"description": "Stable session identifier for schema deduplication. Change it when you lose context.",
				},
				"forceRefresh": map[string]any{
					"type":        "boolean",
					"description": "Force refresh all tool schemas, ignoring cache. Use when you've lost context.",
				},
			},
			"required": []string{},
		},
	}
}

// AutomaticEvalTool returns the MCP tool definition for mcpv.automatic_eval.
func AutomaticEvalTool() mcp.Tool {
	return mcp.Tool{
		Name:        "mcpv.automatic_eval",
		Description: "Invoke a concrete MCP tool that you previously discovered via mcpv.automatic_mcp. Pass the exact toolName, include arguments that match the advertised schema (JSON object), optionally add routingKey if you need sticky routing, and think of it as the execution phase.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"toolName": map[string]any{
					"type":        "string",
					"description": "The name of the tool to execute (as returned by automatic_mcp).",
				},
				"arguments": map[string]any{
					"type":        "object",
					"description": "Arguments to pass to the tool, matching its input schema.",
				},
				"routingKey": map[string]any{
					"type":        "string",
					"description": "Optional routing key for sticky session routing.",
				},
			},
			"required": []string{"toolName", "arguments"},
		},
	}
}
