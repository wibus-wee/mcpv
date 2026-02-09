package services

import "go.uber.org/zap"

// DebugService exposes debugging utilities.
type DebugService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewDebugService(deps *ServiceDeps) *DebugService {
	return &DebugService{
		deps:   deps,
		logger: deps.loggerNamed("debug-service"),
	}
}
