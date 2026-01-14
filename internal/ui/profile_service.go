package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.uber.org/zap"

	"mcpd/internal/domain"
)

// ProfileService exposes profile and server mutation APIs.
type ProfileService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewProfileService(deps *ServiceDeps) *ProfileService {
	return &ProfileService{
		deps:   deps,
		logger: deps.loggerNamed("profile-service"),
	}
}

// SetServerDisabled updates the disabled state for a server in a profile.
func (s *ProfileService) SetServerDisabled(ctx context.Context, req UpdateServerStateRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.SetServerDisabled(ctx, req.Profile, req.Server, req.Disabled); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// DeleteServer removes a server from a profile.
func (s *ProfileService) DeleteServer(ctx context.Context, req DeleteServerRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.DeleteServer(ctx, req.Profile, req.Server); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// CreateProfile creates a new profile file in the profile store.
func (s *ProfileService) CreateProfile(ctx context.Context, req CreateProfileRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.CreateProfile(ctx, req.Name); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// DeleteProfile deletes a profile file from the profile store.
func (s *ProfileService) DeleteProfile(ctx context.Context, req DeleteProfileRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.DeleteProfile(ctx, req.Name); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// SetCallerMapping updates a caller to profile mapping.
func (s *ProfileService) SetCallerMapping(ctx context.Context, req UpdateCallerMappingRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.SetCallerMapping(ctx, req.Caller, req.Profile); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// RemoveCallerMapping removes a caller to profile mapping.
func (s *ProfileService) RemoveCallerMapping(ctx context.Context, caller string) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.RemoveCallerMapping(ctx, caller); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// ListProfiles lists all profiles.
func (s *ProfileService) ListProfiles(ctx context.Context) ([]ProfileSummary, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	store := cp.GetProfileStore()

	names := make([]string, 0, len(store.Profiles))
	for name := range store.Profiles {
		names = append(names, name)
	}

	slices.SortStableFunc(names, func(a, b string) int {
		if a == domain.DefaultProfileName {
			return -1
		}
		if b == domain.DefaultProfileName {
			return 1
		}
		return strings.Compare(a, b)
	})

	result := make([]ProfileSummary, 0, len(store.Profiles))
	for _, name := range names {
		profile := store.Profiles[name]
		result = append(result, ProfileSummary{
			Name:        name,
			ServerCount: len(profile.Catalog.Specs),
			IsDefault:   name == domain.DefaultProfileName,
		})
	}

	return result, nil
}

// GetProfile returns profile detail by name.
func (s *ProfileService) GetProfile(ctx context.Context, name string) (*ProfileDetail, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	store := cp.GetProfileStore()
	profile, ok := store.Profiles[name]
	if !ok {
		return nil, NewUIError(ErrCodeNotFound, fmt.Sprintf("Profile %q not found", name))
	}

	servers := make([]ServerSpecDetail, 0, len(profile.Catalog.Specs))
	for _, spec := range profile.Catalog.Specs {
		specKey, err := domain.SpecFingerprint(spec)
		if err != nil {
			return nil, NewUIError(ErrCodeInternal, fmt.Sprintf("spec fingerprint for %q: %v", spec.Name, err))
		}
		servers = append(servers, mapServerSpecDetail(spec, specKey))
	}

	return &ProfileDetail{
		Name:    profile.Name,
		Runtime: mapRuntimeConfigDetail(profile.Catalog.Runtime),
		Servers: servers,
		SubAgent: ProfileSubAgentConfigDetail{
			Enabled: profile.Catalog.SubAgent.Enabled,
		},
	}, nil
}

// GetCallers returns caller to profile mapping.
func (s *ProfileService) GetCallers(ctx context.Context) (map[string]string, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	store := cp.GetProfileStore()
	result := make(map[string]string, len(store.Callers))
	for k, v := range store.Callers {
		result[k] = v
	}

	return result, nil
}
