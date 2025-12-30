package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcpd/internal/domain"
)

const (
	defaultAutomaticMCPSessionTTL = 30 * time.Minute
	defaultAutomaticMCPCacheSize  = 10000
)

type automationService struct {
	state     *controlPlaneState
	registry  *callerRegistry
	discovery *discoveryService
	subAgent  domain.SubAgent
	cache     *domain.SessionCache
}

func newAutomationService(state *controlPlaneState, registry *callerRegistry, discovery *discoveryService) *automationService {
	return &automationService{
		state:     state,
		registry:  registry,
		discovery: discovery,
		cache:     domain.NewSessionCache(defaultAutomaticMCPSessionTTL, defaultAutomaticMCPCacheSize),
	}
}

func (a *automationService) SetSubAgent(agent domain.SubAgent) {
	a.subAgent = agent
}

func (a *automationService) IsSubAgentEnabled() bool {
	return a.subAgent != nil
}

func (a *automationService) IsSubAgentEnabledForCaller(caller string) bool {
	if a.subAgent == nil {
		return false
	}

	profile, err := a.registry.resolveProfile(caller)
	if err != nil {
		return false
	}

	if a.state.profileStore.Profiles == nil {
		return false
	}
	profileData, ok := a.state.profileStore.Profiles[profile.name]
	if !ok {
		return false
	}

	return profileData.Catalog.SubAgent.Enabled
}

func (a *automationService) AutomaticMCP(ctx context.Context, caller string, params domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	profile, err := a.registry.resolveProfile(caller)
	if err != nil {
		return domain.AutomaticMCPResult{}, err
	}

	if a.subAgent != nil {
		return a.subAgent.SelectToolsForCaller(ctx, caller, params)
	}

	return a.fallbackAutomaticMCP(caller, profile, params)
}

func (a *automationService) fallbackAutomaticMCP(caller string, profile *profileRuntime, params domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	if profile.tools == nil {
		return domain.AutomaticMCPResult{}, nil
	}

	snapshot := profile.tools.Snapshot()
	sessionKey := domain.AutomaticMCPSessionKey(caller, params.SessionID)

	toolsToSend := make([]json.RawMessage, 0, len(snapshot.Tools))
	sentSchemas := make(map[string]string)
	for _, tool := range snapshot.Tools {
		hash := hashToolSchema(tool.ToolJSON)
		shouldSend := params.ForceRefresh || a.cache.NeedsFull(sessionKey, tool.Name, hash)
		if !shouldSend {
			continue
		}

		raw := make([]byte, len(tool.ToolJSON))
		copy(raw, tool.ToolJSON)
		toolsToSend = append(toolsToSend, raw)
		sentSchemas[tool.Name] = hash
	}

	a.cache.Update(sessionKey, sentSchemas)

	return domain.AutomaticMCPResult{
		ETag:           snapshot.ETag,
		Tools:          toolsToSend,
		TotalAvailable: len(snapshot.Tools),
		Filtered:       len(toolsToSend),
	}, nil
}

func (a *automationService) AutomaticEval(ctx context.Context, caller string, params domain.AutomaticEvalParams) (json.RawMessage, error) {
	tool, err := a.getToolDefinition(caller, params.ToolName)
	if err != nil {
		return nil, err
	}

	if err := validateToolArguments(tool.ToolJSON, params.Arguments); err != nil {
		result, buildErr := buildAutomaticEvalSchemaError(tool.ToolJSON, err)
		if buildErr != nil {
			return nil, buildErr
		}
		return result, nil
	}

	return a.discovery.CallTool(ctx, caller, params.ToolName, params.Arguments, params.RoutingKey)
}

func (a *automationService) getToolDefinition(caller, name string) (domain.ToolDefinition, error) {
	profile, err := a.registry.resolveProfile(caller)
	if err != nil {
		return domain.ToolDefinition{}, err
	}
	if profile.tools == nil {
		return domain.ToolDefinition{}, domain.ErrToolNotFound
	}

	snapshot := profile.tools.Snapshot()
	for _, tool := range snapshot.Tools {
		if tool.Name == name {
			return tool, nil
		}
	}
	return domain.ToolDefinition{}, domain.ErrToolNotFound
}

func validateToolArguments(toolJSON, args json.RawMessage) error {
	var tool struct {
		InputSchema json.RawMessage `json:"inputSchema"`
	}
	if err := json.Unmarshal(toolJSON, &tool); err != nil {
		return fmt.Errorf("decode tool schema: %w", err)
	}
	if len(tool.InputSchema) == 0 {
		return nil
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		return fmt.Errorf("decode tool input schema: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return fmt.Errorf("resolve tool input schema: %w", err)
	}

	var payload any
	if len(args) == 0 {
		payload = map[string]any{}
	} else if err := json.Unmarshal(args, &payload); err != nil {
		return fmt.Errorf("decode tool arguments: %w", err)
	}

	if err := resolved.Validate(payload); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}
	return nil
}

func buildAutomaticEvalSchemaError(toolJSON json.RawMessage, err error) (json.RawMessage, error) {
	payload := struct {
		Error      string          `json:"error"`
		ToolSchema json.RawMessage `json:"toolSchema"`
	}{
		Error:      err.Error(),
		ToolSchema: toolJSON,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	result := mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(payloadJSON)},
		},
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func hashToolSchema(schema json.RawMessage) string {
	hasher := sha256.New()
	_, _ = hasher.Write(schema)
	return hex.EncodeToString(hasher.Sum(nil))
}
