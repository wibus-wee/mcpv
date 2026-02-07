package registry

import (
	"context"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/domain"
)

type State interface {
	Catalog() domain.Catalog
	ServerSpecKeys() map[string]string
	SpecRegistry() map[string]domain.ServerSpec
	Runtime() domain.RuntimeConfig
	Logger() *zap.Logger
	Context() context.Context
	Scheduler() domain.Scheduler
	InitManager() *bootstrap.ServerInitializationManager
}
