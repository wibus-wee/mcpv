package app

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/aggregator"
	"mcpd/internal/infra/lifecycle"
	"mcpd/internal/infra/notifications"
	"mcpd/internal/infra/probe"
	"mcpd/internal/infra/router"
	"mcpd/internal/infra/rpc"
	"mcpd/internal/infra/scheduler"
	"mcpd/internal/infra/telemetry"
	"mcpd/internal/infra/transport"
)

func NewMetricsRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	registry.MustRegister(prometheus.NewGoCollector())
	return registry
}

func NewMetrics(registry *prometheus.Registry) domain.Metrics {
	return telemetry.NewPrometheusMetrics(registry)
}

func NewHealthTracker() *telemetry.HealthTracker {
	return telemetry.NewHealthTracker()
}

func NewListChangeHub() *notifications.ListChangeHub {
	return notifications.NewListChangeHub()
}

func NewCommandLauncher(logger *zap.Logger) domain.Launcher {
	return transport.NewCommandLauncher(transport.CommandLauncherOptions{Logger: logger})
}

func NewMCPTransport(logger *zap.Logger, listChanges *notifications.ListChangeHub) domain.Transport {
	return transport.NewMCPTransport(transport.MCPTransportOptions{
		Logger:            logger,
		ListChangeEmitter: listChanges,
	})
}

func NewLifecycleManager(ctx context.Context, launcher domain.Launcher, transport domain.Transport, logger *zap.Logger) domain.Lifecycle {
	return lifecycle.NewManager(ctx, launcher, transport, logger)
}

func NewPingProbe() *probe.PingProbe {
	return &probe.PingProbe{Timeout: defaultPingProbeTimeout}
}

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

func NewProfileRuntimes(
	state *domain.CatalogState,
	scheduler domain.Scheduler,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	listChanges *notifications.ListChangeHub,
	logger *zap.Logger,
) map[string]*profileRuntime {
	summary := state.Summary
	profiles := make(map[string]*profileRuntime, len(summary.Profiles))
	for name, cfg := range summary.Profiles {
		profiles[name] = buildProfileRuntime(name, cfg, scheduler, metrics, health, listChanges, logger)
	}
	return profiles
}

func NewControlPlaneState(
	ctx context.Context,
	profiles map[string]*profileRuntime,
	state *domain.CatalogState,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
	logger *zap.Logger,
) *controlPlaneState {
	return newControlPlaneState(ctx, profiles, scheduler, initManager, state, logger)
}

func NewRPCServer(control domain.ControlPlane, state *domain.CatalogState, logger *zap.Logger) *rpc.Server {
	return rpc.NewServer(control, state.Summary.DefaultRuntime.RPC, logger)
}

func buildProfileRuntime(
	name string,
	cfg domain.CatalogProfile,
	scheduler domain.Scheduler,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	listChanges *notifications.ListChangeHub,
	logger *zap.Logger,
) *profileRuntime {
	profileLogger := logger.With(zap.String("profile", name))
	refreshGate := aggregator.NewRefreshGate()
	baseRouter := router.NewBasicRouter(scheduler, router.RouterOptions{
		Timeout: time.Duration(cfg.Profile.Catalog.Runtime.RouteTimeoutSeconds) * time.Second,
		Logger:  profileLogger,
	})
	rt := router.NewMetricRouter(baseRouter, metrics)
	toolIndex := aggregator.NewToolIndex(rt, cfg.Profile.Catalog.Specs, cfg.SpecKeys, cfg.Profile.Catalog.Runtime, profileLogger, health, refreshGate, listChanges)
	resourceIndex := aggregator.NewResourceIndex(rt, cfg.Profile.Catalog.Specs, cfg.SpecKeys, cfg.Profile.Catalog.Runtime, profileLogger, health, refreshGate, listChanges)
	promptIndex := aggregator.NewPromptIndex(rt, cfg.Profile.Catalog.Specs, cfg.SpecKeys, cfg.Profile.Catalog.Runtime, profileLogger, health, refreshGate, listChanges)
	return &profileRuntime{
		name:      name,
		specKeys:  collectSpecKeys(cfg.SpecKeys),
		tools:     toolIndex,
		resources: resourceIndex,
		prompts:   promptIndex,
	}
}
