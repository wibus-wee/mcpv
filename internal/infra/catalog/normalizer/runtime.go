package normalizer

import (
	"strings"

	"mcpv/internal/domain"
)

func NormalizeRuntimeConfig(cfg RawRuntimeConfig) (domain.RuntimeConfig, []string) {
	var errs []string

	routeTimeout := cfg.RouteTimeoutSeconds
	if routeTimeout <= 0 {
		errs = append(errs, "routeTimeoutSeconds must be > 0")
	}

	pingInterval := cfg.PingIntervalSeconds
	if pingInterval < 0 {
		errs = append(errs, "pingIntervalSeconds must be >= 0")
	}

	toolRefresh := cfg.ToolRefreshSeconds
	if toolRefresh < 0 {
		errs = append(errs, "toolRefreshSeconds must be >= 0")
	}

	refreshConcurrency := cfg.ToolRefreshConcurrency
	if refreshConcurrency < 0 {
		errs = append(errs, "toolRefreshConcurrency must be >= 0")
	}
	if refreshConcurrency <= 0 {
		refreshConcurrency = domain.DefaultToolRefreshConcurrency
	}

	clientCheck := cfg.ClientCheckSeconds
	if clientCheck <= 0 {
		errs = append(errs, "clientCheckSeconds must be > 0")
	}

	clientInactive := cfg.ClientInactiveSeconds
	if clientInactive <= 0 {
		errs = append(errs, "clientInactiveSeconds must be > 0")
	}

	serverInitRetryBase := cfg.ServerInitRetryBaseSeconds
	if serverInitRetryBase <= 0 {
		errs = append(errs, "serverInitRetryBaseSeconds must be > 0")
	}
	serverInitRetryMax := cfg.ServerInitRetryMaxSeconds
	if serverInitRetryMax <= 0 {
		errs = append(errs, "serverInitRetryMaxSeconds must be > 0")
	}
	if serverInitRetryBase > 0 && serverInitRetryMax > 0 && serverInitRetryMax < serverInitRetryBase {
		errs = append(errs, "serverInitRetryMaxSeconds must be >= serverInitRetryBaseSeconds")
	}
	serverInitMaxRetries := cfg.ServerInitMaxRetries
	if serverInitMaxRetries < 0 {
		errs = append(errs, "serverInitMaxRetries must be >= 0")
	}

	reloadMode := strings.ToLower(strings.TrimSpace(cfg.ReloadMode))
	if reloadMode == "" {
		reloadMode = string(domain.DefaultReloadMode)
	}
	if reloadMode != string(domain.ReloadModeStrict) && reloadMode != string(domain.ReloadModeLenient) {
		errs = append(errs, "reloadMode must be strict or lenient")
	}

	bootstrapMode := strings.ToLower(strings.TrimSpace(cfg.BootstrapMode))
	if bootstrapMode == "" {
		bootstrapMode = string(domain.DefaultBootstrapMode)
	}
	if bootstrapMode != string(domain.BootstrapModeMetadata) && bootstrapMode != string(domain.BootstrapModeDisabled) {
		errs = append(errs, "bootstrapMode must be metadata or disabled")
	}

	bootstrapConcurrency := cfg.BootstrapConcurrency
	if bootstrapConcurrency <= 0 {
		bootstrapConcurrency = domain.DefaultBootstrapConcurrency
	}
	bootstrapTimeoutSeconds := cfg.BootstrapTimeoutSeconds
	if bootstrapTimeoutSeconds <= 0 {
		bootstrapTimeoutSeconds = domain.DefaultBootstrapTimeoutSeconds
	}

	defaultActivationMode := strings.ToLower(strings.TrimSpace(cfg.DefaultActivationMode))
	if defaultActivationMode == "" {
		defaultActivationMode = string(domain.DefaultActivationMode)
	}
	if defaultActivationMode != string(domain.ActivationOnDemand) && defaultActivationMode != string(domain.ActivationAlwaysOn) {
		errs = append(errs, "defaultActivationMode must be on-demand or always-on")
	}

	strategy := strings.ToLower(strings.TrimSpace(cfg.ToolNamespaceStrategy))
	if strategy == "" {
		strategy = string(domain.DefaultToolNamespaceStrategy)
	}
	if strategy != string(domain.ToolNamespaceStrategyPrefix) && strategy != string(domain.ToolNamespaceStrategyFlat) {
		errs = append(errs, "toolNamespaceStrategy must be prefix or flat")
	}

	observabilityCfg, observabilityErrs := normalizeObservabilityConfig(cfg.Observability)
	errs = append(errs, observabilityErrs...)

	rpcCfg, rpcErrs := normalizeRPCConfig(cfg.RPC)
	errs = append(errs, rpcErrs...)

	proxyCfg, proxyErrs := normalizeRuntimeProxyConfig(cfg.Proxy)
	errs = append(errs, proxyErrs...)

	enabledTags := NormalizeTags(cfg.SubAgent.EnabledTags)
	enabled := false
	if cfg.SubAgent.Enabled != nil {
		enabled = *cfg.SubAgent.Enabled
	} else if cfg.SubAgent.Model != "" && cfg.SubAgent.Provider != "" && len(enabledTags) > 0 {
		enabled = true
	}
	return domain.RuntimeConfig{
		RouteTimeoutSeconds:        routeTimeout,
		PingIntervalSeconds:        pingInterval,
		ToolRefreshSeconds:         toolRefresh,
		ToolRefreshConcurrency:     refreshConcurrency,
		ClientCheckSeconds:         clientCheck,
		ClientInactiveSeconds:      clientInactive,
		ServerInitRetryBaseSeconds: serverInitRetryBase,
		ServerInitRetryMaxSeconds:  serverInitRetryMax,
		ServerInitMaxRetries:       serverInitMaxRetries,
		ReloadMode:                 domain.ReloadMode(reloadMode),
		BootstrapMode:              domain.BootstrapMode(bootstrapMode),
		BootstrapConcurrency:       bootstrapConcurrency,
		BootstrapTimeoutSeconds:    bootstrapTimeoutSeconds,
		DefaultActivationMode:      domain.ActivationMode(defaultActivationMode),
		ExposeTools:                cfg.ExposeTools,
		ToolNamespaceStrategy:      domain.ToolNamespaceStrategy(strategy),
		Proxy:                      proxyCfg,
		Observability:              observabilityCfg,
		RPC:                        rpcCfg,
		SubAgent: domain.SubAgentConfig{
			Enabled:            enabled,
			EnabledTags:        enabledTags,
			Model:              cfg.SubAgent.Model,
			Provider:           cfg.SubAgent.Provider,
			APIKey:             cfg.SubAgent.APIKey,
			APIKeyEnvVar:       cfg.SubAgent.APIKeyEnvVar,
			BaseURL:            cfg.SubAgent.BaseURL,
			MaxToolsPerRequest: cfg.SubAgent.MaxToolsPerRequest,
			FilterPrompt:       cfg.SubAgent.FilterPrompt,
		},
	}, errs
}

