package observability

import (
	"context"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	"mcpv/internal/domain"
)

type State interface {
	Scheduler() domain.Scheduler
	InitManager() *bootstrap.ServerInitializationManager
	BootstrapManager() *bootstrap.Manager
	Context() context.Context
	Logger() *zap.Logger
}
