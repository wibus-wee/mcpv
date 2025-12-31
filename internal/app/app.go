package app

import (
	"context"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/telemetry"
)

type App struct {
	logger         *zap.Logger
	logBroadcaster *telemetry.LogBroadcaster
}

type ServeConfig struct {
	ConfigPath    string
	OnReady       func(domain.ControlPlane) // Called when Core is ready (after RPC server starts)
	Observability *ObservabilityOptions
}

type ValidateConfig struct {
	ConfigPath string
}

type ObservabilityOptions struct {
	MetricsEnabled *bool
	HealthzEnabled *bool
}

func New(logger *zap.Logger) *App {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &App{
		logger: logger,
	}
}

func NewWithBroadcaster(logger *zap.Logger, broadcaster *telemetry.LogBroadcaster) *App {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &App{
		logger:         logger,
		logBroadcaster: broadcaster,
	}
}

func (a *App) Serve(ctx context.Context, cfg ServeConfig) error {
	application, err := InitializeApplication(ctx, cfg, LoggingConfig{
		Logger:      a.logger,
		Broadcaster: a.logBroadcaster,
	})
	if err != nil {
		return err
	}
	return application.Run()
}
