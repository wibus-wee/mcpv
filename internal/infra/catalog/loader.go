package catalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

type Loader struct {
	logger *zap.Logger
}

type rawCatalog struct {
	Servers               []domain.ServerSpec    `mapstructure:"servers"`
	RouteTimeoutSeconds   int                    `mapstructure:"routeTimeoutSeconds"`
	PingIntervalSeconds   int                    `mapstructure:"pingIntervalSeconds"`
	ToolRefreshSeconds    int                    `mapstructure:"toolRefreshSeconds"`
	CallerCheckSeconds    int                    `mapstructure:"callerCheckSeconds"`
	ExposeTools           bool                   `mapstructure:"exposeTools"`
	ToolNamespaceStrategy string                 `mapstructure:"toolNamespaceStrategy"`
	Observability         rawObservabilityConfig `mapstructure:"observability"`
	RPC                   rawRPCConfig           `mapstructure:"rpc"`
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

func (l *Loader) Load(ctx context.Context, path string) (domain.Catalog, error) {
	if path == "" {
		return domain.Catalog{}, errors.New("config path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Catalog{}, fmt.Errorf("read config: %w", err)
	}

	expanded, err := expandConfigEnv(data)
	if err != nil {
		return domain.Catalog{}, err
	}

	if err := validateCatalogSchema(expanded); err != nil {
		return domain.Catalog{}, err
	}

	v := viper.New()

	v.SetConfigType("yaml")
	v.SetDefault("routeTimeoutSeconds", domain.DefaultRouteTimeoutSeconds)
	v.SetDefault("pingIntervalSeconds", domain.DefaultPingIntervalSeconds)
	v.SetDefault("toolRefreshSeconds", domain.DefaultToolRefreshSeconds)
	v.SetDefault("callerCheckSeconds", domain.DefaultCallerCheckSeconds)
	v.SetDefault("exposeTools", domain.DefaultExposeTools)
	v.SetDefault("toolNamespaceStrategy", domain.DefaultToolNamespaceStrategy)
	v.SetDefault("observability.listenAddress", domain.DefaultObservabilityListenAddress)
	v.SetDefault("rpc.listenAddress", domain.DefaultRPCListenAddress)
	v.SetDefault("rpc.maxRecvMsgSize", domain.DefaultRPCMaxRecvMsgSize)
	v.SetDefault("rpc.maxSendMsgSize", domain.DefaultRPCMaxSendMsgSize)
	v.SetDefault("rpc.keepaliveTimeSeconds", domain.DefaultRPCKeepaliveTimeSeconds)
	v.SetDefault("rpc.keepaliveTimeoutSeconds", domain.DefaultRPCKeepaliveTimeoutSeconds)
	v.SetDefault("rpc.socketMode", domain.DefaultRPCSocketMode)

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

	if len(cfg.Servers) == 0 {
		return domain.Catalog{}, errors.New("no servers defined in catalog")
	}

	specs := make(map[string]domain.ServerSpec, len(cfg.Servers))
	var validationErrors []string
	nameSeen := make(map[string]struct{})
	runtime, runtimeErrs := normalizeRuntimeConfig(cfg)
	validationErrors = append(validationErrors, runtimeErrs...)

	for i, spec := range cfg.Servers {
		spec = normalizeServerSpec(spec)
		if _, exists := nameSeen[spec.Name]; exists {
			validationErrors = append(validationErrors, fmt.Sprintf("servers[%d]: duplicate name %q", i, spec.Name))
		} else if spec.Name != "" {
			nameSeen[spec.Name] = struct{}{}
		}

		if errs := validateServerSpec(spec, i); len(errs) > 0 {
			validationErrors = append(validationErrors, errs...)
			continue
		}

		specs[spec.Name] = spec
	}

	if len(validationErrors) > 0 {
		return domain.Catalog{}, errors.New(strings.Join(validationErrors, "; "))
	}

	return domain.Catalog{
		Specs:   specs,
		Runtime: runtime,
	}, nil
}

func normalizeServerSpec(spec domain.ServerSpec) domain.ServerSpec {
	if spec.ProtocolVersion == "" {
		spec.ProtocolVersion = domain.DefaultProtocolVersion
	}
	if spec.MaxConcurrent == 0 {
		spec.MaxConcurrent = domain.DefaultMaxConcurrent
	}
	return spec
}

func validateServerSpec(spec domain.ServerSpec, index int) []string {
	var errs []string

	if spec.Name == "" {
		errs = append(errs, fmt.Sprintf("servers[%d]: name is required", index))
	}
	if len(spec.Cmd) == 0 {
		errs = append(errs, fmt.Sprintf("servers[%d]: cmd is required", index))
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

	versionPattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if spec.ProtocolVersion == "" {
		errs = append(errs, fmt.Sprintf("servers[%d]: protocolVersion is required", index))
	} else {
		if !versionPattern.MatchString(spec.ProtocolVersion) {
			errs = append(errs, fmt.Sprintf("servers[%d]: protocolVersion must match YYYY-MM-DD", index))
		}
		if spec.ProtocolVersion != domain.DefaultProtocolVersion {
			errs = append(errs, fmt.Sprintf("servers[%d]: protocolVersion must be %s", index, domain.DefaultProtocolVersion))
		}
	}

	for i, tool := range spec.ExposeTools {
		if strings.TrimSpace(tool) == "" {
			errs = append(errs, fmt.Sprintf("servers[%d]: exposeTools[%d] must not be empty", index, i))
		}
	}

	return errs
}

func normalizeRuntimeConfig(cfg rawCatalog) (domain.RuntimeConfig, []string) {
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

	callerCheck := cfg.CallerCheckSeconds
	if callerCheck <= 0 {
		errs = append(errs, "callerCheckSeconds must be > 0")
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
		RouteTimeoutSeconds:   routeTimeout,
		PingIntervalSeconds:   pingInterval,
		ToolRefreshSeconds:    toolRefresh,
		CallerCheckSeconds:    callerCheck,
		ExposeTools:           cfg.ExposeTools,
		ToolNamespaceStrategy: strategy,
		Observability:         observabilityCfg,
		RPC:                   rpcCfg,
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
