package catalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

type Loader struct {
	logger *zap.Logger
}

func newRuntimeViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType("yaml")
	setRuntimeDefaults(v)
	return v
}

func setRuntimeDefaults(v *viper.Viper) {
	v.SetDefault("routeTimeoutSeconds", domain.DefaultRouteTimeoutSeconds)
	v.SetDefault("pingIntervalSeconds", domain.DefaultPingIntervalSeconds)
	v.SetDefault("toolRefreshSeconds", domain.DefaultToolRefreshSeconds)
	v.SetDefault("toolRefreshConcurrency", domain.DefaultToolRefreshConcurrency)
	v.SetDefault("callerCheckSeconds", domain.DefaultCallerCheckSeconds)
	v.SetDefault("callerInactiveSeconds", domain.DefaultCallerInactiveSeconds)
	v.SetDefault("serverInitRetryBaseSeconds", domain.DefaultServerInitRetryBaseSeconds)
	v.SetDefault("serverInitRetryMaxSeconds", domain.DefaultServerInitRetryMaxSeconds)
	v.SetDefault("serverInitMaxRetries", domain.DefaultServerInitMaxRetries)
	v.SetDefault("bootstrapMode", domain.DefaultBootstrapMode)
	v.SetDefault("bootstrapConcurrency", domain.DefaultBootstrapConcurrency)
	v.SetDefault("bootstrapTimeoutSeconds", domain.DefaultBootstrapTimeoutSeconds)
	v.SetDefault("defaultActivationMode", domain.DefaultActivationMode)
	v.SetDefault("exposeTools", domain.DefaultExposeTools)
	v.SetDefault("toolNamespaceStrategy", domain.DefaultToolNamespaceStrategy)
	v.SetDefault("observability.listenAddress", domain.DefaultObservabilityListenAddress)
	v.SetDefault("rpc.listenAddress", domain.DefaultRPCListenAddress)
	v.SetDefault("rpc.maxRecvMsgSize", domain.DefaultRPCMaxRecvMsgSize)
	v.SetDefault("rpc.maxSendMsgSize", domain.DefaultRPCMaxSendMsgSize)
	v.SetDefault("rpc.keepaliveTimeSeconds", domain.DefaultRPCKeepaliveTimeSeconds)
	v.SetDefault("rpc.keepaliveTimeoutSeconds", domain.DefaultRPCKeepaliveTimeoutSeconds)
	v.SetDefault("rpc.socketMode", domain.DefaultRPCSocketMode)
}

type rawCatalog struct {
	Servers          []rawServerSpec          `mapstructure:"servers"`
	SubAgent         rawProfileSubAgentConfig `mapstructure:"subAgent"`
	rawRuntimeConfig `mapstructure:",squash"`
}

type rawServerSpec struct {
	Name                string                  `mapstructure:"name"`
	Transport           string                  `mapstructure:"transport"`
	Cmd                 []string                `mapstructure:"cmd"`
	Env                 map[string]string       `mapstructure:"env"`
	Cwd                 string                  `mapstructure:"cwd"`
	IdleSeconds         int                     `mapstructure:"idleSeconds"`
	MaxConcurrent       int                     `mapstructure:"maxConcurrent"`
	Strategy            string                  `mapstructure:"strategy"`
	SessionTTLSeconds   *int                    `mapstructure:"sessionTTLSeconds"`
	Disabled            bool                    `mapstructure:"disabled"`
	MinReady            int                     `mapstructure:"minReady"`
	ActivationMode      string                  `mapstructure:"activationMode"`
	DrainTimeoutSeconds int                     `mapstructure:"drainTimeoutSeconds"`
	ProtocolVersion     string                  `mapstructure:"protocolVersion"`
	ExposeTools         []string                `mapstructure:"exposeTools"`
	HTTP                rawStreamableHTTPConfig `mapstructure:"http"`
}

type rawStreamableHTTPConfig struct {
	Endpoint   string            `mapstructure:"endpoint"`
	Headers    map[string]string `mapstructure:"headers"`
	MaxRetries *int              `mapstructure:"maxRetries"`
}

type rawProfileSubAgentConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type rawRuntimeConfig struct {
	RouteTimeoutSeconds        int                    `mapstructure:"routeTimeoutSeconds"`
	PingIntervalSeconds        int                    `mapstructure:"pingIntervalSeconds"`
	ToolRefreshSeconds         int                    `mapstructure:"toolRefreshSeconds"`
	ToolRefreshConcurrency     int                    `mapstructure:"toolRefreshConcurrency"`
	CallerCheckSeconds         int                    `mapstructure:"callerCheckSeconds"`
	CallerInactiveSeconds      int                    `mapstructure:"callerInactiveSeconds"`
	ServerInitRetryBaseSeconds int                    `mapstructure:"serverInitRetryBaseSeconds"`
	ServerInitRetryMaxSeconds  int                    `mapstructure:"serverInitRetryMaxSeconds"`
	ServerInitMaxRetries       int                    `mapstructure:"serverInitMaxRetries"`
	BootstrapMode              string                 `mapstructure:"bootstrapMode"`
	BootstrapConcurrency       int                    `mapstructure:"bootstrapConcurrency"`
	BootstrapTimeoutSeconds    int                    `mapstructure:"bootstrapTimeoutSeconds"`
	DefaultActivationMode      string                 `mapstructure:"defaultActivationMode"`
	ExposeTools                bool                   `mapstructure:"exposeTools"`
	ToolNamespaceStrategy      string                 `mapstructure:"toolNamespaceStrategy"`
	Observability              rawObservabilityConfig `mapstructure:"observability"`
	RPC                        rawRPCConfig           `mapstructure:"rpc"`
	SubAgent                   rawSubAgentConfig      `mapstructure:"subAgent"`
}

