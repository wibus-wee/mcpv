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

// Application wires the core runtime and dependencies.
type Application struct {
	ctx           context.Context
	configPath    string
	onReady       func(domain.ControlPlane)
	observability *ObservabilityOptions

	logger           *zap.Logger
	registry         *prometheus.Registry
	metrics          domain.Metrics
	health           *telemetry.HealthTracker
	summary          domain.CatalogSummary
	state            *controlPlaneState
	scheduler        domain.Scheduler
	initManager      *ServerInitializationManager
	bootstrapManager *BootstrapManager
	controlPlane     *ControlPlane
	rpcServer        *rpc.Server
	reloadManager    *ReloadManager
}

// NewApplication constructs the core application runtime.
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
	bootstrapManager *BootstrapManager,
	controlPlane *ControlPlane,
	rpcServer *rpc.Server,
	reloadManager *ReloadManager,
) *Application {
	if ctx == nil {
		ctx = context.Background()
	}
	summary := state.Summary
	return &Application{
		ctx:              ctx,
		configPath:       cfg.ConfigPath,
		onReady:          cfg.OnReady,
		observability:    cfg.Observability,
		logger:           logger,
		registry:         registry,
		metrics:          metrics,
		health:           health,
		summary:          summary,
		state:            controlState,
		scheduler:        scheduler,
		initManager:      initManager,
		bootstrapManager: bootstrapManager,
		controlPlane:     controlPlane,
		rpcServer:        rpcServer,
		reloadManager:    reloadManager,
	}
}

// Run starts the core services and blocks until shutdown.
func (a *Application) Run() error {
	a.logger.Info("configuration loaded",
		zap.String("config", a.configPath),
		zap.Int("servers", a.summary.TotalServers),
	)

	// Open UI immediately (before bootstrap)
	if a.onReady != nil {
		a.onReady(a.controlPlane)
	}

	if a.bootstrapManager != nil {
		a.bootstrapManager.Bootstrap(a.ctx)
	}
	if a.initManager != nil {
		a.initManager.Start(a.ctx)
	}

	if a.reloadManager != nil {
		if err := a.reloadManager.Start(a.ctx); err != nil {
			a.logger.Warn("reload manager start failed", zap.Error(err))
		}
	}

	if a.summary.Runtime.SubAgent.Model != "" && a.summary.Runtime.SubAgent.Provider != "" {
		subAgent, err := initializeSubAgent(a.ctx, a.summary.Runtime.SubAgent, a.controlPlane, a.metrics, a.logger)
		if err != nil {
			a.logger.Warn("failed to initialize SubAgent", zap.Error(err))
		} else {
			a.controlPlane.SetSubAgent(subAgent)
			a.logger.Info("SubAgent initialized",
				zap.String("provider", a.summary.Runtime.SubAgent.Provider),
				zap.String("model", a.summary.Runtime.SubAgent.Model),
			)
		}
	}

	a.controlPlane.StartClientMonitor(a.ctx)

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
			addr := a.summary.Runtime.Observability.ListenAddress
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
	if a.summary.Runtime.PingIntervalSeconds > 0 {
		a.scheduler.StartPingManager(time.Duration(a.summary.Runtime.PingIntervalSeconds) * time.Second)
	}
	defer func() {
		if a.state != nil {
			if runtime := a.state.RuntimeState(); runtime != nil {
				runtime.Deactivate()
			}
		}
		if a.bootstrapManager != nil {
			// Wait for bootstrap to complete before shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = a.bootstrapManager.WaitForCompletion(ctx)
		}
		if a.initManager != nil {
			a.initManager.Stop()
		}
		a.scheduler.StopPingManager()
		a.scheduler.StopIdleManager()
		a.scheduler.StopAll(context.Background())
	}()

	return a.rpcServer.Run(a.ctx)
}
