package app

import (
	"context"

	"go.uber.org/zap"

	"mcpd/internal/infra/catalog"
)

func (a *App) ValidateConfig(ctx context.Context, cfg ValidateConfig) error {
	logging := NewLogging(LoggingConfig{
		Logger:      a.logger,
		Broadcaster: a.logBroadcaster,
	})
	logger := logging.Logger

	storeLoader := catalog.NewProfileStoreLoader(logger)
	store, err := storeLoader.Load(ctx, cfg.ConfigPath, catalog.ProfileStoreOptions{
		AllowCreate: false,
	})
	if err != nil {
		return err
	}

	if _, err := buildProfileSummary(store); err != nil {
		return err
	}

	logger.Info("configuration validated",
		zap.String("config", cfg.ConfigPath),
		zap.Int("profiles", len(store.Profiles)),
	)
	return nil
}
