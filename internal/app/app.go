package app

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"

	"mcpv/internal/app/controlplane"
	"mcpv/internal/infra/plugin"
	"mcpv/internal/infra/telemetry"
)

// App wires core services and provides lifecycle entry points.
type App struct {
	logger         *zap.Logger
	logBroadcaster *telemetry.LogBroadcaster
	mu             sync.RWMutex
	reloadManager  *controlplane.ReloadManager
	application    *Application
}

// ServeConfig describes how to start the core application.
type ServeConfig struct {
	ConfigPath    string
	OnReady       func(controlplane.API) // Called when Core is ready (after RPC server starts).
	Observability *ObservabilityOptions
}

// ValidateConfig describes how to validate configuration.
type ValidateConfig struct {
	ConfigPath string
}

// ObservabilityOptions toggles observability features.
type ObservabilityOptions struct {
	MetricsEnabled *bool
	HealthzEnabled *bool
}

// New constructs an App with the provided logger.
func New(logger *zap.Logger) *App {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &App{
		logger: logger,
	}
}

// NewWithBroadcaster constructs an App with a log broadcaster.
func NewWithBroadcaster(logger *zap.Logger, broadcaster *telemetry.LogBroadcaster) *App {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &App{
		logger:         logger,
		logBroadcaster: broadcaster,
	}
}

// Serve starts the core application.
func (a *App) Serve(ctx context.Context, cfg ServeConfig) error {
	a.logger.Info("core initialization started", zap.String("config", cfg.ConfigPath))
	application, err := InitializeApplication(ctx, cfg, LoggingConfig{
		Logger:      a.logger,
		Broadcaster: a.logBroadcaster,
	})
	if err != nil {
		a.logger.Error("core initialization failed", zap.Error(err))
		return err
	}
	a.logger.Info("core initialization complete")
	a.setApplication(application)
	defer a.setApplication(nil)
	return application.Run()
}

// ReloadConfig triggers a configuration reload.
func (a *App) ReloadConfig(ctx context.Context) error {
	a.mu.RLock()
	manager := a.reloadManager
	a.mu.RUnlock()
	if manager == nil {
		return errors.New("reload manager not available")
	}
	return manager.Reload(ctx)
}

func (a *App) setApplication(application *Application) {
	a.mu.Lock()
	if application != nil {
		a.reloadManager = application.reloadManager
		a.application = application
	} else {
		a.reloadManager = nil
		a.application = nil
	}
	a.mu.Unlock()
}

// GetPluginStatus returns the runtime status of all configured plugins.
func (a *App) GetPluginStatus() []plugin.Status {
	a.mu.RLock()
	application := a.application
	a.mu.RUnlock()
	if application == nil {
		return nil
	}
	return application.GetPluginStatus()
}

// IsPluginRunning checks if a specific plugin is currently running.
func (a *App) IsPluginRunning(name string) bool {
	a.mu.RLock()
	application := a.application
	a.mu.RUnlock()
	if application == nil {
		return false
	}
	return application.IsPluginRunning(name)
}
