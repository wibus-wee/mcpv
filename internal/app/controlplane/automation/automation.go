package automation

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/app/controlplane/discovery"
	"mcpv/internal/app/controlplane/registry"
	"mcpv/internal/domain"
	"mcpv/internal/infra/mcpcodec"
)

const (
	defaultAutomaticMCPSessionTTL = 30 * time.Minute
	defaultAutomaticMCPCacheSize  = 10000
)

type Service struct {
	state      State
	registry   *registry.ClientRegistry
	tools      *discovery.ToolDiscoveryService
	subAgentMu sync.RWMutex
	subAgent   domain.SubAgent
	cache      *domain.SessionCache
}

func NewAutomationService(state State, registry *registry.ClientRegistry, tools *discovery.ToolDiscoveryService) *Service {
	return &Service{
		state:    state,
		registry: registry,
		tools:    tools,
		cache:    domain.NewSessionCache(defaultAutomaticMCPSessionTTL, defaultAutomaticMCPCacheSize),
	}
}

func (a *Service) getSubAgent() domain.SubAgent {
	a.subAgentMu.RLock()
	agent := a.subAgent
	a.subAgentMu.RUnlock()
	return agent
}

// SetSubAgent sets the active SubAgent implementation.
func (a *Service) SetSubAgent(agent domain.SubAgent) {
	a.subAgentMu.Lock()
	a.subAgent = agent
	a.subAgentMu.Unlock()
}

// IsSubAgentEnabled reports whether SubAgent is enabled.
func (a *Service) IsSubAgentEnabled() bool {
	return a.getSubAgent() != nil
}

// IsSubAgentEnabledForClient reports whether SubAgent is enabled for a client.
func (a *Service) IsSubAgentEnabledForClient(client string) bool {
	agent := a.getSubAgent()
	if agent == nil {
		return false
	}
	return a.isSubAgentEnabledForClient(client)
}

func (a *Service) isSubAgentEnabledForClient(client string) bool {
	tags, err := a.registry.ResolveClientTags(client)
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
func (a *Service) AutomaticMCP(ctx context.Context, client string, params domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	agent := a.getSubAgent()
	if agent != nil && a.isSubAgentEnabledForClient(client) {
		return agent.SelectToolsForClient(ctx, client, params)
	}

	return a.fallbackAutomaticMCP(ctx, client, params)
}

func (a *Service) fallbackAutomaticMCP(ctx context.Context, client string, params domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	snapshot, err := a.tools.ListTools(ctx, client)
	if err != nil {
		return domain.AutomaticMCPResult{}, err
	}

	sessionKey := domain.AutomaticMCPSessionKey(client, params.SessionID)

	toolsToSend := make([]domain.ToolDefinition, 0, len(snapshot.Tools))
	sentSchemas := make(map[string]string)
	for _, tool := range snapshot.Tools {
		hash, err := mcpcodec.HashToolDefinition(tool)
		if err != nil {
			a.state.Logger().Warn("tool hash failed", zap.String("tool", tool.Name), zap.Error(err))
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
func (a *Service) AutomaticEval(ctx context.Context, client string, params domain.AutomaticEvalParams) (json.RawMessage, error) {
	if _, err := a.getToolDefinition(ctx, client, params.ToolName); err != nil {
		return nil, err
	}

	return a.tools.CallTool(ctx, client, params.ToolName, params.Arguments, params.RoutingKey)
}

func (a *Service) getToolDefinition(ctx context.Context, client, name string) (domain.ToolDefinition, error) {
	snapshot, err := a.tools.ListTools(ctx, client)
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
