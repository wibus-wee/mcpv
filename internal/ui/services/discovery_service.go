package services

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/ui"
	"mcpv/internal/ui/mapping"
)

// DiscoveryService exposes tools/resources/prompts APIs.
type DiscoveryService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewDiscoveryService(deps *ServiceDeps) *DiscoveryService {
	return &DiscoveryService{
		deps:   deps,
		logger: deps.loggerNamed("discovery-service"),
	}
}

// ListTools lists all available tools.
func (s *DiscoveryService) ListTools(ctx context.Context) ([]ToolEntry, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	catalog, err := cp.ListToolCatalog(ctx)
	if err != nil {
		return nil, ui.MapDomainError(err)
	}

	manager := s.deps.manager()
	if manager != nil {
		manager.GetSharedState().SetToolSnapshot(toolSnapshotFromCatalog(catalog))
	}

	entries, err := mapping.MapToolCatalogEntries(catalog)
	if err != nil {
		return nil, ui.NewErrorWithDetails(ui.ErrCodeInternal, "Failed to map tools", err.Error())
	}
	return entries, nil
}

// ListResources lists resources.
func (s *DiscoveryService) ListResources(ctx context.Context, cursor string) (*ResourcePage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	page, err := cp.ListResourcesAll(ctx, cursor)
	if err != nil {
		return nil, ui.MapDomainError(err)
	}

	result, err := mapping.MapResourcePage(page)
	if err != nil {
		return nil, ui.NewErrorWithDetails(ui.ErrCodeInternal, "Failed to map resources", err.Error())
	}
	return result, nil
}

// ListPrompts lists prompt templates.
func (s *DiscoveryService) ListPrompts(ctx context.Context, cursor string) (*PromptPage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	page, err := cp.ListPromptsAll(ctx, cursor)
	if err != nil {
		return nil, ui.MapDomainError(err)
	}

	result, err := mapping.MapPromptPage(page)
	if err != nil {
		return nil, ui.NewErrorWithDetails(ui.ErrCodeInternal, "Failed to map prompts", err.Error())
	}
	return result, nil
}

// CallTool calls a tool.
func (s *DiscoveryService) CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	result, err := cp.CallToolAll(ctx, name, args, routingKey)
	if err != nil {
		return nil, ui.MapDomainError(err)
	}
	return result, nil
}

// ReadResource reads resource content.
func (s *DiscoveryService) ReadResource(ctx context.Context, uri string) (json.RawMessage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	result, err := cp.ReadResourceAll(ctx, uri)
	if err != nil {
		return nil, ui.MapDomainError(err)
	}
	return result, nil
}

// GetPrompt gets a prompt template.
func (s *DiscoveryService) GetPrompt(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	result, err := cp.GetPromptAll(ctx, name, args)
	if err != nil {
		return nil, ui.MapDomainError(err)
	}
	return result, nil
}

func toolSnapshotFromCatalog(snapshot domain.ToolCatalogSnapshot) domain.ToolSnapshot {
	tools := make([]domain.ToolDefinition, 0, len(snapshot.Tools))
	for _, entry := range snapshot.Tools {
		tools = append(tools, entry.Definition)
	}
	return domain.ToolSnapshot{
		ETag:  snapshot.ETag,
		Tools: tools,
	}
}
