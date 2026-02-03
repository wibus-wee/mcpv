package ui

import (
	"strings"

	"mcpv/internal/domain"
	"mcpv/internal/ui/transfer"
)

func mapTransferIssues(issues []transfer.Issue) []McpTransferIssue {
	out := make([]McpTransferIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, McpTransferIssue{
			Name:    issue.Name,
			Kind:    issue.Kind,
			Message: issue.Message,
		})
	}
	return out
}

func mapDomainToImportSpec(spec domain.ServerSpec) ImportServerSpec {
	env := spec.Env
	if env == nil {
		env = map[string]string{}
	}
	return ImportServerSpec{
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

func mapImportToDomainSpec(spec ImportServerSpec) domain.ServerSpec {
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

func mapStreamableHTTPDetail(cfg *domain.StreamableHTTPConfig, transport domain.TransportKind) *StreamableHTTPConfigDetail {
	if cfg == nil || domain.NormalizeTransport(transport) != domain.TransportStreamableHTTP {
		return nil
	}
	headers := cfg.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	return &StreamableHTTPConfigDetail{
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
