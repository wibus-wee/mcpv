package normalizer

import (
	"net/http"
	"sort"
	"strings"

	"mcpv/internal/domain"
)

func NormalizeServerSpec(raw RawServerSpec) (domain.ServerSpec, bool) {
	strategy := domain.InstanceStrategy(raw.Strategy)
	if strategy == "" {
		strategy = domain.DefaultStrategy
	}
	activationMode := strings.ToLower(strings.TrimSpace(raw.ActivationMode))
	transport := domain.NormalizeTransport(domain.TransportKind(raw.Transport))
	implicitHTTP := false
	if transport == domain.TransportStdio && strings.TrimSpace(raw.Transport) == "" {
		if strings.TrimSpace(raw.HTTP.Endpoint) != "" || len(raw.HTTP.Headers) > 0 || raw.HTTP.MaxRetries != nil {
			transport = domain.TransportStreamableHTTP
			implicitHTTP = true
		}
	}
	httpConfig := normalizeStreamableHTTPConfig(raw.HTTP, transport)

	spec := domain.ServerSpec{
		Name:                raw.Name,
		Transport:           transport,
		Cmd:                 raw.Cmd,
		Env:                 raw.Env,
		Cwd:                 raw.Cwd,
		Tags:                NormalizeTags(raw.Tags),
		IdleSeconds:         raw.IdleSeconds,
		MaxConcurrent:       raw.MaxConcurrent,
		Strategy:            strategy,
		Disabled:            raw.Disabled,
		MinReady:            raw.MinReady,
		ActivationMode:      domain.ActivationMode(activationMode),
		DrainTimeoutSeconds: raw.DrainTimeoutSeconds,
		ProtocolVersion:     raw.ProtocolVersion,
		ExposeTools:         raw.ExposeTools,
		HTTP:                httpConfig,
	}
	if raw.SessionTTLSeconds != nil {
		spec.SessionTTLSeconds = *raw.SessionTTLSeconds
	}
	if spec.ProtocolVersion == "" {
		if transport == domain.TransportStreamableHTTP {
			spec.ProtocolVersion = domain.DefaultStreamableHTTPProtocolVersion
		} else {
			spec.ProtocolVersion = domain.DefaultProtocolVersion
		}
	}
	if spec.MaxConcurrent == 0 {
		spec.MaxConcurrent = domain.DefaultMaxConcurrent
	}
	if spec.DrainTimeoutSeconds == 0 {
		spec.DrainTimeoutSeconds = domain.DefaultDrainTimeoutSeconds
	}
	if spec.Strategy == domain.StrategyStateful && raw.SessionTTLSeconds == nil {
		spec.SessionTTLSeconds = domain.DefaultSessionTTLSeconds
	}
	return spec, implicitHTTP
}

func normalizeStreamableHTTPConfig(raw RawStreamableHTTPConfig, transport domain.TransportKind) *domain.StreamableHTTPConfig {
	if domain.NormalizeTransport(transport) != domain.TransportStreamableHTTP {
		return nil
	}

	maxRetries := domain.DefaultStreamableHTTPMaxRetries
	if raw.MaxRetries != nil {
		maxRetries = *raw.MaxRetries
	}

	headers := normalizeHTTPHeaders(raw.Headers)
	proxy := normalizeServerProxyConfig(raw.Proxy)

	return &domain.StreamableHTTPConfig{
		Endpoint:   strings.TrimSpace(raw.Endpoint),
		Headers:    headers,
		MaxRetries: maxRetries,
		Proxy:      proxy,
	}
}

func normalizeHTTPHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	normalized := make(map[string]string, len(headers))
	for _, key := range keys {
		trimmedKey := strings.TrimSpace(key)
		value := strings.TrimSpace(headers[key])
		if trimmedKey == "" {
			normalized[""] = value
			continue
		}
		normalized[http.CanonicalHeaderKey(trimmedKey)] = value
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func NormalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	unique := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		unique[tag] = struct{}{}
	}

	if len(unique) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(unique))
	for tag := range unique {
		normalized = append(normalized, tag)
	}
	sort.Strings(normalized)
	return normalized
}

func NormalizeEnvMap(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	cleaned := make(map[string]string, len(env))
	for key, value := range env {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		cleaned[key] = value
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}
