package services

import (
	"context"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/ui"
)

// CoreService exposes core lifecycle APIs.
type CoreService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewCoreService(deps *ServiceDeps) *CoreService {
	return &CoreService{
		deps:   deps,
		logger: deps.loggerNamed("core-service"),
	}
}

// GetCoreState returns current core state.
func (s *CoreService) GetCoreState() CoreStateResponse {
	manager := s.deps.manager()
	if manager == nil {
		return CoreStateResponse{State: "unknown"}
	}

	state, uptime, err := manager.GetState()
	resp := CoreStateResponse{
		State:  string(state),
		Uptime: uptime,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

// StartCore starts the core.
func (s *CoreService) StartCore(ctx context.Context) error {
	manager := s.deps.manager()
	if manager == nil {
		return ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	s.logger.Info("core start requested")
	return manager.Start(ctx)
}

// StartCoreWithOptions starts the core with explicit options.
func (s *CoreService) StartCoreWithOptions(ctx context.Context, opts StartCoreOptions) error {
	manager := s.deps.manager()
	if manager == nil {
		return ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	s.logger.Info("core start requested with options", zap.Any("options", opts))
	return manager.StartWithOptions(ctx, opts)
}

// StopCore stops the core.
func (s *CoreService) StopCore() error {
	manager := s.deps.manager()
	if manager == nil {
		return ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	return manager.Stop()
}

// RestartCore restarts the core.
func (s *CoreService) RestartCore(ctx context.Context) error {
	manager := s.deps.manager()
	if manager == nil {
		return ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	return manager.Restart(ctx)
}

// GetInfo returns control plane info.
func (s *CoreService) GetInfo(ctx context.Context) (InfoResponse, error) {
	cp, err := s.deps.getControlPlane()
	if err != nil {
		return InfoResponse{}, err
	}

	info, err := cp.Info(ctx)
	if err != nil {
		return InfoResponse{}, ui.MapDomainError(err)
	}
	return InfoResponse{
		Name:    info.Name,
		Version: info.Version,
		Build:   info.Build,
	}, nil
}

// GetBootstrapProgress returns bootstrap progress.
func (s *CoreService) GetBootstrapProgress(ctx context.Context) (BootstrapProgressResponse, error) {
	manager := s.deps.manager()
	if manager == nil {
		return BootstrapProgressResponse{State: string(domain.BootstrapPending)}, nil
	}

	state, _, _ := manager.GetState()
	if state != ui.CoreStateRunning {
		return BootstrapProgressResponse{State: string(domain.BootstrapPending)}, nil
	}

	cp, err := s.deps.getControlPlane()
	if err != nil {
		if uiErr, ok := err.(*Error); ok && uiErr.Code == ui.ErrCodeCoreNotRunning {
			return BootstrapProgressResponse{State: string(domain.BootstrapPending)}, nil
		}
		return BootstrapProgressResponse{}, err
	}

	progress, err := cp.GetBootstrapProgress(ctx)
	if err != nil {
		return BootstrapProgressResponse{}, ui.MapDomainError(err)
	}

	return BootstrapProgressResponse{
		State:     string(progress.State),
		Total:     progress.Total,
		Completed: progress.Completed,
		Failed:    progress.Failed,
		Current:   progress.Current,
		Errors:    progress.Errors,
	}, nil
}
