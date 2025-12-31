package domain

// CatalogAccessor abstracts access to configuration snapshots.
type CatalogAccessor interface {
	GetProfileStore() (ProfileStore, error)
}
