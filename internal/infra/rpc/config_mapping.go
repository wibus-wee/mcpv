package rpc

import (
	"strings"

	"mcpv/internal/domain"
	catalogeditor "mcpv/internal/infra/catalog/editor"
)

func runtimeConfigToPayload(cfg domain.RuntimeConfig) RuntimeConfigPayload {
	return RuntimeConfigPayload{
		RouteTimeoutSeconds:        cfg.RouteTimeoutSeconds,
		PingIntervalSeconds:        cfg.PingIntervalSeconds,
		ToolRefreshSeconds:         cfg.ToolRefreshSeconds,
		ToolRefreshConcurrency:     cfg.ToolRefreshConcurrency,
		ClientCheckSeconds:         cfg.ClientCheckSeconds,
		ClientInactiveSeconds:      cfg.ClientInactiveSeconds,
		ServerInitRetryBaseSeconds: cfg.ServerInitRetryBaseSeconds,
		ServerInitRetryMaxSeconds:  cfg.ServerInitRetryMaxSeconds,
		ServerInitMaxRetries:       cfg.ServerInitMaxRetries,
		ReloadMode:                 string(cfg.ReloadMode),
		BootstrapMode:              string(cfg.BootstrapMode),
		BootstrapConcurrency:       cfg.BootstrapConcurrency,
		BootstrapTimeoutSeconds:    cfg.BootstrapTimeoutSeconds,
		DefaultActivationMode:      string(cfg.DefaultActivationMode),
		ExposeTools:                cfg.ExposeTools,
		ToolNamespaceStrategy:      string(cfg.ToolNamespaceStrategy),
		Proxy: ProxyConfigPayload{
			Mode:    string(cfg.Proxy.Mode),
			URL:     cfg.Proxy.URL,
			NoProxy: cfg.Proxy.NoProxy,
		},
		Observability: ObservabilityConfigPayload{
			ListenAddress:  cfg.Observability.ListenAddress,
			MetricsEnabled: cfg.Observability.MetricsEnabled,
			HealthzEnabled: cfg.Observability.HealthzEnabled,
		},
		RPC: rpcConfigPayload{
			ListenAddress:           cfg.RPC.ListenAddress,
			MaxRecvMsgSize:          cfg.RPC.MaxRecvMsgSize,
			MaxSendMsgSize:          cfg.RPC.MaxSendMsgSize,
			KeepaliveTimeSeconds:    cfg.RPC.KeepaliveTimeSeconds,
			KeepaliveTimeoutSeconds: cfg.RPC.KeepaliveTimeoutSeconds,
			SocketMode:              cfg.RPC.SocketMode,
			TLS: rpcTLSConfigPayload{
				Enabled:    cfg.RPC.TLS.Enabled,
				CertFile:   cfg.RPC.TLS.CertFile,
				KeyFile:    cfg.RPC.TLS.KeyFile,
				CAFile:     cfg.RPC.TLS.CAFile,
				ClientAuth: cfg.RPC.TLS.ClientAuth,
			},
			Auth: rpcAuthConfigPayload{
				Enabled:  cfg.RPC.Auth.Enabled,
				Mode:     string(cfg.RPC.Auth.Mode),
				Token:    cfg.RPC.Auth.Token,
				TokenEnv: cfg.RPC.Auth.TokenEnv,
			},
		},
	}
}

func runtimeConfigFromPayload(payload RuntimeConfigPayload) domain.RuntimeConfig {
	return domain.RuntimeConfig{
		RouteTimeoutSeconds:        payload.RouteTimeoutSeconds,
		PingIntervalSeconds:        payload.PingIntervalSeconds,
		ToolRefreshSeconds:         payload.ToolRefreshSeconds,
		ToolRefreshConcurrency:     payload.ToolRefreshConcurrency,
		ClientCheckSeconds:         payload.ClientCheckSeconds,
		ClientInactiveSeconds:      payload.ClientInactiveSeconds,
		ServerInitRetryBaseSeconds: payload.ServerInitRetryBaseSeconds,
		ServerInitRetryMaxSeconds:  payload.ServerInitRetryMaxSeconds,
		ServerInitMaxRetries:       payload.ServerInitMaxRetries,
		ReloadMode:                 domain.ReloadMode(strings.TrimSpace(payload.ReloadMode)),
		BootstrapMode:              domain.BootstrapMode(strings.TrimSpace(payload.BootstrapMode)),
		BootstrapConcurrency:       payload.BootstrapConcurrency,
		BootstrapTimeoutSeconds:    payload.BootstrapTimeoutSeconds,
		DefaultActivationMode:      domain.ActivationMode(strings.TrimSpace(payload.DefaultActivationMode)),
		ExposeTools:                payload.ExposeTools,
		ToolNamespaceStrategy:      domain.ToolNamespaceStrategy(strings.TrimSpace(payload.ToolNamespaceStrategy)),
		Proxy: domain.ProxyConfig{
			Mode:    domain.ProxyMode(strings.TrimSpace(payload.Proxy.Mode)),
			URL:     strings.TrimSpace(payload.Proxy.URL),
			NoProxy: strings.TrimSpace(payload.Proxy.NoProxy),
		},
		Observability: domain.ObservabilityConfig{
			ListenAddress:  strings.TrimSpace(payload.Observability.ListenAddress),
			MetricsEnabled: payload.Observability.MetricsEnabled,
			HealthzEnabled: payload.Observability.HealthzEnabled,
		},
		RPC: domain.RPCConfig{
			ListenAddress:           strings.TrimSpace(payload.RPC.ListenAddress),
			MaxRecvMsgSize:          payload.RPC.MaxRecvMsgSize,
			MaxSendMsgSize:          payload.RPC.MaxSendMsgSize,
			KeepaliveTimeSeconds:    payload.RPC.KeepaliveTimeSeconds,
			KeepaliveTimeoutSeconds: payload.RPC.KeepaliveTimeoutSeconds,
			SocketMode:              strings.TrimSpace(payload.RPC.SocketMode),
			TLS: domain.RPCTLSConfig{
				Enabled:    payload.RPC.TLS.Enabled,
				CertFile:   strings.TrimSpace(payload.RPC.TLS.CertFile),
				KeyFile:    strings.TrimSpace(payload.RPC.TLS.KeyFile),
				CAFile:     strings.TrimSpace(payload.RPC.TLS.CAFile),
				ClientAuth: payload.RPC.TLS.ClientAuth,
			},
			Auth: domain.RPCAuthConfig{
				Enabled:  payload.RPC.Auth.Enabled,
				Mode:     domain.RPCAuthMode(strings.TrimSpace(payload.RPC.Auth.Mode)),
				Token:    strings.TrimSpace(payload.RPC.Auth.Token),
				TokenEnv: strings.TrimSpace(payload.RPC.Auth.TokenEnv),
			},
		},
	}
}

