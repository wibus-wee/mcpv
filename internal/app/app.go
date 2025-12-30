package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
	"mcpd/internal/infra/catalog"
	"mcpd/internal/infra/lifecycle"
	"mcpd/internal/infra/probe"
	"mcpd/internal/infra/router"
	"mcpd/internal/infra/rpc"
	"mcpd/internal/infra/scheduler"
	"mcpd/internal/infra/subagent"
	"mcpd/internal/infra/telemetry"
	"mcpd/internal/infra/transport"
)

type App struct {
	logger         *zap.Logger
	logBroadcaster *telemetry.LogBroadcaster
}

type ServeConfig struct {
	ConfigPath string
	OnReady    func(domain.ControlPlane) // Called when Core is ready (after RPC server starts)
}

type ValidateConfig struct {
	ConfigPath string
}

type profileConfig struct {
	profile  domain.Profile
	specKeys map[string]string
}

type profileSummary struct {
	configs         map[string]profileConfig
	specRegistry    map[string]domain.ServerSpec
	totalServers    int
	minPingInterval int
	defaultRuntime  domain.RuntimeConfig
}

func New(logger *zap.Logger) *App {
	if logger == nil {
		logger = zap.NewNop()
	}
	logger = logger.With(zap.String(telemetry.FieldLogSource, telemetry.LogSourceCore)).Named("app")
	return &App{
		logger: logger,
	}
}

func NewWithBroadcaster(logger *zap.Logger, broadcaster *telemetry.LogBroadcaster) *App {
	if logger == nil {
		logger = zap.NewNop()
	}
	logger = logger.With(zap.String(telemetry.FieldLogSource, telemetry.LogSourceCore)).Named("app")
	return &App{
		logger:         logger,
		logBroadcaster: broadcaster,
	}
}