type rawSubAgentConfig struct {
	Model              string `mapstructure:"model"`
	Provider           string `mapstructure:"provider"`
	APIKey             string `mapstructure:"apiKey"`
	APIKeyEnvVar       string `mapstructure:"apiKeyEnvVar"`
	BaseURL            string `mapstructure:"baseURL"`
	MaxToolsPerRequest int    `mapstructure:"maxToolsPerRequest"`
	FilterPrompt       string `mapstructure:"filterPrompt"`
}

type rawObservabilityConfig struct {
	ListenAddress string `mapstructure:"listenAddress"`
}

type rawRPCConfig struct {
	ListenAddress           string          `mapstructure:"listenAddress"`
	MaxRecvMsgSize          int             `mapstructure:"maxRecvMsgSize"`
	MaxSendMsgSize          int             `mapstructure:"maxSendMsgSize"`
	KeepaliveTimeSeconds    int             `mapstructure:"keepaliveTimeSeconds"`
	KeepaliveTimeoutSeconds int             `mapstructure:"keepaliveTimeoutSeconds"`
	SocketMode              string          `mapstructure:"socketMode"`
	TLS                     rawRPCTLSConfig `mapstructure:"tls"`
}

type rawRPCTLSConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	CertFile   string `mapstructure:"certFile"`
	KeyFile    string `mapstructure:"keyFile"`
	CAFile     string `mapstructure:"caFile"`
	ClientAuth bool   `mapstructure:"clientAuth"`
}

func NewLoader(logger *zap.Logger) *Loader {
	if logger == nil {
		return &Loader{logger: zap.NewNop()}
	}
	return &Loader{logger: logger.Named("catalog")}
}

