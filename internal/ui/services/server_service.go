package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/ui"
	"mcpv/internal/ui/mapping"
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
func (s *ServerService) ListServers(_ context.Context) ([]ServerSummary, error) {
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
		specKey := domain.SpecFingerprint(spec)
		servers = append(servers, mapping.MapServerSummary(spec, specKey))
	}
	return servers, nil
}

// GetServer returns server detail by name.
func (s *ServerService) GetServer(_ context.Context, name string) (*ServerDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ui.NewError(ui.ErrCodeInvalidRequest, "Server name is required")
	}
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}
	spec, ok := cp.GetCatalog().Specs[name]
	if !ok {
		return nil, ui.NewError(ui.ErrCodeNotFound, fmt.Sprintf("Server %q not found", name))
	}
	specKey := domain.SpecFingerprint(spec)
	detail := mapping.MapServerSpecDetail(spec, specKey)
	return &detail, nil
}

// ListServerGroups returns aggregated server groups with tool metadata.
func (s *ServerService) ListServerGroups(ctx context.Context) ([]ServerGroup, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	toolCatalog, err := cp.ListToolCatalog(ctx)
	if err != nil {
		return nil, ui.MapDomainError(err)
	}

	tools, err := mapping.MapToolCatalogEntries(toolCatalog)
	if err != nil {
		return nil, ui.NewErrorWithDetails(ui.ErrCodeInternal, "Failed to map tools", err.Error())
	}

	toolsBySpecKey := make(map[string][]ToolEntry, len(tools))
	for _, tool := range tools {
		specKey := strings.TrimSpace(tool.SpecKey)
		if specKey == "" {
			specKey = strings.TrimSpace(tool.ServerName)
		}
		if specKey == "" {
			specKey = strings.TrimSpace(tool.Name)
		}
		if specKey == "" {
			continue
		}
		toolsBySpecKey[specKey] = append(toolsBySpecKey[specKey], tool)
	}

	catalog := cp.GetCatalog()
	names := make([]string, 0, len(catalog.Specs))
	for name := range catalog.Specs {
		names = append(names, name)
	}
	sort.Strings(names)

	serverGroups := make([]ServerGroup, 0, len(names)+len(toolsBySpecKey))
	serverMap := make(map[string]*ServerGroup, len(names))

	ensureServer := func(specKey string, serverName string, detail *ServerDetail, tags []string) *ServerGroup {
		if specKey == "" {
			return nil
		}
		if existing := serverMap[specKey]; existing != nil {
			if existing.ServerName == "" && serverName != "" {
				existing.ServerName = serverName
			}
			if existing.SpecDetail == nil && detail != nil {
				existing.SpecDetail = detail
			}
			if len(existing.Tags) == 0 && len(tags) > 0 {
				existing.Tags = append([]string(nil), tags...)
			}
			if existing.Tools == nil {
				existing.Tools = toolsBySpecKey[specKey]
			}
			existing.HasToolData = len(existing.Tools) > 0
			return existing
		}

		toolsForServer := toolsBySpecKey[specKey]
		group := ServerGroup{
			ID:          specKey,
			SpecKey:     specKey,
			ServerName:  serverName,
			Tools:       toolsForServer,
			Tags:        append([]string(nil), tags...),
			HasToolData: len(toolsForServer) > 0,
			SpecDetail:  detail,
		}
		if group.ServerName == "" {
			group.ServerName = specKey
		}
		serverGroups = append(serverGroups, group)
		serverMap[specKey] = &serverGroups[len(serverGroups)-1]
		return serverMap[specKey]
	}

	for _, name := range names {
		spec := catalog.Specs[name]
		specKey := domain.SpecFingerprint(spec)
		detail := mapping.MapServerSpecDetail(spec, specKey)
		ensureServer(specKey, spec.Name, &detail, spec.Tags)
	}

	toolSpecKeys := make([]string, 0, len(toolsBySpecKey))
	for specKey := range toolsBySpecKey {
		if _, exists := serverMap[specKey]; !exists {
			toolSpecKeys = append(toolSpecKeys, specKey)
		}
	}
	sort.Strings(toolSpecKeys)
	for _, specKey := range toolSpecKeys {
		serverName := ""
		for _, tool := range toolsBySpecKey[specKey] {
			if strings.TrimSpace(tool.ServerName) != "" {
				serverName = tool.ServerName
				break
			}
		}
		ensureServer(specKey, serverName, nil, nil)
	}

	return serverGroups, nil
}

// CreateServer adds a server to the config file.
func (s *ServerService) CreateServer(ctx context.Context, req CreateServerRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	spec := mapping.MapServerSpecDetailToDomain(req.Spec)
	if err := editor.CreateServer(ctx, spec); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// UpdateServer updates an existing server in the config file.
func (s *ServerService) UpdateServer(ctx context.Context, req UpdateServerRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	spec := mapping.MapServerSpecDetailToDomain(req.Spec)
	if err := editor.UpdateServer(ctx, spec); err != nil {
		return mapCatalogError(err)
	}
	return nil
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
