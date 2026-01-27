package app

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
	"mcpd/internal/infra/elicitation"
	"mcpd/internal/infra/lifecycle"
	"mcpd/internal/infra/notifications"
	"mcpd/internal/infra/probe"
	"mcpd/internal/infra/router"
	"mcpd/internal/infra/rpc"
	"mcpd/internal/infra/sampling"
	"mcpd/internal/infra/scheduler"
	"mcpd/internal/infra/telemetry"
	"mcpd/internal/infra/transport"
)

// NewMetricsRegistry creates a Prometheus registry.
func NewMetricsRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	registry.MustRegister(prometheus.NewGoCollector())
	return registry
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
	return scheduler.NewBasicScheduler(lifecycle, summary.SpecRegistry, scheduler.SchedulerOptions{
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
) *BootstrapManager {
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

	timeout := time.Duration(runtime.BootstrapTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(domain.DefaultBootstrapTimeoutSeconds) * time.Second
	}

	return NewBootstrapManager(BootstrapManagerOptions{
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

// NewRuntimeState constructs runtime state for the catalog.
func NewRuntimeState(
	state *domain.CatalogState,
	scheduler domain.Scheduler,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	metadataCache *domain.MetadataCache,
	listChanges *notifications.ListChangeHub,
	logger *zap.Logger,
) *runtimeState {
	return buildRuntimeState(state, scheduler, metrics, health, metadataCache, listChanges, logger)
}

// NewControlPlaneState constructs a control plane state container.
func NewControlPlaneState(
	ctx context.Context,
	runtime *runtimeState,
	state *domain.CatalogState,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
	bootstrapManager *BootstrapManager,
	logger *zap.Logger,
) *controlPlaneState {
	controlState := newControlPlaneState(ctx, runtime, scheduler, initManager, bootstrapManager, state, logger)

	if bootstrapManager != nil && runtime != nil {
		waiter := func(ctx context.Context) error {
			return bootstrapManager.WaitForCompletion(ctx)
		}
		if runtime.tools != nil {
			runtime.tools.SetBootstrapWaiter(waiter)
		}
		if runtime.resources != nil {
			runtime.resources.SetBootstrapWaiter(waiter)
		}
		if runtime.prompts != nil {
			runtime.prompts.SetBootstrapWaiter(waiter)
		}
	}

	return controlState
}

// NewRPCServer constructs the RPC server.
func NewRPCServer(control domain.ControlPlane, state *domain.CatalogState, logger *zap.Logger) *rpc.Server {
	return rpc.NewServer(control, state.Summary.Runtime.RPC, logger)
}

func buildRuntimeState(
	state *domain.CatalogState,
	scheduler domain.Scheduler,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	metadataCache *domain.MetadataCache,
	listChanges *notifications.ListChangeHub,
	logger *zap.Logger,
) *runtimeState {
	if logger == nil {
		logger = zap.NewNop()
	}
	refreshGate := aggregator.NewRefreshGate()
	baseRouter := router.NewBasicRouter(scheduler, router.RouterOptions{
		Timeout: time.Duration(state.Summary.Runtime.RouteTimeoutSeconds) * time.Second,
		Logger:  logger,
	})
	rt := router.NewMetricRouter(baseRouter, metrics)
	toolIndex := aggregator.NewToolIndex(rt, state.Catalog.Specs, state.Summary.ServerSpecKeys, state.Summary.Runtime, metadataCache, logger, health, refreshGate, listChanges)
	resourceIndex := aggregator.NewResourceIndex(rt, state.Catalog.Specs, state.Summary.ServerSpecKeys, state.Summary.Runtime, metadataCache, logger, health, refreshGate, listChanges)
	promptIndex := aggregator.NewPromptIndex(rt, state.Catalog.Specs, state.Summary.ServerSpecKeys, state.Summary.Runtime, metadataCache, logger, health, refreshGate, listChanges)
	return &runtimeState{
		specKeys:      copySpecKeyMap(state.Summary.ServerSpecKeys),
		metadataCache: metadataCache,
		tools:         toolIndex,
		resources:     resourceIndex,
		prompts:       promptIndex,
	}
}
