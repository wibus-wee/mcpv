package app

import (
	"context"

	"go.uber.org/zap"
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
	return &App{
		logger: logger.Named("app"),
	}
}

func (a *App) Serve(ctx context.Context, cfg ServeConfig) error {
	a.logger.Info("serve stub called", zap.String("config", cfg.ConfigPath))
	return nil
}

func (a *App) ValidateConfig(ctx context.Context, cfg ValidateConfig) error {
	a.logger.Info("validate stub called", zap.String("config", cfg.ConfigPath))
	return nil
}
