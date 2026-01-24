package domain

import "time"

// CatalogState captures the current catalog snapshot and metadata.
type CatalogState struct {
	Catalog  Catalog
	Summary  CatalogSummary
	Revision uint64
	LoadedAt time.Time
}

// NewCatalogState builds a catalog state from a catalog.
func NewCatalogState(catalog Catalog, revision uint64, loadedAt time.Time) (CatalogState, error) {
	if loadedAt.IsZero() {
		loadedAt = time.Now()
	}
	summary, err := BuildCatalogSummary(catalog)
	if err != nil {
		return CatalogState{}, err
	}
	return CatalogState{
		Catalog:  catalog,
		Summary:  summary,
		Revision: revision,
		LoadedAt: loadedAt,
	}, nil
}
