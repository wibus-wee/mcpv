package ui

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"
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

	snapshot, err := cp.ListToolsAllProfiles(ctx)
	if err != nil {
		return nil, MapDomainError(err)
	}

	manager := s.deps.manager()
	if manager != nil {
		manager.GetSharedState().SetToolSnapshot(snapshot)
	}

	return mapToolEntries(snapshot), nil
}

// ListResources lists resources.
func (s *DiscoveryService) ListResources(ctx context.Context, cursor string) (*ResourcePage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	page, err := cp.ListResourcesAllProfiles(ctx, cursor)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapResourcePage(page), nil
}

// ListPrompts lists prompt templates.
func (s *DiscoveryService) ListPrompts(ctx context.Context, cursor string) (*PromptPage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	page, err := cp.ListPromptsAllProfiles(ctx, cursor)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapPromptPage(page), nil
}

// CallTool calls a tool.
func (s *DiscoveryService) CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	specKey := s.deps.extractSpecKeyFromCache(name)
	result, err := cp.CallToolAllProfiles(ctx, name, args, routingKey, specKey)
	if err != nil {
		return nil, MapDomainError(err)
	}
	return result, nil
}

// ReadResource reads resource content.
func (s *DiscoveryService) ReadResource(ctx context.Context, uri string) (json.RawMessage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	result, err := cp.ReadResourceAllProfiles(ctx, uri, "")
	if err != nil {
		return nil, MapDomainError(err)
	}
	return result, nil
}

// GetPrompt gets a prompt template.
func (s *DiscoveryService) GetPrompt(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	result, err := cp.GetPromptAllProfiles(ctx, name, args, "")
	if err != nil {
		return nil, MapDomainError(err)
	}
	return result, nil
}
