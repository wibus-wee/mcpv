package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/catalog"
)

// StaticCatalogProvider serves an immutable catalog snapshot.
type StaticCatalogProvider struct {
	state domain.CatalogState
}

// NewStaticCatalogProvider loads a catalog once and returns a static provider.
func NewStaticCatalogProvider(ctx context.Context, cfg ServeConfig, logger *zap.Logger) (*StaticCatalogProvider, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	loader := catalog.NewLoader(logger)
	catalogData, err := loader.Load(ctx, cfg.ConfigPath)
	if err != nil {
		return nil, err
	}
	state, err := domain.NewCatalogState(catalogData, 1, time.Now())
	if err != nil {
		return nil, err
	}
	return &StaticCatalogProvider{state: state}, nil
}

// Snapshot returns the current catalog snapshot.
func (p *StaticCatalogProvider) Snapshot(ctx context.Context) (domain.CatalogState, error) {
	if ctx == nil {
		return p.state, nil
	}
	if err := ctx.Err(); err != nil {
		return domain.CatalogState{}, err
	}
	return p.state, nil
}

// Watch returns a closed update channel for static catalogs.
func (p *StaticCatalogProvider) Watch(ctx context.Context) (<-chan domain.CatalogUpdate, error) {
	ch := make(chan domain.CatalogUpdate)
	close(ch)
	return ch, nil
}

// Reload is a no-op for static catalogs.
func (p *StaticCatalogProvider) Reload(ctx context.Context) error {
	return nil
}
