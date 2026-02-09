package mapping

import (
	"strings"

	"mcpv/internal/domain"
	"mcpv/internal/ui/transfer"
	"mcpv/internal/ui/types"
)

func MapTransferIssues(issues []transfer.Issue) []types.McpTransferIssue {
	out := make([]types.McpTransferIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, types.McpTransferIssue{
			Name:    issue.Name,
			Kind:    issue.Kind,
			Message: issue.Message,
		})
	}
	return out
}

func MapDomainToImportSpec(spec domain.ServerSpec) types.ImportServerSpec {
	env := spec.Env
	if env == nil {
		env = map[string]string{}
	}
	return types.ImportServerSpec{
		Name:            spec.Name,
		Transport:       string(domain.NormalizeTransport(spec.Transport)),
		Cmd:             append([]string(nil), spec.Cmd...),
		Env:             copyStringMap(env),
		Cwd:             spec.Cwd,
		Tags:            append([]string(nil), spec.Tags...),
		ProtocolVersion: spec.ProtocolVersion,
		HTTP:            mapStreamableHTTPDetail(spec.HTTP, spec.Transport),
	}
}

func MapImportToDomainSpec(spec types.ImportServerSpec) domain.ServerSpec {
	return domain.ServerSpec{
		Name:            strings.TrimSpace(spec.Name),
		Transport:       domain.TransportKind(strings.TrimSpace(spec.Transport)),
		Cmd:             append([]string(nil), spec.Cmd...),
		Env:             copyStringMap(spec.Env),
		Cwd:             strings.TrimSpace(spec.Cwd),
		Tags:            append([]string(nil), spec.Tags...),
		ProtocolVersion: strings.TrimSpace(spec.ProtocolVersion),
		HTTP:            mapStreamableHTTPConfig(spec.HTTP),
	}
}

func mapStreamableHTTPConfig(cfg *types.StreamableHTTPConfigDetail) *domain.StreamableHTTPConfig {
	if cfg == nil {
		return nil
	}
	headers := cfg.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	return &domain.StreamableHTTPConfig{
		Endpoint:   cfg.Endpoint,
		Headers:    copyStringMap(headers),
		MaxRetries: cfg.MaxRetries,
	}
}

func mapStreamableHTTPDetail(cfg *domain.StreamableHTTPConfig, transport domain.TransportKind) *types.StreamableHTTPConfigDetail {
	if cfg == nil || domain.NormalizeTransport(transport) != domain.TransportStreamableHTTP {
		return nil
	}
	headers := cfg.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	return &types.StreamableHTTPConfigDetail{
		Endpoint:   cfg.Endpoint,
		Headers:    copyStringMap(headers),
		MaxRetries: cfg.MaxRetries,
	}
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
