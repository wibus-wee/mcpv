package app

import (
	"context"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/catalog"
)

type StaticCatalogAccessor struct {
	store domain.ProfileStore
}

func NewStaticCatalogAccessor(ctx context.Context, cfg ServeConfig, logger *zap.Logger) (*StaticCatalogAccessor, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	loader := catalog.NewProfileStoreLoader(logger)
	store, err := loader.Load(ctx, cfg.ConfigPath, catalog.ProfileStoreOptions{
		AllowCreate: true,
	})
	if err != nil {
		return nil, err
	}
	return &StaticCatalogAccessor{store: store}, nil
}

func (a *StaticCatalogAccessor) GetProfileStore() (domain.ProfileStore, error) {
	return a.store, nil
}
