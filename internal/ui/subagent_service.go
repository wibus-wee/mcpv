package ui

import (
	"context"

	"go.uber.org/zap"

	"mcpd/internal/infra/catalog"
)

// SubAgentService exposes SubAgent configuration APIs.
type SubAgentService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewSubAgentService(deps *ServiceDeps) *SubAgentService {
	return &SubAgentService{
		deps:   deps,
		logger: deps.loggerNamed("subagent-service"),
	}
}

// GetSubAgentConfig returns the runtime-level SubAgent configuration.
func (s *SubAgentService) GetSubAgentConfig(ctx context.Context) (SubAgentConfigDetail, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return SubAgentConfigDetail{}, err
	}

	catalog := cp.GetCatalog()
	cfg := catalog.Runtime.SubAgent
	return SubAgentConfigDetail{
		EnabledTags:        append([]string(nil), cfg.EnabledTags...),
		Model:              cfg.Model,
		Provider:           cfg.Provider,
		APIKeyEnvVar:       cfg.APIKeyEnvVar,
		BaseURL:            cfg.BaseURL,
		MaxToolsPerRequest: cfg.MaxToolsPerRequest,
		FilterPrompt:       cfg.FilterPrompt,
	}, nil
}

// UpdateSubAgentConfig updates the runtime-level SubAgent config.
func (s *SubAgentService) UpdateSubAgentConfig(ctx context.Context, req UpdateSubAgentConfigRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}

	model := req.Model
	provider := req.Provider
	apiKeyEnvVar := req.APIKeyEnvVar
	baseURL := req.BaseURL
	maxTools := req.MaxToolsPerRequest
	filterPrompt := req.FilterPrompt

	update := catalog.SubAgentConfigUpdate{
		EnabledTags:        nil,
		Model:              &model,
		Provider:           &provider,
		APIKeyEnvVar:       &apiKeyEnvVar,
		BaseURL:            &baseURL,
		MaxToolsPerRequest: &maxTools,
		FilterPrompt:       &filterPrompt,
	}
	if req.EnabledTags != nil {
		enabledTags := append([]string(nil), req.EnabledTags...)
		update.EnabledTags = &enabledTags
	}
	if req.APIKey != nil {
		apiKey := *req.APIKey
		update.APIKey = &apiKey
	}

	if err := editor.UpdateSubAgentConfig(ctx, update); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// IsSubAgentAvailable returns whether SubAgent infrastructure is configured.
func (s *SubAgentService) IsSubAgentAvailable(ctx context.Context) bool {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return false
	}
	return cp.IsSubAgentEnabled()
}
