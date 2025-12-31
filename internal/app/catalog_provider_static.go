package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/catalog"
)

type StaticCatalogProvider struct {
	state domain.CatalogState
}

func NewStaticCatalogProvider(ctx context.Context, cfg ServeConfig, logger *zap.Logger) (*StaticCatalogProvider, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	loader := catalog.NewProfileStoreLoader(logger)
	store, err := loader.Load(ctx, cfg.ConfigPath, catalog.ProfileStoreOptions{
		AllowCreate: true,
	})
	if err != nil {
		return nil, err
	}
	state, err := domain.NewCatalogState(store, 1, time.Now())
	if err != nil {
		return nil, err
	}
	return &StaticCatalogProvider{state: state}, nil
}

func (p *StaticCatalogProvider) Snapshot(ctx context.Context) (domain.CatalogState, error) {
	if ctx == nil {
		return p.state, nil
	}
	if err := ctx.Err(); err != nil {
		return domain.CatalogState{}, err
	}
	return p.state, nil
}

func (p *StaticCatalogProvider) Watch(ctx context.Context) (<-chan domain.CatalogUpdate, error) {
	ch := make(chan domain.CatalogUpdate)
	close(ch)
	return ch, nil
}

func (p *StaticCatalogProvider) Reload(ctx context.Context) error {
	return nil
}