func (a *App) Serve(ctx context.Context, cfg ServeConfig) error {
	var logs *telemetry.LogBroadcaster
	var logger *zap.Logger

	// Use existing broadcaster if available, otherwise create a new one
	if a.logBroadcaster != nil {
		logs = a.logBroadcaster
		logger = a.logger
	} else {
		logs = telemetry.NewLogBroadcaster(zapcore.DebugLevel)
		logger = a.logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewTee(core, logs.Core())
		}))
	}

	storeLoader := catalog.NewProfileStoreLoader(logger)
	store, err := storeLoader.Load(ctx, cfg.ConfigPath, catalog.ProfileStoreOptions{
		AllowCreate: true,
	})
	if err != nil {
		return err
	}

	summary, err := buildProfileSummary(store)
	if err != nil {
		return err
	}

	logger.Info("configuration loaded",
		zap.String("config", cfg.ConfigPath),
		zap.Int("profiles", len(store.Profiles)),
		zap.Int("servers", summary.totalServers),
	)

	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	registry.MustRegister(prometheus.NewGoCollector())

	stdioTransport := transport.NewStdioTransport(transport.StdioTransportOptions{Logger: logger})
	lc := lifecycle.NewManager(ctx, stdioTransport, logger)
	pingProbe := &probe.PingProbe{Timeout: defaultPingProbeTimeout}
	metrics := telemetry.NewPrometheusMetrics(registry)
	health := telemetry.NewHealthTracker()
	sched, err := scheduler.NewBasicScheduler(lc, summary.specRegistry, scheduler.SchedulerOptions{
		Probe:   pingProbe,
		Logger:  logger,
		Metrics: metrics,
		Health:  health,
	})
	if err != nil {
		return err
	}
	initManager := NewServerInitializationManager(sched, summary.specRegistry, logger)
	initManager.Start(ctx)

	profiles := make(map[string]*profileRuntime, len(summary.configs))
	for name, cfg := range summary.configs {
		profileLogger := logger.With(zap.String("profile", name))
		refreshGate := aggregator.NewRefreshGate()
		rt := router.NewBasicRouter(sched, router.RouterOptions{
			Timeout: time.Duration(cfg.profile.Catalog.Runtime.RouteTimeoutSeconds) * time.Second,
			Logger:  profileLogger,
			Metrics: metrics,
		})
		toolIndex := aggregator.NewToolIndex(rt, cfg.profile.Catalog.Specs, cfg.specKeys, cfg.profile.Catalog.Runtime, profileLogger, health, refreshGate)
		resourceIndex := aggregator.NewResourceIndex(rt, cfg.profile.Catalog.Specs, cfg.specKeys, cfg.profile.Catalog.Runtime, profileLogger, health, refreshGate)
		promptIndex := aggregator.NewPromptIndex(rt, cfg.profile.Catalog.Specs, cfg.specKeys, cfg.profile.Catalog.Runtime, profileLogger, health, refreshGate)
		profiles[name] = &profileRuntime{
			name:      name,
			specKeys:  collectSpecKeys(cfg.specKeys),
			tools:     toolIndex,
			resources: resourceIndex,
			prompts:   promptIndex,
		}
	}

	control := NewControlPlane(ctx, profiles, store.Callers, summary.specRegistry, sched, initManager, summary.defaultRuntime, store, logs, logger)

	// Initialize SubAgent if configured in runtime
	if summary.defaultRuntime.SubAgent.Model != "" && summary.defaultRuntime.SubAgent.Provider != "" {
		subAgent, err := initializeSubAgent(ctx, summary.defaultRuntime.SubAgent, control, metrics, logger)
		if err != nil {
			logger.Warn("failed to initialize SubAgent", zap.Error(err))
		} else {
			control.SetSubAgent(subAgent)
			logger.Info("SubAgent initialized",
				zap.String("provider", summary.defaultRuntime.SubAgent.Provider),
				zap.String("model", summary.defaultRuntime.SubAgent.Model),
			)
		}
	}

	if cfg.OnReady != nil {
		cfg.OnReady(control)
	}
	control.StartCallerMonitor(ctx)
	rpcServer := rpc.NewServer(control, summary.defaultRuntime.RPC, logger)

	metricsEnabled := envBool("MCPD_METRICS_ENABLED")
	healthzEnabled := envBool("MCPD_HEALTHZ_ENABLED")
	if metricsEnabled || healthzEnabled {
		go func() {
			addr := summary.defaultRuntime.Observability.ListenAddress
			logger.Info("starting observability server", zap.String("addr", addr))
			if err := telemetry.StartHTTPServer(ctx, telemetry.HTTPServerOptions{
				Addr:          addr,
				EnableMetrics: metricsEnabled,
				EnableHealthz: healthzEnabled,
				Health:        health,
				Registry:      registry,
			}, logger); err != nil {
				logger.Error("observability server failed", zap.Error(err))
			}
		}()
	}

	sched.StartIdleManager(defaultIdleManagerInterval)
	if summary.minPingInterval > 0 {
		sched.StartPingManager(time.Duration(summary.minPingInterval) * time.Second)
	}
	defer func() {
		for _, runtime := range profiles {
			runtime.Deactivate()
		}
		initManager.Stop()
		sched.StopPingManager()
		sched.StopIdleManager()
		sched.StopAll(context.Background())
	}()

	return rpcServer.Run(ctx)
}

func (a *App) ValidateConfig(ctx context.Context, cfg ValidateConfig) error {
	storeLoader := catalog.NewProfileStoreLoader(a.logger)
	store, err := storeLoader.Load(ctx, cfg.ConfigPath, catalog.ProfileStoreOptions{
		AllowCreate: false,
	})
	if err != nil {
		return err
	}

	if _, err := buildProfileSummary(store); err != nil {
		return err
	}

	a.logger.Info("configuration validated",
		zap.String("config", cfg.ConfigPath),
		zap.Int("profiles", len(store.Profiles)),
	)
	return nil
}

