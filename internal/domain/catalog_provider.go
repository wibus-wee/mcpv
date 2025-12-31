package domain

import "context"

type CatalogUpdateSource string

const (
	CatalogUpdateSourceBootstrap CatalogUpdateSource = "bootstrap"
	CatalogUpdateSourceWatch     CatalogUpdateSource = "watch"
	CatalogUpdateSourceManual    CatalogUpdateSource = "manual"
)

type CatalogUpdate struct {
	Snapshot CatalogState
	Diff     CatalogDiff
	Source   CatalogUpdateSource
}

type CatalogProvider interface {
	Snapshot(ctx context.Context) (CatalogState, error)
	Watch(ctx context.Context) (<-chan CatalogUpdate, error)
	Reload(ctx context.Context) error
}
