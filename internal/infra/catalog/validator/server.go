package validator

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"mcpv/internal/domain"
)

func ValidateServerSpec(spec domain.ServerSpec, index int) []string {
	var errs []string

	if spec.Name == "" {
		errs = append(errs, fmt.Sprintf("servers[%d]: name is required", index))
	}
	transport := domain.NormalizeTransport(spec.Transport)
	switch transport {
	case domain.TransportStdio:
		if len(spec.Cmd) == 0 {
			errs = append(errs, fmt.Sprintf("servers[%d]: cmd is required", index))
		}
	case domain.TransportStreamableHTTP:
		if len(spec.Cmd) > 0 {
			errs = append(errs, fmt.Sprintf("servers[%d]: cmd must be empty for streamable_http transport (external connection)", index))
		}
		if spec.Cwd != "" {
			errs = append(errs, fmt.Sprintf("servers[%d]: cwd must be empty for streamable_http transport (external connection)", index))
		}
		if len(spec.Env) > 0 {
			errs = append(errs, fmt.Sprintf("servers[%d]: env must be empty for streamable_http transport (external connection)", index))
		}
	default:
		errs = append(errs, fmt.Sprintf("servers[%d]: transport must be stdio or streamable_http", index))
	}
	if spec.MaxConcurrent < 1 {
		errs = append(errs, fmt.Sprintf("servers[%d]: maxConcurrent must be >= 1", index))
	}
	if spec.IdleSeconds < 0 {
		errs = append(errs, fmt.Sprintf("servers[%d]: idleSeconds must be >= 0", index))
	}
	if spec.MinReady < 0 {
		errs = append(errs, fmt.Sprintf("servers[%d]: minReady must be >= 0", index))
	}
	if spec.ActivationMode != "" && spec.ActivationMode != domain.ActivationOnDemand && spec.ActivationMode != domain.ActivationAlwaysOn {
		errs = append(errs, fmt.Sprintf("servers[%d]: activationMode must be on-demand or always-on", index))
	}

	switch spec.Strategy {
	case domain.StrategyStateless, domain.StrategyStateful, domain.StrategyPersistent, domain.StrategySingleton:
		// valid
	default:
		errs = append(errs, fmt.Sprintf("servers[%d]: strategy must be one of: stateless, stateful, persistent, singleton", index))
	}

	if spec.Strategy == domain.StrategyStateful && spec.SessionTTLSeconds < 0 {
		errs = append(errs, fmt.Sprintf("servers[%d]: sessionTTLSeconds must be >= 0 for stateful strategy", index))
	}

	versionPattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if spec.ProtocolVersion == "" {
		errs = append(errs, fmt.Sprintf("servers[%d]: protocolVersion is required", index))
	} else {
		if !versionPattern.MatchString(spec.ProtocolVersion) {
			errs = append(errs, fmt.Sprintf("servers[%d]: protocolVersion must match YYYY-MM-DD", index))
		}
		if !domain.IsSupportedProtocolVersion(transport, spec.ProtocolVersion) {
			if transport == domain.TransportStreamableHTTP {
				errs = append(errs, fmt.Sprintf("servers[%d]: protocolVersion must be one of %s for streamable_http transport", index, strings.Join(domain.StreamableHTTPProtocolVersions, ", ")))
			} else {
				errs = append(errs, fmt.Sprintf("servers[%d]: protocolVersion must be %s", index, domain.DefaultProtocolVersion))
			}
		}
	}

	for i, tool := range spec.ExposeTools {
		if strings.TrimSpace(tool) == "" {
			errs = append(errs, fmt.Sprintf("servers[%d]: exposeTools[%d] must not be empty", index, i))
		}
	}

	if transport == domain.TransportStreamableHTTP {
		errs = append(errs, validateStreamableHTTPSpec(spec, index)...)
	}

	return errs
}

func validateStreamableHTTPSpec(spec domain.ServerSpec, index int) []string {
	var errs []string

	if spec.HTTP == nil {
		return append(errs, fmt.Sprintf("servers[%d]: http config is required for streamable_http transport", index))
	}
	endpoint := strings.TrimSpace(spec.HTTP.Endpoint)
	if endpoint == "" {
		errs = append(errs, fmt.Sprintf("servers[%d]: http.endpoint is required for streamable_http transport", index))
	} else {
		if strings.Contains(endpoint, " ") {
			errs = append(errs, fmt.Sprintf("servers[%d]: http.endpoint must be a valid http(s) URL", index))
		} else if parsed, err := url.ParseRequestURI(endpoint); err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			errs = append(errs, fmt.Sprintf("servers[%d]: http.endpoint must be a valid http(s) URL", index))
		}
	}

	if spec.HTTP.MaxRetries < -1 {
		errs = append(errs, fmt.Sprintf("servers[%d]: http.maxRetries must be >= -1 (-1 disables retries)", index))
	}

	for key, value := range spec.HTTP.Headers {
		name := strings.TrimSpace(key)
		if name == "" {
			errs = append(errs, fmt.Sprintf("servers[%d]: http.headers contains empty header name", index))
			continue
		}
		if isReservedHTTPHeader(name) {
			errs = append(errs, fmt.Sprintf("servers[%d]: http.headers.%s is reserved and managed by transport", index, name))
		}
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Sprintf("servers[%d]: http.headers.%s must not be empty", index, name))
		}
	}

	if spec.HTTP.Proxy != nil {
		errs = append(errs, validateHTTPProxyConfig(spec.HTTP.Proxy, index)...)
	}

	return errs
}

func isReservedHTTPHeader(header string) bool {
	switch strings.ToLower(strings.TrimSpace(header)) {
	case "content-type", "accept", "mcp-protocol-version", "mcp-session-id", "last-event-id",
		"host", "content-length", "transfer-encoding", "connection":
		return true
	default:
		return false
	}
}

func validateHTTPProxyConfig(proxy *domain.ProxyConfig, index int) []string {
	if proxy == nil {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(string(proxy.Mode)))
	if mode == "" {
		mode = string(domain.ProxyModeInherit)
	}

	switch mode {
	case string(domain.ProxyModeInherit),
		string(domain.ProxyModeDisabled),
		string(domain.ProxyModeCustom),
		string(domain.ProxyModeSystem):
		// valid
	default:
		return []string{fmt.Sprintf("servers[%d]: http.proxy.mode must be inherit, disabled, custom, or system", index)}
	}

	if mode == string(domain.ProxyModeCustom) {
		if strings.TrimSpace(proxy.URL) == "" {
			return []string{fmt.Sprintf("servers[%d]: http.proxy.url is required when proxy.mode is custom", index)}
		}
		if err := validateProxyURL(proxy.URL); err != nil {
			return []string{fmt.Sprintf("servers[%d]: http.proxy.url must be a valid proxy URL (%v)", index, err)}
		}
	}
	return nil
}

func validateProxyURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("empty URL")
	}
	if strings.Contains(trimmed, " ") {
		return fmt.Errorf("URL contains spaces")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("scheme and host are required")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return nil
	default:
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
}