func envBool(key string) bool {
	val := strings.TrimSpace(os.Getenv(key))
	return val == "1" || strings.EqualFold(val, "true")
}

func buildProfileSummary(store domain.ProfileStore) (profileSummary, error) {
	if len(store.Profiles) == 0 {
		return profileSummary{}, errors.New("no profiles loaded")
	}

	defaultProfile, ok := store.Profiles[domain.DefaultProfileName]
	if !ok {
		return profileSummary{}, fmt.Errorf("default profile %q not found", domain.DefaultProfileName)
	}

	summary := profileSummary{
		configs:         make(map[string]profileConfig, len(store.Profiles)),
		specRegistry:    make(map[string]domain.ServerSpec),
		totalServers:    0,
		minPingInterval: 0,
		defaultRuntime:  defaultProfile.Catalog.Runtime,
	}

	for name, profile := range store.Profiles {
		if err := validateSharedRuntime(summary.defaultRuntime, profile.Catalog.Runtime); err != nil {
			return profileSummary{}, fmt.Errorf("profile %q: %w", name, err)
		}

		enabledSpecs, enabledCount := filterEnabledSpecs(profile.Catalog.Specs)
		runtimeProfile := profile
		runtimeProfile.Catalog.Specs = enabledSpecs

		specKeys, err := buildSpecKeys(runtimeProfile.Catalog.Specs)
		if err != nil {
			return profileSummary{}, fmt.Errorf("profile %q: %w", name, err)
		}
		summary.configs[name] = profileConfig{
			profile:  runtimeProfile,
			specKeys: specKeys,
		}
		summary.totalServers += enabledCount
		if profile.Catalog.Runtime.PingIntervalSeconds > 0 {
			if summary.minPingInterval == 0 || profile.Catalog.Runtime.PingIntervalSeconds < summary.minPingInterval {
				summary.minPingInterval = profile.Catalog.Runtime.PingIntervalSeconds
			}
		}

		for serverType, spec := range runtimeProfile.Catalog.Specs {
			specKey := specKeys[serverType]
			if specKey == "" {
				return profileSummary{}, fmt.Errorf("profile %q: missing spec key for %q", name, serverType)
			}
			if _, ok := summary.specRegistry[specKey]; ok {
				continue
			}
			// Keep the original spec.Name as-is for display purposes
			summary.specRegistry[specKey] = spec
		}
	}

	return summary, nil
}

func filterEnabledSpecs(specs map[string]domain.ServerSpec) (map[string]domain.ServerSpec, int) {
	if len(specs) == 0 {
		return map[string]domain.ServerSpec{}, 0
	}

	enabled := make(map[string]domain.ServerSpec, len(specs))
	count := 0
	for name, spec := range specs {
		if spec.Disabled {
			continue
		}
		enabled[name] = spec
		count++
	}
	return enabled, count
}

func buildSpecKeys(specs map[string]domain.ServerSpec) (map[string]string, error) {
	keys := make(map[string]string, len(specs))
	for serverType, spec := range specs {
		specKey, err := domain.SpecFingerprint(spec)
		if err != nil {
			return nil, fmt.Errorf("spec fingerprint for %q: %w", serverType, err)
		}
		keys[serverType] = specKey
	}
	return keys, nil
}

func collectSpecKeys(specKeys map[string]string) []string {
	if len(specKeys) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(specKeys))
	for _, key := range specKeys {
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func validateSharedRuntime(base domain.RuntimeConfig, current domain.RuntimeConfig) error {
	if base.RPC != current.RPC {
		return errors.New("rpc config must match across profiles")
	}
	if base.Observability != current.Observability {
		return errors.New("observability config must match across profiles")
	}
	return nil
}

func initializeSubAgent(ctx context.Context, config domain.SubAgentConfig, controlPlane *ControlPlane, metrics domain.Metrics, logger *zap.Logger) (domain.SubAgent, error) {
	return subagent.NewEinoSubAgent(ctx, config, controlPlane, metrics, logger)
}
