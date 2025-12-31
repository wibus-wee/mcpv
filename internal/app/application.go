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

	logger       *zap.Logger
	registry     *prometheus.Registry
	metrics      domain.Metrics
	health       *telemetry.HealthTracker
	summary      profileSummary
	profiles     map[string]*profileRuntime
	scheduler    domain.Scheduler
	initManager  *ServerInitializationManager
	controlPlane *ControlPlane
	rpcServer    *rpc.Server
}

func NewApplication(
	ctx context.Context,
	cfg ServeConfig,
	logger *zap.Logger,
	registry *prometheus.Registry,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	snapshot *CatalogSnapshot,
	profiles map[string]*profileRuntime,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
	controlPlane *ControlPlane,
	rpcServer *rpc.Server,
) *Application {
	if ctx == nil {
		ctx = context.Background()
	}
	summary := snapshot.Summary()
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
		profiles:      profiles,
		scheduler:     scheduler,
		initManager:   initManager,
		controlPlane:  controlPlane,
		rpcServer:     rpcServer,
	}
}

func (a *Application) Run() error {
	a.logger.Info("configuration loaded",
		zap.String("config", a.configPath),
		zap.Int("profiles", len(a.summary.configs)),
		zap.Int("servers", a.summary.totalServers),
	)

	a.initManager.Start(a.ctx)

	if a.summary.defaultRuntime.SubAgent.Model != "" && a.summary.defaultRuntime.SubAgent.Provider != "" {
		subAgent, err := initializeSubAgent(a.ctx, a.summary.defaultRuntime.SubAgent, a.controlPlane, a.metrics, a.logger)
		if err != nil {
			a.logger.Warn("failed to initialize SubAgent", zap.Error(err))
		} else {
			a.controlPlane.SetSubAgent(subAgent)
			a.logger.Info("SubAgent initialized",
				zap.String("provider", a.summary.defaultRuntime.SubAgent.Provider),
				zap.String("model", a.summary.defaultRuntime.SubAgent.Model),
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
			addr := a.summary.defaultRuntime.Observability.ListenAddress
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
	if a.summary.minPingInterval > 0 {
		a.scheduler.StartPingManager(time.Duration(a.summary.minPingInterval) * time.Second)
	}
	defer func() {
		for _, runtime := range a.profiles {
			runtime.Deactivate()
		}
		a.initManager.Stop()
		a.scheduler.StopPingManager()
		a.scheduler.StopIdleManager()
		a.scheduler.StopAll(context.Background())
	}()

	return a.rpcServer.Run(a.ctx)
}
