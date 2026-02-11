package normalizer

import (
	"fmt"
	"net/url"
	"strings"

	"mcpv/internal/domain"
)

var allowedProxySchemes = map[string]struct{}{
	"http":    {},
	"https":   {},
	"socks5":  {},
	"socks5h": {},
}

func normalizeRuntimeProxyConfig(raw RawProxyConfig) (domain.ProxyConfig, []string) {
	var errs []string
	mode := normalizeProxyMode(raw.Mode)
	if mode == "" {
		mode = string(domain.ProxyModeSystem)
	}

	switch mode {
	case string(domain.ProxyModeSystem), string(domain.ProxyModeCustom), string(domain.ProxyModeDisabled):
		// valid
	default:
		errs = append(errs, "proxy.mode must be system, custom, or disabled")
	}

	urlValue := strings.TrimSpace(raw.URL)
	noProxy := strings.TrimSpace(raw.NoProxy)
	if mode == string(domain.ProxyModeCustom) {
		if urlValue == "" {
			errs = append(errs, "proxy.url is required when proxy.mode is custom")
		} else if err := validateProxyURL(urlValue); err != nil {
			errs = append(errs, fmt.Sprintf("proxy.url must be a valid proxy URL (%v)", err))
		}
	}

	return domain.ProxyConfig{
		Mode:    domain.ProxyMode(mode),
		URL:     urlValue,
		NoProxy: noProxy,
	}, errs
}

func normalizeServerProxyConfig(raw RawProxyConfig) *domain.ProxyConfig {
	if isRawProxyEmpty(raw) {
		return nil
	}

	mode := normalizeProxyMode(raw.Mode)
	urlValue := strings.TrimSpace(raw.URL)
	noProxy := strings.TrimSpace(raw.NoProxy)
	if mode == "" {
		if urlValue != "" {
			mode = string(domain.ProxyModeCustom)
		} else {
			mode = string(domain.ProxyModeInherit)
		}
	}

	return &domain.ProxyConfig{
		Mode:    domain.ProxyMode(mode),
		URL:     urlValue,
		NoProxy: noProxy,
	}
}

func ResolveStreamableHTTPProxy(runtime domain.ProxyConfig, override *domain.ProxyConfig) *domain.ProxyConfig {
	base := copyProxyConfig(runtime)
	if override == nil {
		return &base
	}

	mode := override.Mode
	if mode == "" {
		mode = domain.ProxyModeInherit
	}

	noProxy := strings.TrimSpace(override.NoProxy)
	switch mode {
	case domain.ProxyModeDisabled:
		return &domain.ProxyConfig{
			Mode:    domain.ProxyModeDisabled,
			NoProxy: noProxy,
		}
	case domain.ProxyModeCustom:
		return &domain.ProxyConfig{
			Mode:    domain.ProxyModeCustom,
			URL:     strings.TrimSpace(override.URL),
			NoProxy: noProxy,
		}
	case domain.ProxyModeSystem:
		return &domain.ProxyConfig{
			Mode:    domain.ProxyModeSystem,
			NoProxy: noProxy,
		}
	case domain.ProxyModeInherit:
		if noProxy != "" {
			base.NoProxy = noProxy
		}
		return &base
	default:
		return &base
	}
}

func ApplyRuntimeProxyToSpecs(runtime domain.RuntimeConfig, specs map[string]domain.ServerSpec) {
	if len(specs) == 0 {
		return
	}

	for name, spec := range specs {
		if domain.NormalizeTransport(spec.Transport) != domain.TransportStreamableHTTP || spec.HTTP == nil {
			continue
		}
		spec.HTTP.EffectiveProxy = ResolveStreamableHTTPProxy(runtime.Proxy, spec.HTTP.Proxy)
		specs[name] = spec
	}
}

func normalizeProxyMode(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isRawProxyEmpty(raw RawProxyConfig) bool {
	return strings.TrimSpace(raw.Mode) == "" &&
		strings.TrimSpace(raw.URL) == "" &&
		strings.TrimSpace(raw.NoProxy) == ""
}

func copyProxyConfig(cfg domain.ProxyConfig) domain.ProxyConfig {
	return domain.ProxyConfig{
		Mode:    cfg.Mode,
		URL:     cfg.URL,
		NoProxy: cfg.NoProxy,
	}
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
	scheme := strings.ToLower(parsed.Scheme)
	if _, ok := allowedProxySchemes[scheme]; !ok {
		return fmt.Errorf("unsupported scheme %q", scheme)
	}
	return nil
}
