package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"

	"mcpd/internal/domain"
)

// ServerService exposes server configuration APIs.
type ServerService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewServerService(deps *ServiceDeps) *ServerService {
	return &ServerService{
		deps:   deps,
		logger: deps.loggerNamed("server-service"),
	}
}

// ListServers returns all configured servers.
func (s *ServerService) ListServers(ctx context.Context) ([]ServerSummary, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}
	catalog := cp.GetCatalog()
	if len(catalog.Specs) == 0 {
		return []ServerSummary{}, nil
	}

	names := make([]string, 0, len(catalog.Specs))
	for name := range catalog.Specs {
		names = append(names, name)
	}
	sort.Strings(names)

	servers := make([]ServerSummary, 0, len(names))
	for _, name := range names {
		spec := catalog.Specs[name]
		specKey, err := domain.SpecFingerprint(spec)
		if err != nil {
			return nil, NewUIError(ErrCodeInternal, fmt.Sprintf("spec fingerprint for %q: %v", spec.Name, err))
		}
		servers = append(servers, mapServerSummary(spec, specKey))
	}
	return servers, nil
}

// GetServer returns server detail by name.
func (s *ServerService) GetServer(ctx context.Context, name string) (*ServerDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, NewUIError(ErrCodeInvalidRequest, "Server name is required")
	}
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}
	spec, ok := cp.GetCatalog().Specs[name]
	if !ok {
		return nil, NewUIError(ErrCodeNotFound, fmt.Sprintf("Server %q not found", name))
	}
	specKey, err := domain.SpecFingerprint(spec)
	if err != nil {
		return nil, NewUIError(ErrCodeInternal, fmt.Sprintf("spec fingerprint for %q: %v", spec.Name, err))
	}
	detail := mapServerSpecDetail(spec, specKey)
	return &detail, nil
}

// SetServerDisabled updates the disabled state for a server.
func (s *ServerService) SetServerDisabled(ctx context.Context, req UpdateServerStateRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.SetServerDisabled(ctx, req.Server, req.Disabled); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// DeleteServer removes a server from the config file.
func (s *ServerService) DeleteServer(ctx context.Context, req DeleteServerRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.DeleteServer(ctx, req.Server); err != nil {
		return mapCatalogError(err)
	}
	return nil
}
