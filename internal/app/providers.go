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
	snapshot *CatalogSnapshot,
	pingProbe *probe.PingProbe,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	logger *zap.Logger,
) (domain.Scheduler, error) {
	summary := snapshot.Summary()
	return scheduler.NewBasicScheduler(lifecycle, summary.specRegistry, scheduler.SchedulerOptions{
		Probe:   pingProbe,
		Logger:  logger,
		Metrics: metrics,
		Health:  health,
	})
}

func NewProfileRuntimes(
	snapshot *CatalogSnapshot,
	scheduler domain.Scheduler,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	listChanges *notifications.ListChangeHub,
	logger *zap.Logger,
) map[string]*profileRuntime {
	summary := snapshot.Summary()
	profiles := make(map[string]*profileRuntime, len(summary.configs))
	for name, cfg := range summary.configs {
		profileLogger := logger.With(zap.String("profile", name))
		refreshGate := aggregator.NewRefreshGate()
		baseRouter := router.NewBasicRouter(scheduler, router.RouterOptions{
			Timeout: time.Duration(cfg.profile.Catalog.Runtime.RouteTimeoutSeconds) * time.Second,
			Logger:  profileLogger,
		})
		rt := router.NewMetricRouter(baseRouter, metrics)
		toolIndex := aggregator.NewToolIndex(rt, cfg.profile.Catalog.Specs, cfg.specKeys, cfg.profile.Catalog.Runtime, profileLogger, health, refreshGate, listChanges)
		resourceIndex := aggregator.NewResourceIndex(rt, cfg.profile.Catalog.Specs, cfg.specKeys, cfg.profile.Catalog.Runtime, profileLogger, health, refreshGate, listChanges)
		promptIndex := aggregator.NewPromptIndex(rt, cfg.profile.Catalog.Specs, cfg.specKeys, cfg.profile.Catalog.Runtime, profileLogger, health, refreshGate, listChanges)
		profiles[name] = &profileRuntime{
			name:      name,
			specKeys:  collectSpecKeys(cfg.specKeys),
			tools:     toolIndex,
			resources: resourceIndex,
			prompts:   promptIndex,
		}
	}
	return profiles
}

func NewControlPlaneState(
	ctx context.Context,
	profiles map[string]*profileRuntime,
	snapshot *CatalogSnapshot,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
	logger *zap.Logger,
) *controlPlaneState {
	return newControlPlaneState(ctx, profiles, scheduler, initManager, snapshot.Store(), snapshot.Summary(), logger)
}

func NewRPCServer(control domain.ControlPlane, snapshot *CatalogSnapshot, logger *zap.Logger) *rpc.Server {
	return rpc.NewServer(control, snapshot.Summary().defaultRuntime.RPC, logger)
}
