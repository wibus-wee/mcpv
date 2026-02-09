package services

import (
	"context"

	"go.uber.org/zap"

	"mcpv/internal/ui"
	"mcpv/internal/ui/mapping"
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
		return nil, ui.MapDomainError(err)
	}

	return mapping.MapRuntimeStatuses(pools), nil
}

// GetServerInitStatus returns per-server initialization status.
func (s *RuntimeService) GetServerInitStatus(ctx context.Context) ([]ServerInitStatus, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	statuses, err := cp.GetServerInitStatus(ctx)
	if err != nil {
		return nil, ui.MapDomainError(err)
	}

	return mapping.MapServerInitStatuses(statuses), nil
}

// RetryServerInit triggers a manual retry for a suspended server init.
func (s *RuntimeService) RetryServerInit(ctx context.Context, req RetryServerInitRequest) error {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return err
	}
	if err := cp.RetryServerInit(ctx, req.SpecKey); err != nil {
		return ui.MapDomainError(err)
	}
	return nil
}

// GetActiveClients returns active client registrations.
func (s *RuntimeService) GetActiveClients(ctx context.Context) ([]ActiveClient, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return nil, err
	}

	clients, err := cp.ListActiveClients(ctx)
	if err != nil {
		return nil, ui.MapDomainError(err)
	}

	return mapping.MapActiveClients(clients), nil
}