// LoadRuntimeConfig loads only the runtime section from a config file.
// This is intended for profile-store level runtime defaults.
func (l *Loader) LoadRuntimeConfig(ctx context.Context, path string) (domain.RuntimeConfig, error) {
	if path == "" {
		return domain.RuntimeConfig{}, errors.New("config path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return domain.RuntimeConfig{}, fmt.Errorf("read config: %w", err)
	}

	expanded, missing, err := expandConfigEnv(data)
	if err != nil {
		return domain.RuntimeConfig{}, err
	}
	if len(missing) > 0 {
		l.logger.Warn("missing environment variables in runtime config", zap.String("path", path), zap.Strings("missing", missing))
	}

	rawCfg, err := decodeRuntimeConfig(expanded)
	if err != nil {
		return domain.RuntimeConfig{}, err
	}

	runtime, errs := normalizeRuntimeConfig(rawCfg)
	if len(errs) > 0 {
		return domain.RuntimeConfig{}, errors.New(strings.Join(errs, "; "))
	}
	return runtime, ctx.Err()
}

func (l *Loader) Load(ctx context.Context, path string) (domain.Catalog, error) {
	if path == "" {
		return domain.Catalog{}, errors.New("config path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Catalog{}, fmt.Errorf("read config: %w", err)
	}

	expanded, missing, err := expandConfigEnv(data)
	if err != nil {
		return domain.Catalog{}, err
	}
	if len(missing) > 0 {
		l.logger.Warn("missing environment variables in config", zap.String("path", path), zap.Strings("missing", missing))
	}

	if err := validateCatalogSchema(expanded); err != nil {
		return domain.Catalog{}, err
	}

	v := newRuntimeViper()

	if err := v.ReadConfig(bytes.NewBufferString(expanded)); err != nil {
		return domain.Catalog{}, fmt.Errorf("parse config: %w", err)
	}

	var cfg rawCatalog
	if err := v.Unmarshal(&cfg); err != nil {
		return domain.Catalog{}, fmt.Errorf("decode config: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return domain.Catalog{}, err
	}

	specs := make(map[string]domain.ServerSpec, len(cfg.Servers))
	var validationErrors []string
	nameSeen := make(map[string]struct{})
	runtime, runtimeErrs := normalizeRuntimeConfig(cfg.rawRuntimeConfig)
	validationErrors = append(validationErrors, runtimeErrs...)

	for i, spec := range cfg.Servers {
		normalized, implicitHTTP := normalizeServerSpec(spec)
		if implicitHTTP {
			l.logger.Warn("server transport inferred from http config; consider setting transport explicitly",
				zap.String("server", normalized.Name),
				zap.Int("index", i),
			)
		}
		if _, exists := nameSeen[normalized.Name]; exists {
			validationErrors = append(validationErrors, fmt.Sprintf("servers[%d]: duplicate name %q", i, normalized.Name))
		} else if normalized.Name != "" {
			nameSeen[normalized.Name] = struct{}{}
		}

		if errs := validateServerSpec(normalized, i); len(errs) > 0 {
			validationErrors = append(validationErrors, errs...)
			continue
		}

		specs[normalized.Name] = normalized
	}

	if len(validationErrors) > 0 {
		return domain.Catalog{}, errors.New(strings.Join(validationErrors, "; "))
	}

	return domain.Catalog{
		Specs:    specs,
		Runtime:  runtime,
		SubAgent: domain.ProfileSubAgentConfig{Enabled: cfg.SubAgent.Enabled},
	}, nil
}

func decodeRuntimeConfig(expanded string) (rawRuntimeConfig, error) {
	v := newRuntimeViper()
	if err := v.ReadConfig(bytes.NewBufferString(expanded)); err != nil {
		return rawRuntimeConfig{}, fmt.Errorf("parse config: %w", err)
	}
	var cfg rawRuntimeConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return rawRuntimeConfig{}, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func normalizeServerSpec(raw rawServerSpec) (domain.ServerSpec, bool) {
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

func normalizeStreamableHTTPConfig(raw rawStreamableHTTPConfig, transport domain.TransportKind) *domain.StreamableHTTPConfig {
	if domain.NormalizeTransport(transport) != domain.TransportStreamableHTTP {
		return nil
	}

	maxRetries := domain.DefaultStreamableHTTPMaxRetries
	if raw.MaxRetries != nil {
		maxRetries = *raw.MaxRetries
	}

	headers := normalizeHTTPHeaders(raw.Headers)

	return &domain.StreamableHTTPConfig{
		Endpoint:   strings.TrimSpace(raw.Endpoint),
		Headers:    headers,
		MaxRetries: maxRetries,
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

func validateServerSpec(spec domain.ServerSpec, index int) []string {
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

	// Validate strategy
	switch spec.Strategy {
	case domain.StrategyStateless, domain.StrategyStateful, domain.StrategyPersistent, domain.StrategySingleton:
		// valid
	default:
		errs = append(errs, fmt.Sprintf("servers[%d]: strategy must be one of: stateless, stateful, persistent, singleton", index))
	}

	// Validate sessionTTLSeconds for stateful strategy
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

func normalizeRuntimeConfig(cfg rawRuntimeConfig) (domain.RuntimeConfig, []string) {
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

	callerCheck := cfg.CallerCheckSeconds
	if callerCheck <= 0 {
		errs = append(errs, "callerCheckSeconds must be > 0")
	}

	callerInactive := cfg.CallerInactiveSeconds
	if callerInactive <= 0 {
		errs = append(errs, "callerInactiveSeconds must be > 0")
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
		strategy = domain.DefaultToolNamespaceStrategy
	}
	if strategy != "prefix" && strategy != "flat" {
		errs = append(errs, "toolNamespaceStrategy must be prefix or flat")
	}

	observabilityCfg, observabilityErrs := normalizeObservabilityConfig(cfg.Observability)
	errs = append(errs, observabilityErrs...)

	rpcCfg, rpcErrs := normalizeRPCConfig(cfg.RPC)
	errs = append(errs, rpcErrs...)

	return domain.RuntimeConfig{
		RouteTimeoutSeconds:        routeTimeout,
		PingIntervalSeconds:        pingInterval,
		ToolRefreshSeconds:         toolRefresh,
		ToolRefreshConcurrency:     refreshConcurrency,
		CallerCheckSeconds:         callerCheck,
		CallerInactiveSeconds:      callerInactive,
		ServerInitRetryBaseSeconds: serverInitRetryBase,
		ServerInitRetryMaxSeconds:  serverInitRetryMax,
		ServerInitMaxRetries:       serverInitMaxRetries,
		BootstrapMode:              domain.BootstrapMode(bootstrapMode),
		BootstrapConcurrency:       bootstrapConcurrency,
		BootstrapTimeoutSeconds:    bootstrapTimeoutSeconds,
		DefaultActivationMode:      domain.ActivationMode(defaultActivationMode),
		ExposeTools:                cfg.ExposeTools,
		ToolNamespaceStrategy:      strategy,
		Observability:              observabilityCfg,
		RPC:                        rpcCfg,
		SubAgent: domain.SubAgentConfig{
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

func normalizeObservabilityConfig(cfg rawObservabilityConfig) (domain.ObservabilityConfig, []string) {
	addr := strings.TrimSpace(cfg.ListenAddress)
	if addr == "" {
		addr = domain.DefaultObservabilityListenAddress
	}
	return domain.ObservabilityConfig{
		ListenAddress: addr,
	}, nil
}

func normalizeRPCConfig(cfg rawRPCConfig) (domain.RPCConfig, []string) {
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
	if _, err := parseSocketMode(socketMode); err != nil {
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
