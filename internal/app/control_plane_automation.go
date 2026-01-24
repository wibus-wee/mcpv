package app

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mcpcodec"
)

const (
	defaultAutomaticMCPSessionTTL = 30 * time.Minute
	defaultAutomaticMCPCacheSize  = 10000
)

type automationService struct {
	state     *controlPlaneState
	registry  *clientRegistry
	discovery *discoveryService
	subAgent  domain.SubAgent
	cache     *domain.SessionCache
}

func newAutomationService(state *controlPlaneState, registry *clientRegistry, discovery *discoveryService) *automationService {
	return &automationService{
		state:     state,
		registry:  registry,
		discovery: discovery,
		cache:     domain.NewSessionCache(defaultAutomaticMCPSessionTTL, defaultAutomaticMCPCacheSize),
	}
}

// SetSubAgent sets the active SubAgent implementation.
func (a *automationService) SetSubAgent(agent domain.SubAgent) {
	a.subAgent = agent
}

// IsSubAgentEnabled reports whether SubAgent is enabled.
func (a *automationService) IsSubAgentEnabled() bool {
	return a.subAgent != nil
}

// IsSubAgentEnabledForClient reports whether SubAgent is enabled for a client.
func (a *automationService) IsSubAgentEnabledForClient(client string) bool {
	if a.subAgent == nil {
		return false
	}
	tags, err := a.registry.resolveClientTags(client)
	if err != nil {
		return false
	}

	enabledTags := a.state.Runtime().SubAgent.EnabledTags
	if len(enabledTags) == 0 {
		return true
	}
	return hasTagOverlap(tags, enabledTags)
}

// AutomaticMCP filters tools using the automatic MCP flow.
func (a *automationService) AutomaticMCP(ctx context.Context, client string, params domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	if a.subAgent != nil && a.IsSubAgentEnabledForClient(client) {
		return a.subAgent.SelectToolsForClient(ctx, client, params)
	}

	return a.fallbackAutomaticMCP(ctx, client, params)
}

func (a *automationService) fallbackAutomaticMCP(ctx context.Context, client string, params domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	snapshot, err := a.discovery.ListTools(ctx, client)
	if err != nil {
		return domain.AutomaticMCPResult{}, err
	}

	sessionKey := domain.AutomaticMCPSessionKey(client, params.SessionID)

	toolsToSend := make([]domain.ToolDefinition, 0, len(snapshot.Tools))
	sentSchemas := make(map[string]string)
	for _, tool := range snapshot.Tools {
		hash, err := mcpcodec.HashToolDefinition(tool)
		if err != nil {
			a.state.logger.Warn("tool hash failed", zap.String("tool", tool.Name), zap.Error(err))
			toolsToSend = append(toolsToSend, domain.CloneToolDefinition(tool))
			continue
		}
		shouldSend := params.ForceRefresh || a.cache.NeedsFull(sessionKey, tool.Name, hash)
		if !shouldSend {
			continue
		}

		toolsToSend = append(toolsToSend, domain.CloneToolDefinition(tool))
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

// AutomaticEval evaluates a tool call using the automatic MCP flow.
func (a *automationService) AutomaticEval(ctx context.Context, client string, params domain.AutomaticEvalParams) (json.RawMessage, error) {
	if _, err := a.getToolDefinition(ctx, client, params.ToolName); err != nil {
		return nil, err
	}

	return a.discovery.CallTool(ctx, client, params.ToolName, params.Arguments, params.RoutingKey)
}

func (a *automationService) getToolDefinition(ctx context.Context, client, name string) (domain.ToolDefinition, error) {
	snapshot, err := a.discovery.ListTools(ctx, client)
	if err != nil {
		return domain.ToolDefinition{}, err
	}
	for _, tool := range snapshot.Tools {
		if tool.Name == name {
			return tool, nil
		}
	}
	return domain.ToolDefinition{}, domain.ErrToolNotFound
}

func hasTagOverlap(left []string, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(left))
	for _, tag := range left {
		set[tag] = struct{}{}
	}
	for _, tag := range right {
		if _, ok := set[tag]; ok {
			return true
		}
	}
	return false
}
