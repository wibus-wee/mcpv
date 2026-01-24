package domain

import (
	"context"
	"encoding/json"
	"time"
)

// SubAgentConfig contains configuration for the automatic SubAgent LLM provider.
// This is configured at the runtime level (shared across all profiles).
type SubAgentConfig struct {
	EnabledTags       []string `json:"enabledTags,omitempty"`
	Model              string `json:"model"`        // e.g., "gpt-4"
	Provider           string `json:"provider"`     // e.g., "openai"
	APIKey             string `json:"apiKey"`       // optional inline API key
	APIKeyEnvVar       string `json:"apiKeyEnvVar"` // e.g., "OPENAI_API_KEY"
	BaseURL            string `json:"baseURL"`      // e.g., "https://api.openai.com/v1" (optional)
	MaxToolsPerRequest int    `json:"maxToolsPerRequest"`
	FilterPrompt       string `json:"filterPrompt"` // optional custom prompt
}

// AutomaticMCPResult is returned by automatic_mcp.
type AutomaticMCPResult struct {
	ETag           string           `json:"etag"`
	Tools          []ToolDefinition `json:"tools"`
	TotalAvailable int              `json:"totalAvailable"`
	Filtered       int              `json:"filtered"`
}

// AutomaticMCPParams are the input parameters for automatic_mcp.
type AutomaticMCPParams struct {
	Query        string `json:"query"`
	SessionID    string `json:"sessionId"`
	ForceRefresh bool   `json:"forceRefresh"`
}

// AutomaticEvalParams are the input parameters for automatic_eval.
type AutomaticEvalParams struct {
	ToolName   string          `json:"toolName"`
	Arguments  json.RawMessage `json:"arguments"`
	RoutingKey string          `json:"routingKey,omitempty"`
}

// AutomaticMCPSessionKey returns the cache key for automatic_mcp deduplication.
func AutomaticMCPSessionKey(clientID, sessionID string) string {
	if sessionID != "" {
		return sessionID
	}
	return clientID
}

// SessionCacheEntry tracks what has been sent to a client.
type SessionCacheEntry struct {
	SessionKey   string
	SentSchemas  map[string]string // toolName -> schemaHash
	LastUpdated  time.Time
	RequestCount int
}

// SubAgent interface for tool filtering and proxying.
type SubAgent interface {
	// SelectToolsForClient filters tools based on the query using LLM reasoning
	// and applies deduplication based on session cache.
	SelectToolsForClient(ctx context.Context, clientID string, params AutomaticMCPParams) (AutomaticMCPResult, error)

	// InvalidateSession clears the session cache for a client (e.g., on context compression).
	InvalidateSession(clientID string)

	// Close shuts down the SubAgent and releases resources.
	Close() error
}

// ToolIndexProvider abstracts access to the current tool snapshot.
type ToolIndexProvider interface {
	// Snapshot returns the current tool snapshot.
	Snapshot() ToolSnapshot

	// CallTool invokes a tool by name with the given arguments.
	CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error)
}
