package app

import "mcpd/internal/domain"

type CatalogSnapshot struct {
	store   domain.ProfileStore
	summary profileSummary
}

func NewCatalogSnapshot(accessor domain.CatalogAccessor) (*CatalogSnapshot, error) {
	store, err := accessor.GetProfileStore()
	if err != nil {
		return nil, err
	}
	summary, err := buildProfileSummary(store)
	if err != nil {
		return nil, err
	}
	return &CatalogSnapshot{
		store:   store,
		summary: summary,
	}, nil
}

func (s *CatalogSnapshot) Store() domain.ProfileStore {
	return s.store
}

func (s *CatalogSnapshot) Summary() profileSummary {
	return s.summary
}
