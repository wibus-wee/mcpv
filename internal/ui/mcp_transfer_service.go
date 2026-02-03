package ui

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/catalog"
	"mcpv/internal/ui/transfer"
)

// McpTransferService exposes MCP transfer APIs for Wails.
type McpTransferService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

// NewMcpTransferService constructs a McpTransferService.
func NewMcpTransferService(deps *ServiceDeps) *McpTransferService {
	return &McpTransferService{
		deps:   deps,
		logger: deps.loggerNamed("mcp-transfer-service"),
	}
}

// Preview returns importable MCP servers from an external config source.
func (s *McpTransferService) Preview(ctx context.Context, req McpTransferRequest) (McpTransferPreview, error) {
	source, err := transfer.ParseSource(req.Source)
	if err != nil {
		return McpTransferPreview{}, NewError(ErrCodeInvalidRequest, "Unsupported transfer source")
	}

	result, err := transfer.ReadSource(source)
	if err != nil {
		switch {
		case errors.Is(err, transfer.ErrNotFound):
			return McpTransferPreview{}, NewError(ErrCodeNotFound, "Source config not found")
		case errors.Is(err, transfer.ErrUnknownSource):
			return McpTransferPreview{}, NewError(ErrCodeInvalidRequest, "Unsupported transfer source")
		default:
			return McpTransferPreview{}, NewErrorWithDetails(ErrCodeInvalidConfig, "Failed to parse source config", err.Error())
		}
	}

	existing, err := s.loadExistingServerNames(ctx)
	if err != nil {
		return McpTransferPreview{}, err
	}

	filtered, filterIssues := filterTransferSpecs(result.Servers, existing)
	issues := append(mapTransferIssues(result.Issues), filterIssues...)

	servers := make([]ImportServerSpec, 0, len(filtered))
	for _, spec := range filtered {
		servers = append(servers, mapDomainToImportSpec(spec))
	}

	return McpTransferPreview{
		Source:  string(source),
		Path:    result.Path,
		Servers: servers,
		Issues:  issues,
	}, nil
}

// Import reads and imports MCP servers from an external config source.
func (s *McpTransferService) Import(ctx context.Context, req McpTransferRequest) (McpTransferImportResult, error) {
	preview, err := s.Preview(ctx, req)
	if err != nil {
		return McpTransferImportResult{}, err
	}

	result := McpTransferImportResult{
		Source:  preview.Source,
		Path:    preview.Path,
		Issues:  preview.Issues,
		Skipped: len(preview.Issues),
	}

	if len(preview.Servers) == 0 {
		return result, nil
	}

	editor, err := s.deps.catalogEditor()
	if err != nil {
		return McpTransferImportResult{}, err
	}

	importSpecs := make([]domain.ServerSpec, 0, len(preview.Servers))
	for _, server := range preview.Servers {
		importSpecs = append(importSpecs, mapImportToDomainSpec(server))
	}

	if err := editor.ImportServers(ctx, catalog.ImportRequest{Servers: importSpecs}); err != nil {
		return McpTransferImportResult{}, mapCatalogError(err)
	}

	result.Imported = len(importSpecs)
	return result, nil
}

func (s *McpTransferService) loadExistingServerNames(ctx context.Context) (map[string]struct{}, error) {
	manager := s.deps.manager()
	if manager == nil {
		return nil, NewError(ErrCodeInternal, "Manager not initialized")
	}
	path := strings.TrimSpace(manager.GetConfigPath())
	if path == "" {
		return nil, NewError(ErrCodeInvalidConfig, "Configuration path is not available")
	}

	loader := catalog.NewLoader(s.logger)
	catalogState, err := loader.Load(ctx, path)
	if err != nil {
		return nil, NewErrorWithDetails(ErrCodeInvalidConfig, "Failed to load config", err.Error())
	}

	existing := make(map[string]struct{}, len(catalogState.Specs))
	for name := range catalogState.Specs {
		existing[name] = struct{}{}
	}
	return existing, nil
}
