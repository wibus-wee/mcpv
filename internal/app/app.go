package app

import (
	"context"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"mcpd/internal/infra/aggregator"
	"mcpd/internal/infra/catalog"
	"mcpd/internal/infra/lifecycle"
	"mcpd/internal/infra/probe"
	"mcpd/internal/infra/router"
	"mcpd/internal/infra/rpc"
	"mcpd/internal/infra/scheduler"
	"mcpd/internal/infra/telemetry"
	"mcpd/internal/infra/transport"
)

type App struct {
	logger *zap.Logger
}

type ServeConfig struct {
	ConfigPath string
}

type ValidateConfig struct {
	ConfigPath string
}

func New(logger *zap.Logger) *App {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &App{
		logger: logger.Named("app"),
	}
}

func (a *App) Serve(ctx context.Context, cfg ServeConfig) error {
	logs := telemetry.NewLogBroadcaster(zapcore.DebugLevel)
	logger := a.logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, logs.Core())
	}))
	loader := catalog.NewLoader(logger)

	catalogData, err := loader.Load(ctx, cfg.ConfigPath)
	if err != nil {
		return err
	}

	logger.Info("configuration loaded", zap.String("config", cfg.ConfigPath), zap.Int("servers", len(catalogData.Specs)))

	stdioTransport := transport.NewStdioTransport()
	lc := lifecycle.NewManager(stdioTransport, logger)
	pingProbe := &probe.PingProbe{Timeout: 2 * time.Second}
	metrics := telemetry.NewPrometheusMetrics()
	health := telemetry.NewHealthTracker()
	sched := scheduler.NewBasicScheduler(lc, catalogData.Specs, scheduler.SchedulerOptions{
		Probe:   pingProbe,
		Logger:  logger,
		Metrics: metrics,
		Health:  health,
	})
	rt := router.NewBasicRouter(sched, router.RouterOptions{
		Timeout: time.Duration(catalogData.Runtime.RouteTimeoutSeconds) * time.Second,
		Logger:  logger,
		Metrics: metrics,
	})
	toolIndex := aggregator.NewToolIndex(rt, catalogData.Specs, catalogData.Runtime, logger, health)
	control := NewControlPlane(toolIndex, logs)
	rpcServer := rpc.NewServer(control, catalogData.Runtime.RPC, logger)

	metricsEnabled := envBool("MCPD_METRICS_ENABLED")
	healthzEnabled := envBool("MCPD_HEALTHZ_ENABLED")
	if metricsEnabled || healthzEnabled {
		go func() {
			addr := catalogData.Runtime.Observability.ListenAddress
			logger.Info("starting observability server", zap.String("addr", addr))
			if err := telemetry.StartHTTPServer(ctx, telemetry.HTTPServerOptions{
				Addr:          addr,
				EnableMetrics: metricsEnabled,
				EnableHealthz: healthzEnabled,
				Health:        health,
			}, logger); err != nil {
				logger.Error("observability server failed", zap.Error(err))
			}
		}()
	}

	sched.StartIdleManager(time.Second)
	sched.StartPingManager(time.Duration(catalogData.Runtime.PingIntervalSeconds) * time.Second)
	toolIndex.Start(ctx)
	defer func() {
		toolIndex.Stop()
		sched.StopPingManager()
		sched.StopIdleManager()
		sched.StopAll(context.Background())
	}()

	return rpcServer.Run(ctx)
}

func (a *App) ValidateConfig(ctx context.Context, cfg ValidateConfig) error {
	loader := catalog.NewLoader(a.logger)

	catalogData, err := loader.Load(ctx, cfg.ConfigPath)
	if err != nil {
		return err
	}

	a.logger.Info("configuration validated", zap.String("config", cfg.ConfigPath), zap.Int("servers", len(catalogData.Specs)))
	return nil
}

func envBool(key string) bool {
	val := strings.TrimSpace(os.Getenv(key))
	return val == "1" || strings.EqualFold(val, "true")
}