func runtimeUpdateFromPayload(payload RuntimeConfigUpdatePayload) catalogeditor.RuntimeConfigUpdate {
	return catalogeditor.RuntimeConfigUpdate{
		RouteTimeoutSeconds:         payload.RouteTimeoutSeconds,
		PingIntervalSeconds:         payload.PingIntervalSeconds,
		ToolRefreshSeconds:          payload.ToolRefreshSeconds,
		ToolRefreshConcurrency:      payload.ToolRefreshConcurrency,
		ClientCheckSeconds:          payload.ClientCheckSeconds,
		ClientInactiveSeconds:       payload.ClientInactiveSeconds,
		ServerInitRetryBaseSeconds:  payload.ServerInitRetryBaseSeconds,
		ServerInitRetryMaxSeconds:   payload.ServerInitRetryMaxSeconds,
		ServerInitMaxRetries:        payload.ServerInitMaxRetries,
		ReloadMode:                  payload.ReloadMode,
		BootstrapMode:               payload.BootstrapMode,
		BootstrapConcurrency:        payload.BootstrapConcurrency,
		BootstrapTimeoutSeconds:     payload.BootstrapTimeoutSeconds,
		DefaultActivationMode:       payload.DefaultActivationMode,
		ExposeTools:                 payload.ExposeTools,
		ToolNamespaceStrategy:       payload.ToolNamespaceStrategy,
		ProxyMode:                   payload.ProxyMode,
		ProxyURL:                    payload.ProxyURL,
		ProxyNoProxy:                payload.ProxyNoProxy,
		ObservabilityListenAddress:  payload.ObservabilityListenAddress,
		ObservabilityMetricsEnabled: payload.ObservabilityMetricsEnabled,
		ObservabilityHealthzEnabled: payload.ObservabilityHealthzEnabled,
	}
}

func subAgentConfigToPayload(cfg domain.SubAgentConfig) SubAgentConfigPayload {
	return SubAgentConfigPayload{
		Enabled:            cfg.Enabled,
		EnabledTags:        append([]string(nil), cfg.EnabledTags...),
		Model:              cfg.Model,
		Provider:           cfg.Provider,
		APIKeyEnvVar:       cfg.APIKeyEnvVar,
		BaseURL:            cfg.BaseURL,
		MaxToolsPerRequest: cfg.MaxToolsPerRequest,
		FilterPrompt:       cfg.FilterPrompt,
	}
}

func subAgentConfigFromPayload(payload SubAgentConfigPayload) domain.SubAgentConfig {
	return domain.SubAgentConfig{
		Enabled:            payload.Enabled,
		EnabledTags:        append([]string(nil), payload.EnabledTags...),
		Model:              payload.Model,
		Provider:           payload.Provider,
		APIKeyEnvVar:       payload.APIKeyEnvVar,
		BaseURL:            payload.BaseURL,
		MaxToolsPerRequest: payload.MaxToolsPerRequest,
		FilterPrompt:       payload.FilterPrompt,
	}
}

func subAgentUpdateFromPayload(payload SubAgentUpdatePayload) catalogeditor.SubAgentConfigUpdate {
	enabled := payload.Enabled
	model := payload.Model
	provider := payload.Provider
	apiKeyEnvVar := payload.APIKeyEnvVar
	baseURL := payload.BaseURL
	maxTools := payload.MaxToolsPerRequest
	filterPrompt := payload.FilterPrompt

	update := catalogeditor.SubAgentConfigUpdate{
		Enabled:            &enabled,
		Model:              &model,
		Provider:           &provider,
		APIKeyEnvVar:       &apiKeyEnvVar,
		BaseURL:            &baseURL,
		MaxToolsPerRequest: &maxTools,
		FilterPrompt:       &filterPrompt,
	}
	if payload.EnabledTags != nil {
		tags := append([]string(nil), payload.EnabledTags...)
		update.EnabledTags = &tags
	}
	if payload.APIKey != nil {
		apiKey := *payload.APIKey
		update.APIKey = &apiKey
	}

	return update
}
