package normalizer

import (
	"encoding/json"
	"fmt"
	"strings"

	"mcpv/internal/domain"
)

func NormalizePluginSpecs(raw []RawPluginSpec) ([]domain.PluginSpec, []string) {
	if len(raw) == 0 {
		return nil, nil
	}

	plugins := make([]domain.PluginSpec, 0, len(raw))
	var errs []string
	nameSeen := make(map[string]struct{}, len(raw))

	for i, entry := range raw {
		normalized, entryErrs := normalizePluginSpec(entry, i)
		if len(entryErrs) > 0 {
			errs = append(errs, entryErrs...)
			continue
		}
		if _, exists := nameSeen[normalized.Name]; exists {
			errs = append(errs, fmt.Sprintf("plugins[%d]: duplicate name %q", i, normalized.Name))
			continue
		}
		nameSeen[normalized.Name] = struct{}{}
		plugins = append(plugins, normalized)
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return plugins, nil
}

func normalizePluginSpec(raw RawPluginSpec, index int) (domain.PluginSpec, []string) {
	var errs []string
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		errs = append(errs, fmt.Sprintf("plugins[%d]: name is required", index))
	}
	category, ok := domain.NormalizePluginCategory(raw.Category)
	if !ok {
		errs = append(errs, fmt.Sprintf("plugins[%d]: category must be one of: observability, authentication, authorization, rate_limiting, validation, content, audit", index))
	}

	required := true
	if raw.Required != nil {
		required = *raw.Required
	}

	flows, ok := domain.NormalizePluginFlows(raw.Flows)
	if !ok {
		errs = append(errs, fmt.Sprintf("plugins[%d]: flows must contain request and/or response", index))
	}

	timeoutMs := 0
	if raw.TimeoutMs != nil {
		timeoutMs = *raw.TimeoutMs
	}
	if timeoutMs < 0 {
		errs = append(errs, fmt.Sprintf("plugins[%d]: timeoutMs must be >= 0", index))
	}

	handshakeTimeoutMs := 0
	if raw.HandshakeTimeoutMs != nil {
		handshakeTimeoutMs = *raw.HandshakeTimeoutMs
	}
	if handshakeTimeoutMs < 0 {
		errs = append(errs, fmt.Sprintf("plugins[%d]: handshakeTimeoutMs must be >= 0", index))
	}

	cmd := raw.Cmd
	if len(cmd) == 0 {
		errs = append(errs, fmt.Sprintf("plugins[%d]: cmd is required", index))
	}

	var configJSON json.RawMessage
	if raw.Config != nil {
		encoded, err := json.Marshal(raw.Config)
		if err != nil {
			errs = append(errs, fmt.Sprintf("plugins[%d]: config must be valid JSON object: %v", index, err))
		} else {
			configJSON = encoded
		}
	}

	if len(errs) > 0 {
		return domain.PluginSpec{}, errs
	}

	return domain.PluginSpec{
		Name:               name,
		Category:           category,
		Required:           required,
		Disabled:           raw.Disabled,
		Cmd:                cmd,
		Env:                NormalizeEnvMap(raw.Env),
		Cwd:                strings.TrimSpace(raw.Cwd),
		CommitHash:         strings.TrimSpace(raw.CommitHash),
		TimeoutMs:          timeoutMs,
		HandshakeTimeoutMs: handshakeTimeoutMs,
		ConfigJSON:         configJSON,
		Flows:              flows,
	}, nil
}
