package app

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/app/controlplane"
	"mcpv/internal/domain"
	pluginmanager "mcpv/internal/infra/plugin/manager"
	"mcpv/internal/infra/rpc"
	"mcpv/internal/infra/telemetry"
	"mcpv/internal/infra/telemetry/diagnostics"
)

// Application wires the core runtime and dependencies.
type Application struct {
	ctx           context.Context
	configPath    string
	onReady       func(controlplane.API)
	observability *ObservabilityOptions

	logger        *zap.Logger
	registry      *prometheus.Registry
	metrics       domain.Metrics
	health        *telemetry.HealthTracker
	diagnostics   *diagnostics.Hub
	summary       domain.CatalogSummary
	state         *controlplane.State
	scheduler     domain.Scheduler
	startup       *bootstrap.ServerStartupOrchestrator
	controlPlane  *controlplane.ControlPlane
	rpcServer     *rpc.Server
	reloadManager *controlplane.ReloadManager
	pluginManager *pluginmanager.Manager
}

// ApplicationOptions captures dependencies and settings for Application.
type ApplicationOptions struct {
	Context           context.Context
	ServeConfig       ServeConfig
	Logger            *zap.Logger
	Registry          *prometheus.Registry
	Metrics           domain.Metrics
	Health            *telemetry.HealthTracker
	Diagnostics       *diagnostics.Hub
	CatalogState      *domain.CatalogState
	ControlPlaneState *controlplane.State
	Scheduler         domain.Scheduler
	Startup           *bootstrap.ServerStartupOrchestrator
	ControlPlane      *controlplane.ControlPlane
	RPCServer         *rpc.Server
	ReloadManager     *controlplane.ReloadManager
	PluginManager     *pluginmanager.Manager
}

// NewApplication constructs the core application runtime.
func NewApplication(opts ApplicationOptions) *Application {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	var summary domain.CatalogSummary
	if opts.CatalogState != nil {
		summary = opts.CatalogState.Summary
	}
	return &Application{
		ctx:           ctx,
		configPath:    opts.ServeConfig.ConfigPath,
		onReady:       opts.ServeConfig.OnReady,
		observability: opts.ServeConfig.Observability,
		logger:        opts.Logger,
		registry:      opts.Registry,
		metrics:       opts.Metrics,
		health:        opts.Health,
		diagnostics:   opts.Diagnostics,
		summary:       summary,
		state:         opts.ControlPlaneState,
		scheduler:     opts.Scheduler,
		startup:       opts.Startup,
		controlPlane:  opts.ControlPlane,
		rpcServer:     opts.RPCServer,
		reloadManager: opts.ReloadManager,
		pluginManager: opts.PluginManager,
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

	if a.startup != nil {
		a.startup.Bootstrap(a.ctx)
	}
	if a.state != nil {
		if runtime := a.state.RuntimeState(); runtime != nil {
			runtime.Activate(a.ctx)
		}
	}
	if a.startup != nil {
		a.startup.StartInit(a.ctx)
	}

	metricsEnabled, healthzEnabled := resolveObservabilityDefaults(a.observability)
	obsController := telemetry.NewObservabilityController(telemetry.ObservabilityControllerOptions{
		DefaultMetricsEnabled: metricsEnabled,
		DefaultHealthzEnabled: healthzEnabled,
		Registry:              a.registry,
		Health:                a.health,
		Logger:                a.logger,
	})
	if a.reloadManager != nil {
		a.reloadManager.SetObservabilityController(obsController)
	}
	if err := obsController.Apply(a.ctx, a.summary.Runtime.Observability); err != nil {
		a.logger.Warn("observability apply failed", zap.Error(err))
	}

	if a.reloadManager != nil {
		if err := a.reloadManager.Start(a.ctx); err != nil {
			a.logger.Warn("reload manager start failed", zap.Error(err))
		}
	}

	subAgentConfig := a.summary.Runtime.SubAgent
	if subAgentConfig.Enabled && len(subAgentConfig.EnabledTags) > 0 && subAgentConfig.Model != "" && subAgentConfig.Provider != "" {
		subAgent, err := controlplane.InitializeSubAgent(a.ctx, subAgentConfig, a.controlPlane, a.metrics, a.logger)
		if err != nil {
			a.logger.Warn("failed to initialize SubAgent", zap.Error(err))
		} else {
			a.controlPlane.SetSubAgent(subAgent)
			a.logger.Info("SubAgent initialized",
				zap.String("provider", subAgentConfig.Provider),
				zap.String("model", subAgentConfig.Model),
			)
		}
	}

	a.controlPlane.StartClientMonitor(a.ctx)

	a.scheduler.StartIdleManager(defaultIdleManagerInterval)
	if interval := a.summary.Runtime.PingInterval(); interval > 0 {
		a.scheduler.StartPingManager(interval)
	}
	defer func() {
		if a.pluginManager != nil {
			a.pluginManager.Stop(context.Background())
		}
		if a.state != nil {
			if runtime := a.state.RuntimeState(); runtime != nil {
				runtime.Deactivate()
			}
		}
		if a.startup != nil {
			// Wait for bootstrap to complete before shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = a.startup.WaitForBootstrap(ctx)
			a.startup.StopInit()
		}
		a.scheduler.StopPingManager()
		a.scheduler.StopIdleManager()
		a.scheduler.StopAll(context.Background())
	}()

	return a.rpcServer.Run(a.ctx)
}

// GetPluginStatus returns the runtime status of all configured plugins.
func (a *Application) GetPluginStatus() []pluginmanager.Status {
	if a.pluginManager == nil {
		return nil
	}
	return a.pluginManager.GetStatus(a.summary.Plugins)
}

// IsPluginRunning checks if a specific plugin is currently running.
func (a *Application) IsPluginRunning(name string) bool {
	if a.pluginManager == nil {
		return false
	}
	return a.pluginManager.IsRunning(name)
}

// Diagnostics returns the diagnostics hub if configured.
func (a *Application) Diagnostics() *diagnostics.Hub {
	if a == nil {
		return nil
	}
	return a.diagnostics
}
