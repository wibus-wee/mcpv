package app

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/app/bootstrap/metadata"
	"mcpv/internal/app/controlplane"
	"mcpv/internal/app/runtime"
	"mcpv/internal/domain"
	"mcpv/internal/infra/elicitation"
	"mcpv/internal/infra/governance"
	"mcpv/internal/infra/lifecycle"
	"mcpv/internal/infra/notifications"
	"mcpv/internal/infra/pipeline"
	pluginmanager "mcpv/internal/infra/plugin/manager"
	"mcpv/internal/infra/probe"
	"mcpv/internal/infra/rpc"
	"mcpv/internal/infra/sampling"
	"mcpv/internal/infra/scheduler"
	"mcpv/internal/infra/telemetry"
	"mcpv/internal/infra/transport"
)

// NewMetricsRegistry creates a Prometheus registry.
func NewMetricsRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())
	return registry
}

// ConfigPath extracts the config path from the serve config.
func ConfigPath(cfg ServeConfig) string {
	return cfg.ConfigPath
}

// NewMetrics constructs metrics backed by Prometheus.
func NewMetrics(registry *prometheus.Registry) domain.Metrics {
	return telemetry.NewPrometheusMetrics(registry)
}

// NewHealthTracker constructs a health tracker.
func NewHealthTracker() *telemetry.HealthTracker {
	return telemetry.NewHealthTracker()
}

// NewListChangeHub constructs a list change hub.
func NewListChangeHub() *notifications.ListChangeHub {
	return notifications.NewListChangeHub()
}

// NewCommandLauncher constructs a launcher for stdio servers.
func NewCommandLauncher(logger *zap.Logger) domain.Launcher {
	return transport.NewCommandLauncher(transport.CommandLauncherOptions{Logger: logger})
}

// NewMCPTransport constructs an MCP transport for stdio servers.
func NewMCPTransport(logger *zap.Logger, listChanges *notifications.ListChangeHub, samplingHandler domain.SamplingHandler, elicitationHandler domain.ElicitationHandler) domain.Transport {
	stdioTransport := transport.NewMCPTransport(transport.MCPTransportOptions{
		Logger:             logger,
		ListChangeEmitter:  listChanges,
		SamplingHandler:    samplingHandler,
		ElicitationHandler: elicitationHandler,
	})
	httpTransport := transport.NewStreamableHTTPTransport(transport.StreamableHTTPTransportOptions{
		Logger:             logger,
		ListChangeEmitter:  listChanges,
		SamplingHandler:    samplingHandler,
		ElicitationHandler: elicitationHandler,
	})
	return transport.NewCompositeTransport(transport.CompositeTransportOptions{
		Stdio:          stdioTransport,
		StreamableHTTP: httpTransport,
	})
}

// NewLifecycleManager constructs the lifecycle manager.
func NewLifecycleManager(ctx context.Context, launcher domain.Launcher, transport domain.Transport, samplingHandler domain.SamplingHandler, elicitationHandler domain.ElicitationHandler, logger *zap.Logger) domain.Lifecycle {
	manager := lifecycle.NewManager(ctx, launcher, transport, logger)
	manager.SetSamplingHandler(samplingHandler)
	manager.SetElicitationHandler(elicitationHandler)
	return manager
}

// NewPluginManager constructs the governance plugin manager.
func NewPluginManager(logger *zap.Logger, metrics domain.Metrics) (*pluginmanager.Manager, error) {
	return pluginmanager.NewManager(pluginmanager.Options{
		Logger:  logger,
		Metrics: metrics,
	})
}

// NewPipelineEngine constructs the governance pipeline engine and applies initial specs.
func NewPipelineEngine(state *domain.CatalogState, manager *pluginmanager.Manager, metrics domain.Metrics, logger *zap.Logger) (*pipeline.Engine, error) {
	engine := pipeline.NewEngine(manager, logger, metrics)
	if state == nil || manager == nil {
		return engine, nil
	}
	if err := manager.Apply(context.Background(), state.Summary.Plugins); err != nil {
		return nil, err
	}
	engine.Update(state.Summary.Plugins)
	return engine, nil
}

