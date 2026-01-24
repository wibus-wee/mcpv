package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
)

// ControlPlane aggregates control plane services behind a facade.
type ControlPlane struct {
	state         *controlPlaneState
	registry      *clientRegistry
	discovery     *discoveryService
	observability *observabilityService
	automation    *automationService
}

// NewControlPlane constructs a control plane facade from services.
func NewControlPlane(
	state *controlPlaneState,
	registry *clientRegistry,
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

// StartClientMonitor begins client monitoring and tag change handling.
func (c *ControlPlane) StartClientMonitor(ctx context.Context) {
	c.discovery.StartClientChangeListener(ctx)
	c.registry.StartMonitor(ctx)
}

// Info returns control plane metadata.
func (c *ControlPlane) Info(ctx context.Context) (domain.ControlPlaneInfo, error) {
	return c.state.info, nil
}

// RegisterClient registers a client with the control plane.
func (c *ControlPlane) RegisterClient(ctx context.Context, client string, pid int, tags []string) (domain.ClientRegistration, error) {
	return c.registry.RegisterClient(ctx, client, pid, tags)
}

// UnregisterClient unregisters a client.
func (c *ControlPlane) UnregisterClient(ctx context.Context, client string) error {
	return c.registry.UnregisterClient(ctx, client)
}

// ListActiveClients lists active clients.
func (c *ControlPlane) ListActiveClients(ctx context.Context) ([]domain.ActiveClient, error) {
	return c.registry.ListActiveClients(ctx)
}

// WatchActiveClients streams active client updates.
func (c *ControlPlane) WatchActiveClients(ctx context.Context) (<-chan domain.ActiveClientSnapshot, error) {
	return c.registry.WatchActiveClients(ctx)
}

// ListTools lists tools visible to a client.
func (c *ControlPlane) ListTools(ctx context.Context, client string) (domain.ToolSnapshot, error) {
	return c.discovery.ListTools(ctx, client)
}

// ListToolCatalog returns the full tool catalog snapshot.
func (c *ControlPlane) ListToolCatalog(ctx context.Context) (domain.ToolCatalogSnapshot, error) {
	return c.discovery.ListToolCatalog(ctx)
}

// WatchTools streams tool snapshots for a client.
func (c *ControlPlane) WatchTools(ctx context.Context, client string) (<-chan domain.ToolSnapshot, error) {
	return c.discovery.WatchTools(ctx, client)
}

// CallTool executes a tool on behalf of a client.
func (c *ControlPlane) CallTool(ctx context.Context, client, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	return c.discovery.CallTool(ctx, client, name, args, routingKey)
}

// ListResources lists resources visible to a client.
func (c *ControlPlane) ListResources(ctx context.Context, client string, cursor string) (domain.ResourcePage, error) {
	return c.discovery.ListResources(ctx, client, cursor)
}

// WatchResources streams resource snapshots for a client.
func (c *ControlPlane) WatchResources(ctx context.Context, client string) (<-chan domain.ResourceSnapshot, error) {
	return c.discovery.WatchResources(ctx, client)
}

// ReadResource reads a resource on behalf of a client.
func (c *ControlPlane) ReadResource(ctx context.Context, client, uri string) (json.RawMessage, error) {
	return c.discovery.ReadResource(ctx, client, uri)
}

// ListPrompts lists prompts visible to a client.
func (c *ControlPlane) ListPrompts(ctx context.Context, client string, cursor string) (domain.PromptPage, error) {
	return c.discovery.ListPrompts(ctx, client, cursor)
}

// WatchPrompts streams prompt snapshots for a client.
func (c *ControlPlane) WatchPrompts(ctx context.Context, client string) (<-chan domain.PromptSnapshot, error) {
	return c.discovery.WatchPrompts(ctx, client)
}

// GetPrompt resolves a prompt for a client.
func (c *ControlPlane) GetPrompt(ctx context.Context, client, name string, args json.RawMessage) (json.RawMessage, error) {
	return c.discovery.GetPrompt(ctx, client, name, args)
}

// StreamLogs streams logs for a client.
func (c *ControlPlane) StreamLogs(ctx context.Context, client string, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return c.observability.StreamLogs(ctx, client, minLevel)
}

// StreamLogsAllServers streams logs across all servers.
func (c *ControlPlane) StreamLogsAllServers(ctx context.Context, minLevel domain.LogLevel) (<-chan domain.LogEntry, error) {
	return c.observability.StreamLogsAllServers(ctx, minLevel)
}

// GetCatalog returns the current catalog.
func (c *ControlPlane) GetCatalog() domain.Catalog {
	return c.state.Catalog()
}

// GetPoolStatus returns the current pool status snapshot.
func (c *ControlPlane) GetPoolStatus(ctx context.Context) ([]domain.PoolInfo, error) {
	return c.observability.GetPoolStatus(ctx)
}

// GetServerInitStatus returns current server init statuses.
func (c *ControlPlane) GetServerInitStatus(ctx context.Context) ([]domain.ServerInitStatus, error) {
	return c.observability.GetServerInitStatus(ctx)
}

// RetryServerInit requests a retry for server initialization.
func (c *ControlPlane) RetryServerInit(ctx context.Context, specKey string) error {
	if c.state.initManager == nil {
		return errors.New("server init manager not configured")
	}
	return c.state.initManager.RetrySpec(specKey)
}

// WatchRuntimeStatus streams runtime status snapshots for a client.
func (c *ControlPlane) WatchRuntimeStatus(ctx context.Context, client string) (<-chan domain.RuntimeStatusSnapshot, error) {
	return c.observability.WatchRuntimeStatus(ctx, client)
}

// WatchRuntimeStatusAllServers streams runtime status snapshots across servers.
func (c *ControlPlane) WatchRuntimeStatusAllServers(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
	return c.observability.WatchRuntimeStatusAllServers(ctx)
}

// WatchServerInitStatus streams server init status snapshots for a client.
func (c *ControlPlane) WatchServerInitStatus(ctx context.Context, client string) (<-chan domain.ServerInitStatusSnapshot, error) {
	return c.observability.WatchServerInitStatus(ctx, client)
}

// WatchServerInitStatusAllServers streams server init status snapshots across servers.
func (c *ControlPlane) WatchServerInitStatusAllServers(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
	return c.observability.WatchServerInitStatusAllServers(ctx)
}

// SetRuntimeStatusIndex updates the runtime status index.
func (c *ControlPlane) SetRuntimeStatusIndex(idx *aggregator.RuntimeStatusIndex) {
	c.observability.SetRuntimeStatusIndex(idx)
}

// SetServerInitIndex updates the server init status index.
func (c *ControlPlane) SetServerInitIndex(idx *aggregator.ServerInitIndex) {
	c.observability.SetServerInitIndex(idx)
}

// SetSubAgent sets the active SubAgent implementation.
func (c *ControlPlane) SetSubAgent(agent domain.SubAgent) {
	c.automation.SetSubAgent(agent)
}

// IsSubAgentEnabledForClient reports whether SubAgent is enabled for a client.
func (c *ControlPlane) IsSubAgentEnabledForClient(client string) bool {
	return c.automation.IsSubAgentEnabledForClient(client)
}

// IsSubAgentEnabled reports whether SubAgent is enabled.
func (c *ControlPlane) IsSubAgentEnabled() bool {
	return c.automation.IsSubAgentEnabled()
}

// GetToolSnapshotForClient returns the tool snapshot for a client.
func (c *ControlPlane) GetToolSnapshotForClient(client string) (domain.ToolSnapshot, error) {
	return c.discovery.GetToolSnapshotForClient(client)
}

// AutomaticMCP filters tools using the automatic MCP flow.
func (c *ControlPlane) AutomaticMCP(ctx context.Context, client string, params domain.AutomaticMCPParams) (domain.AutomaticMCPResult, error) {
	return c.automation.AutomaticMCP(ctx, client, params)
}

// AutomaticEval evaluates a tool call using the automatic MCP flow.
func (c *ControlPlane) AutomaticEval(ctx context.Context, client string, params domain.AutomaticEvalParams) (json.RawMessage, error) {
	return c.automation.AutomaticEval(ctx, client, params)
}

// GetBootstrapProgress returns bootstrap progress.
func (c *ControlPlane) GetBootstrapProgress(ctx context.Context) (domain.BootstrapProgress, error) {
	if c.state.bootstrapManager == nil {
		return domain.BootstrapProgress{State: domain.BootstrapCompleted}, nil
	}
	return c.state.bootstrapManager.GetProgress(), nil
}

// WatchBootstrapProgress streams bootstrap progress updates.
func (c *ControlPlane) WatchBootstrapProgress(ctx context.Context) (<-chan domain.BootstrapProgress, error) {
	ch := make(chan domain.BootstrapProgress, 1)

	if c.state.bootstrapManager == nil {
		close(ch)
		return ch, nil
	}

	go func() {
		defer close(ch)

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		// Send initial progress
		select {
		case ch <- c.state.bootstrapManager.GetProgress():
		case <-ctx.Done():
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				progress := c.state.bootstrapManager.GetProgress()

				select {
				case ch <- progress:
				default:
				}

				// Stop watching if bootstrap is done
				if progress.State == domain.BootstrapCompleted || progress.State == domain.BootstrapFailed {
					return
				}
			}
		}
	}()

	return ch, nil
}
