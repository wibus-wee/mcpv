package ui

import (
	"context"

	"go.uber.org/zap"
)

// RuntimeService exposes runtime status APIs.
type RuntimeService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewRuntimeService(deps *ServiceDeps) *RuntimeService {
	return &RuntimeService{
		deps:   deps,
		logger: deps.loggerNamed("runtime-service"),
	}
}

// GetRuntimeStatus returns the runtime status of all server pools.
func (s *RuntimeService) GetRuntimeStatus(ctx context.Context) ([]ServerRuntimeStatus, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	pools, err := cp.GetPoolStatus(ctx)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapRuntimeStatuses(pools), nil
}

// GetServerInitStatus returns per-server initialization status.
func (s *RuntimeService) GetServerInitStatus(ctx context.Context) ([]ServerInitStatus, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	statuses, err := cp.GetServerInitStatus(ctx)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapServerInitStatuses(statuses), nil
}

// RetryServerInit triggers a manual retry for a suspended server init.
func (s *RuntimeService) RetryServerInit(ctx context.Context, req RetryServerInitRequest) error {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return err
	}
	if err := cp.RetryServerInit(ctx, req.SpecKey); err != nil {
		return MapDomainError(err)
	}
	return nil
}

// GetActiveCallers returns active caller registrations.
func (s *RuntimeService) GetActiveCallers(ctx context.Context) ([]ActiveCaller, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	callers, err := cp.ListActiveCallers(ctx)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapActiveCallers(callers), nil
}