// NewGovernanceExecutor constructs the governance executor.
func NewGovernanceExecutor(engine *pipeline.Engine) *governance.Executor {
	return governance.NewExecutor(engine)
}

// NewSamplingHandler builds a sampling handler using the SubAgent config.
func NewSamplingHandler(ctx context.Context, state *domain.CatalogState, logger *zap.Logger) domain.SamplingHandler {
	if state == nil {
		return nil
	}
	cfg := state.Summary.Runtime.SubAgent
	if cfg.Model == "" || cfg.Provider == "" {
		return nil
	}
	handler, err := sampling.NewHandler(ctx, cfg, logger)
	if err != nil {
		if logger != nil {
			logger.Warn("sampling handler disabled", zap.Error(err))
		}
		return nil
	}
	return handler
}

// NewElicitationHandler builds a default elicitation handler.
func NewElicitationHandler(logger *zap.Logger) domain.ElicitationHandler {
	return elicitation.NewDefaultHandler(logger)
}

// NewPingProbe constructs a ping-based health probe.
func NewPingProbe() *probe.PingProbe {
	return &probe.PingProbe{Timeout: defaultPingProbeTimeout}
}

// NewScheduler constructs the scheduler.
func NewScheduler(
	lifecycle domain.Lifecycle,
	state *domain.CatalogState,
	pingProbe *probe.PingProbe,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	logger *zap.Logger,
) (domain.Scheduler, error) {
	summary := state.Summary
	return scheduler.NewBasicScheduler(lifecycle, summary.SpecRegistry, scheduler.Options{
		Probe:   pingProbe,
		Logger:  logger,
		Metrics: metrics,
		Health:  health,
	})
}

// NewBootstrapManagerProvider constructs the bootstrap manager.
func NewBootstrapManagerProvider(
	lifecycle domain.Lifecycle,
	scheduler domain.Scheduler,
	state *domain.CatalogState,
	cache *domain.MetadataCache,
	logger *zap.Logger,
) *metadata.Manager {
	summary := state.Summary
	runtime := summary.Runtime

	mode := runtime.BootstrapMode
	if mode == "" {
		mode = domain.DefaultBootstrapMode
	}
	if mode == domain.BootstrapModeDisabled {
		return nil
	}

	concurrency := runtime.BootstrapConcurrency
	if concurrency <= 0 {
		concurrency = domain.DefaultBootstrapConcurrency
	}

	timeout := runtime.BootstrapTimeout()

	return metadata.NewManager(metadata.Options{
		Scheduler:   scheduler,
		Lifecycle:   lifecycle,
		Specs:       summary.SpecRegistry,
		SpecKeys:    summary.ServerSpecKeys,
		Runtime:     runtime,
		Cache:       cache,
		Logger:      logger,
		Concurrency: concurrency,
		Timeout:     timeout,
		Mode:        mode,
	})
}

// newRuntimeState constructs runtime state for the catalog.
func newRuntimeState(
	state *domain.CatalogState,
	scheduler domain.Scheduler,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	metadataCache *domain.MetadataCache,
	listChanges *notifications.ListChangeHub,
	logger *zap.Logger,
) *runtime.State {
	return runtime.NewState(state, scheduler, metrics, health, metadataCache, listChanges, logger)
}

// provideControlPlaneState constructs a control plane state container.
func provideControlPlaneState(
	ctx context.Context,
	runtimeState *runtime.State,
	state *domain.CatalogState,
	scheduler domain.Scheduler,
	startup *bootstrap.ServerStartupOrchestrator,
	logger *zap.Logger,
) *controlplane.State {
	controlState := controlplane.NewState(ctx, runtimeState, scheduler, startup, state, logger)

	if startup != nil && startup.HasBootstrap() && runtimeState != nil {
		waiter := func(ctx context.Context) error {
			return startup.WaitForBootstrap(ctx)
		}
		runtimeState.SetBootstrapWaiter(waiter)
	}

	return controlState
}

// NewRPCServer constructs the RPC server.
func NewRPCServer(control rpc.ControlPlaneAPI, executor *governance.Executor, state *domain.CatalogState, logger *zap.Logger) *rpc.Server {
	return rpc.NewServer(control, executor, state.Summary.Runtime.RPC, logger)
}
