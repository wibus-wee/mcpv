package ui

import (
	"context"
	"fmt"

	"go.uber.org/zap"
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

	store := cp.GetProfileStore()
	for _, profile := range store.Profiles {
		cfg := profile.Catalog.Runtime.SubAgent
		return SubAgentConfigDetail{
			Model:              cfg.Model,
			Provider:           cfg.Provider,
			APIKeyEnvVar:       cfg.APIKeyEnvVar,
			BaseURL:            cfg.BaseURL,
			MaxToolsPerRequest: cfg.MaxToolsPerRequest,
			FilterPrompt:       cfg.FilterPrompt,
		}, nil
	}

	return SubAgentConfigDetail{}, nil
}

// GetProfileSubAgentConfig returns the per-profile SubAgent enabled state.
func (s *SubAgentService) GetProfileSubAgentConfig(ctx context.Context, profileName string) (ProfileSubAgentConfigDetail, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return ProfileSubAgentConfigDetail{}, err
	}

	store := cp.GetProfileStore()
	profile, ok := store.Profiles[profileName]
	if !ok {
		return ProfileSubAgentConfigDetail{}, NewUIError(ErrCodeNotFound, fmt.Sprintf("Profile %q not found", profileName))
	}

	return ProfileSubAgentConfigDetail{Enabled: profile.Catalog.SubAgent.Enabled}, nil
}

// SetProfileSubAgentEnabled updates the per-profile SubAgent enabled state.
func (s *SubAgentService) SetProfileSubAgentEnabled(ctx context.Context, req UpdateProfileSubAgentRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.SetProfileSubAgentEnabled(ctx, req.Profile, req.Enabled); err != nil {
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
