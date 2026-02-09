package services

import (
	"strings"

	"mcpv/internal/domain"
)

func filterTransferSpecs(specs []domain.ServerSpec, existing map[string]struct{}) ([]domain.ServerSpec, []McpTransferIssue) {
	if len(specs) == 0 {
		return nil, nil
	}
	if existing == nil {
		existing = map[string]struct{}{}
	}
	seen := make(map[string]struct{}, len(specs))
	filtered := make([]domain.ServerSpec, 0, len(specs))
	issues := make([]McpTransferIssue, 0)

	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			issues = append(issues, McpTransferIssue{
				Kind:    "invalid",
				Message: "server name is required",
			})
			continue
		}
		if _, ok := seen[name]; ok {
			issues = append(issues, McpTransferIssue{
				Name:    name,
				Kind:    "duplicate",
				Message: "duplicate server name in import list",
			})
			continue
		}
		seen[name] = struct{}{}
		if _, ok := existing[name]; ok {
			issues = append(issues, McpTransferIssue{
				Name:    name,
				Kind:    "conflict",
				Message: "server already exists in current config",
			})
			continue
		}
		if name != spec.Name {
			spec.Name = name
		}
		filtered = append(filtered, spec)
	}
	return filtered, issues
}
