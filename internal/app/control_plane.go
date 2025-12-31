package app

import (
	"context"
	"encoding/json"
	"errors"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
)

type ControlPlane struct {
	state         *controlPlaneState
	registry      *callerRegistry
	discovery     *discoveryService
	observability *observabilityService
	automation    *automationService
}

func NewControlPlane(
	state *controlPlaneState,
	registry *callerRegistry,
	discovery *discoveryService,
	observability *observabilityService,
	automation *automationService,
) *ControlPlane {
	return &ControlPlane{
		state:         state,
		registry:      registry,
		discovery:     discovery,
		observability: observability,
		automation:    automation,
	}
}

func (c *ControlPlane) StartCallerMonitor(ctx context.Context) {
	c.registry.StartMonitor(ctx)
}

func (c *ControlPlane) Info(ctx context.Context) (domain.ControlPlaneInfo, error) {
	return c.state.info, nil
}

func (c *ControlPlane) RegisterCaller(ctx context.Context, caller string, pid int) (string, error) {
	return c.registry.RegisterCaller(ctx, caller, pid)
}

func (c *ControlPlane) UnregisterCaller(ctx context.Context, caller string) error {
	return c.registry.UnregisterCaller(ctx, caller)
}

func (c *ControlPlane) ListActiveCallers(ctx context.Context) ([]domain.ActiveCaller, error) {
	return c.registry.ListActiveCallers(ctx)
}

func (c *ControlPlane) WatchActiveCallers(ctx context.Context) (<-chan domain.ActiveCallerSnapshot, error) {
	return c.registry.WatchActiveCallers(ctx)
}

func (c *ControlPlane) ListTools(ctx context.Context, caller string) (domain.ToolSnapshot, error) {
	return c.discovery.ListTools(ctx, caller)
}

func (c *ControlPlane) ListToolsAllProfiles(ctx context.Context) (domain.ToolSnapshot, error) {
	return c.discovery.ListToolsAllProfiles(ctx)
}

func (c *ControlPlane) WatchTools(ctx context.Context, caller string) (<-chan domain.ToolSnapshot, error) {
	return c.discovery.WatchTools(ctx, caller)
}

func (c *ControlPlane) CallTool(ctx context.Context, caller, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	return c.discovery.CallTool(ctx, caller, name, args, routingKey)
}

func (c *ControlPlane) CallToolAllProfiles(ctx context.Context, name string, args json.RawMessage, routingKey, specKey string) (json.RawMessage, error) {
	return c.discovery.CallToolAllProfiles(ctx, name, args, routingKey, specKey)
}

func (c *ControlPlane) ListResources(ctx context.Context, caller string, cursor string) (domain.ResourcePage, error) {
	return c.discovery.ListResources(ctx, caller, cursor)
}

func (c *ControlPlane) ListResourcesAllProfiles(ctx context.Context, cursor string) (domain.ResourcePage, error) {
	return c.discovery.ListResourcesAllProfiles(ctx, cursor)
}

func (c *ControlPlane) WatchResources(ctx context.Context, caller string) (<-chan domain.ResourceSnapshot, error) {
	return c.discovery.WatchResources(ctx, caller)
}

func (c *ControlPlane) ReadResource(ctx context.Context, caller, uri string) (json.RawMessage, error) {
	return c.discovery.ReadResource(ctx, caller, uri)
}

func (c *ControlPlane) ReadResourceAllProfiles(ctx context.Context, uri, specKey string) (json.RawMessage, error) {
	return c.discovery.ReadResourceAllProfiles(ctx, uri, specKey)
}

func (c *ControlPlane) ListPrompts(ctx context.Context, caller string, cursor string) (domain.PromptPage, error) {
	return c.discovery.ListPrompts(ctx, caller, cursor)
}

func (c *ControlPlane) ListPromptsAllProfiles(ctx context.Context, cursor string) (domain.PromptPage, error) {
	return c.discovery.ListPromptsAllProfiles(ctx, cursor)
}

func (c *ControlPlane) WatchPrompts(ctx context.Context, caller string) (<-chan domain.PromptSnapshot, error) {
	return c.discovery.WatchPrompts(ctx, caller)
}

func (c *ControlPlane) GetPrompt(ctx context.Context, caller, name string, args json.RawMessage) (json.RawMessage, error) {
	return c.discovery.GetPrompt(ctx, caller, name, args)
}

func (c *ControlPlane) GetPromptAllProfiles(ctx context.Context, name string, args json.RawMessage, specKey string) (json.RawMessage, error) {
	return c.discovery.GetPromptAllProfiles(ctx, name, args, specKey)
}

func (c *ControlPlane) StreamLogs(ctx context.Context, caller string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return c.observability.StreamLogs(ctx, caller, minLevel)
}

func (c *ControlPlane) StreamLogsAllProfiles(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return c.observability.StreamLogsAllProfiles(ctx, minLevel)
}

func (c *ControlPlane) GetProfileStore() domain.ProfileStore {
	return c.state.profileStore
}

func (c *ControlPlane) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	return c.observability.GetPoolStatus(ctx)
}

func (c *ControlPlane) GetServerInitStatus(ctx context.Context) ([]domain.ServerInitStatus, error) {
	return c.observability.GetServerInitStatus(ctx)
}

func (c *ControlPlane) RetryServerInit(ctx context.Context, specKey string) error {
	if c.state.initManager == nil {
		return errors.New("server init manager not configured")
	}
	return c.state.initManager.RetrySpec(specKey)
}

func (c *ControlPlane) WatchRuntimeStatus(ctx context.Context, caller string) (<-chan domain.RuntimeStatusSnapshot, error) {
	return c.observability.WatchRuntimeStatus(ctx, caller)
}

func (c *ControlPlane) WatchRuntimeStatusAllProfiles(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
	return c.observability.WatchRuntimeStatusAllProfiles(ctx)
}

func (c *ControlPlane) WatchServerInitStatus(ctx context.Context, caller string) (<-chan domain.ServerInitStatusSnapshot, error) {
	return c.observability.WatchServerInitStatus(ctx, caller)
}

func (c *ControlPlane) WatchServerInitStatusAllProfiles(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
	return c.observability.WatchServerInitStatusAllProfiles(ctx)
}

func (c *ControlPlane) SetRuntimeStatusIndex(idx *aggregator.RuntimeStatusIndex) {
	c.observability.SetRuntimeStatusIndex(idx)
}

func (c *ControlPlane) SetServerInitIndex(idx *aggregator.ServerInitIndex) {
	c.observability.SetServerInitIndex(idx)
}

func (c *ControlPlane) SetSubAgent(agent domain.SubAgent) {
	c.automation.SetSubAgent(agent)
}

func (c *ControlPlane) IsSubAgentEnabledForCaller(caller string) bool {
	return c.automation.IsSubAgentEnabledForCaller(caller)
}

func (c *ControlPlane) IsSubAgentEnabled() bool {
	return c.automation.IsSubAgentEnabled()
}

func (c *ControlPlane) GetToolSnapshotForCaller(caller string) (domain.ToolSnapshot, error) {
	return c.discovery.GetToolSnapshotForCaller(caller)
}

func (c *ControlPlane) AutomaticMCP(ctx context.Context, caller string, params domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	return c.automation.AutomaticMCP(ctx, caller, params)
}

func (c *ControlPlane) AutomaticEval(ctx context.Context, caller string, params domain.AutomaticEvalParams) (json.RawMessage, error) {
	return c.automation.AutomaticEval(ctx, caller, params)
}
