package app

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/telemetry"
)

type App struct {
	logger         *zap.Logger
	logBroadcaster *telemetry.LogBroadcaster
	mu             sync.RWMutex
	reloadManager  *ReloadManager
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
	a.setReloadManager(application.reloadManager)
	defer a.setReloadManager(nil)
	return application.Run()
}

func (a *App) ReloadConfig(ctx context.Context) error {
	a.mu.RLock()
	manager := a.reloadManager
	a.mu.RUnlock()
	if manager == nil {
		return errors.New("reload manager not available")
	}
	return manager.Reload(ctx)
}

func (a *App) setReloadManager(manager *ReloadManager) {
	a.mu.Lock()
	a.reloadManager = manager
	a.mu.Unlock()
}
