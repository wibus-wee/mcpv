package app

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/rpc"
	"mcpd/internal/infra/telemetry"
)

type Application struct {
	ctx           context.Context
	configPath    string
	onReady       func(domain.ControlPlane)
	observability *ObservabilityOptions

	logger        *zap.Logger
	registry      *prometheus.Registry
	metrics       domain.Metrics
	health        *telemetry.HealthTracker
	summary       domain.CatalogSummary
	state         *controlPlaneState
	scheduler     domain.Scheduler
	initManager   *ServerInitializationManager
	controlPlane  *ControlPlane
	rpcServer     *rpc.Server
	reloadManager *ReloadManager
}

func NewApplication(
	ctx context.Context,
	cfg ServeConfig,
	logger *zap.Logger,
	registry *prometheus.Registry,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	state *domain.CatalogState,
	controlState *controlPlaneState,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
	controlPlane *ControlPlane,
	rpcServer *rpc.Server,
	reloadManager *ReloadManager,
) *Application {
	if ctx == nil {
		ctx = context.Background()
	}
	summary := state.Summary
	return &Application{
		ctx:           ctx,
		configPath:    cfg.ConfigPath,
		onReady:       cfg.OnReady,
		observability: cfg.Observability,
		logger:        logger,
		registry:      registry,
		metrics:       metrics,
		health:        health,
		summary:       summary,
		state:         controlState,
		scheduler:     scheduler,
		initManager:   initManager,
		controlPlane:  controlPlane,
		rpcServer:     rpcServer,
		reloadManager: reloadManager,
	}
}

func (a *Application) Run() error {
	a.logger.Info("configuration loaded",
		zap.String("config", a.configPath),
		zap.Int("profiles", len(a.summary.Profiles)),
		zap.Int("servers", a.summary.TotalServers),
	)

	a.initManager.Start(a.ctx)
	if a.reloadManager != nil {
		if err := a.reloadManager.Start(a.ctx); err != nil {
			a.logger.Warn("reload manager start failed", zap.Error(err))
		}
	}

	if a.summary.DefaultRuntime.SubAgent.Model != "" && a.summary.DefaultRuntime.SubAgent.Provider != "" {
		subAgent, err := initializeSubAgent(a.ctx, a.summary.DefaultRuntime.SubAgent, a.controlPlane, a.metrics, a.logger)
		if err != nil {
			a.logger.Warn("failed to initialize SubAgent", zap.Error(err))
		} else {
			a.controlPlane.SetSubAgent(subAgent)
			a.logger.Info("SubAgent initialized",
				zap.String("provider", a.summary.DefaultRuntime.SubAgent.Provider),
				zap.String("model", a.summary.DefaultRuntime.SubAgent.Model),
			)
		}
	}

	if a.onReady != nil {
		a.onReady(a.controlPlane)
	}
	a.controlPlane.StartCallerMonitor(a.ctx)

	metricsEnabled := envBool("MCPD_METRICS_ENABLED")
	healthzEnabled := envBool("MCPD_HEALTHZ_ENABLED")
	if a.observability != nil {
		if a.observability.MetricsEnabled != nil {
			metricsEnabled = *a.observability.MetricsEnabled
		}
		if a.observability.HealthzEnabled != nil {
			healthzEnabled = *a.observability.HealthzEnabled
		}
	}
	if metricsEnabled || healthzEnabled {
		go func() {
			addr := a.summary.DefaultRuntime.Observability.ListenAddress
			a.logger.Info("starting observability server", zap.String("addr", addr))
			if err := telemetry.StartHTTPServer(a.ctx, telemetry.HTTPServerOptions{
				Addr:          addr,
				EnableMetrics: metricsEnabled,
				EnableHealthz: healthzEnabled,
				Health:        a.health,
				Registry:      a.registry,
			}, a.logger); err != nil {
				a.logger.Error("observability server failed", zap.Error(err))
			}
		}()
	}

	a.scheduler.StartIdleManager(defaultIdleManagerInterval)
	if a.summary.MinPingInterval > 0 {
		a.scheduler.StartPingManager(time.Duration(a.summary.MinPingInterval) * time.Second)
	}
	defer func() {
		if a.state != nil {
			for _, runtime := range a.state.Profiles() {
				runtime.Deactivate()
			}
		}
		a.initManager.Stop()
		a.scheduler.StopPingManager()
		a.scheduler.StopIdleManager()
		a.scheduler.StopAll(context.Background())
	}()

	return a.rpcServer.Run(a.ctx)
}
