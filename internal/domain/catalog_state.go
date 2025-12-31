package domain

import "time"

type CatalogState struct {
	Store    ProfileStore
	Summary  CatalogSummary
	Revision uint64
	LoadedAt time.Time
}

func NewCatalogState(store ProfileStore, revision uint64, loadedAt time.Time) (CatalogState, error) {
	if loadedAt.IsZero() {
		loadedAt = time.Now()
	}
	summary, err := BuildCatalogSummary(store)
	if err != nil {
		return CatalogState{}, err
	}
	return CatalogState{
		Store:    store,
		Summary:  summary,
		Revision: revision,
		LoadedAt: loadedAt,
	}, nil
}
