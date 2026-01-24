package app

import (
	"context"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/catalog"
)

// ValidateConfig validates the configuration at the provided path.
func (a *App) ValidateConfig(ctx context.Context, cfg ValidateConfig) error {
	logging := NewLogging(LoggingConfig{
		Logger:      a.logger,
		Broadcaster: a.logBroadcaster,
	})
	logger := logging.Logger

	loader := catalog.NewLoader(logger)
	catalogData, err := loader.Load(ctx, cfg.ConfigPath)
	if err != nil {
		return err
	}

	summary, err := domain.BuildCatalogSummary(catalogData)
	if err != nil {
		return err
	}

	logger.Info("configuration validated",
		zap.String("config", cfg.ConfigPath),
		zap.Int("servers", summary.TotalServers),
	)
	return nil
}