func normalizeObservabilityConfig(cfg RawObservabilityConfig) (domain.ObservabilityConfig, []string) {
	addr := strings.TrimSpace(cfg.ListenAddress)
	if addr == "" {
		addr = domain.DefaultObservabilityListenAddress
	}
	return domain.ObservabilityConfig{
		ListenAddress:  addr,
		MetricsEnabled: cfg.MetricsEnabled,
		HealthzEnabled: cfg.HealthzEnabled,
	}, nil
}

func normalizeRPCConfig(cfg RawRPCConfig) (domain.RPCConfig, []string) {
	var errs []string

	addr := strings.TrimSpace(cfg.ListenAddress)
	if addr == "" {
		errs = append(errs, "rpc.listenAddress is required")
	}

	if cfg.MaxRecvMsgSize <= 0 {
		errs = append(errs, "rpc.maxRecvMsgSize must be > 0")
	}
	if cfg.MaxSendMsgSize <= 0 {
		errs = append(errs, "rpc.maxSendMsgSize must be > 0")
	}
	if cfg.KeepaliveTimeSeconds < 0 {
		errs = append(errs, "rpc.keepaliveTimeSeconds must be >= 0")
	}
	if cfg.KeepaliveTimeoutSeconds < 0 {
		errs = append(errs, "rpc.keepaliveTimeoutSeconds must be >= 0")
	}

	socketMode := strings.TrimSpace(cfg.SocketMode)
	if socketMode == "" {
		socketMode = domain.DefaultRPCSocketMode
	}
	if _, err := domain.ParseSocketMode(socketMode); err != nil {
		errs = append(errs, err.Error())
	}

	tlsCfg := domain.RPCTLSConfig{
		Enabled:    cfg.TLS.Enabled,
		CertFile:   strings.TrimSpace(cfg.TLS.CertFile),
		KeyFile:    strings.TrimSpace(cfg.TLS.KeyFile),
		CAFile:     strings.TrimSpace(cfg.TLS.CAFile),
		ClientAuth: cfg.TLS.ClientAuth,
	}
	if tlsCfg.Enabled {
		if tlsCfg.CertFile == "" || tlsCfg.KeyFile == "" {
			errs = append(errs, "rpc.tls.certFile and rpc.tls.keyFile are required when rpc.tls.enabled is true")
		}
		if tlsCfg.ClientAuth && tlsCfg.CAFile == "" {
			errs = append(errs, "rpc.tls.caFile is required when rpc.tls.clientAuth is true")
		}
	}

	return domain.RPCConfig{
		ListenAddress:           addr,
		MaxRecvMsgSize:          cfg.MaxRecvMsgSize,
		MaxSendMsgSize:          cfg.MaxSendMsgSize,
		KeepaliveTimeSeconds:    cfg.KeepaliveTimeSeconds,
		KeepaliveTimeoutSeconds: cfg.KeepaliveTimeoutSeconds,
		SocketMode:              socketMode,
		TLS:                     tlsCfg,
	}, errs
}
