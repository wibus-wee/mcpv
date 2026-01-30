package catalog

import (
	"context"

	"mcpv/internal/domain"
)

// NewCatalogState loads a catalog state from a provider snapshot.
func NewCatalogState(ctx context.Context, provider domain.CatalogProvider) (*domain.CatalogState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	state, err := provider.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return &state, nil
}
