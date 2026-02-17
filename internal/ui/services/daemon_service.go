package services

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/daemon"
	"mcpv/internal/ui"
)

// DaemonService exposes system daemon management APIs.
type DaemonService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewDaemonService(deps *ServiceDeps) *DaemonService {
	return &DaemonService{
		deps:   deps,
		logger: deps.loggerNamed("daemon-service"),
	}
}

func (s *DaemonService) Status(ctx context.Context) (DaemonStatus, error) {
	manager, err := s.managerFromRequest(DaemonInstallRequest{}, false)
	if err != nil {
		return DaemonStatus{}, err
	}
	status, err := manager.Status(ctx)
	if err != nil {
		return DaemonStatus{}, err
	}
	return toDaemonStatus(status), nil
}

func (s *DaemonService) Install(ctx context.Context, req DaemonInstallRequest) (DaemonStatus, error) {
	manager, err := s.managerFromRequest(req, true)
	if err != nil {
		return DaemonStatus{}, err
	}
	status, err := manager.Install(ctx)
	if err != nil {
		return DaemonStatus{}, err
	}
	return toDaemonStatus(status), nil
}

func (s *DaemonService) Uninstall(ctx context.Context) (DaemonStatus, error) {
	manager, err := s.managerFromRequest(DaemonInstallRequest{}, false)
	if err != nil {
		return DaemonStatus{}, err
	}
	status, err := manager.Uninstall(ctx)
	if err != nil {
		return DaemonStatus{}, err
	}
	return toDaemonStatus(status), nil
}

func (s *DaemonService) Start(ctx context.Context, req DaemonInstallRequest) (DaemonStatus, error) {
	manager, err := s.managerFromRequest(req, true)
	if err != nil {
		return DaemonStatus{}, err
	}
	status, err := manager.Start(ctx)
	if err != nil {
		return DaemonStatus{}, err
	}
	return toDaemonStatus(status), nil
}

func (s *DaemonService) Stop(ctx context.Context) (DaemonStatus, error) {
	manager, err := s.managerFromRequest(DaemonInstallRequest{}, false)
	if err != nil {
		return DaemonStatus{}, err
	}
	status, err := manager.Stop(ctx)
	if err != nil {
		return DaemonStatus{}, err
	}
	return toDaemonStatus(status), nil
}

func (s *DaemonService) EnsureRunning(ctx context.Context, req DaemonEnsureRequest) (DaemonStatus, error) {
	manager, err := s.managerFromRequest(DaemonInstallRequest{
		ConfigPath: req.ConfigPath,
		RPCAddress: req.RPCAddress,
		LogPath:    req.LogPath,
		BinaryPath: req.BinaryPath,
	}, req.AllowStart)
	if err != nil {
		return DaemonStatus{}, err
	}
	status, err := manager.EnsureRunning(ctx, req.AllowStart)
	if err != nil {
		return DaemonStatus{}, err
	}
	return toDaemonStatus(status), nil
}

func (s *DaemonService) managerFromRequest(req DaemonInstallRequest, requireConfig bool) (*daemon.Manager, error) {
	configPath := strings.TrimSpace(req.ConfigPath)
	if requireConfig {
		if configPath == "" {
			configPath = ui.ResolveDefaultConfigPath()
		}
		if err := ui.EnsureConfigFile(configPath); err != nil {
			return nil, err
		}
	}
	rpcAddress := strings.TrimSpace(req.RPCAddress)
	if rpcAddress == "" {
		rpcAddress = domain.DefaultRPCListenAddress
	}
	logPath := strings.TrimSpace(req.LogPath)
	binaryPath := strings.TrimSpace(req.BinaryPath)
	if binaryPath == "" {
		binaryPath = ui.ResolveMcpvPath()
	}
	return daemon.NewManager(daemon.Options{
		BinaryPath: binaryPath,
		ConfigPath: configPath,
		RPCAddress: rpcAddress,
		LogPath:    logPath,
	})
}

func toDaemonStatus(status daemon.Status) DaemonStatus {
	return DaemonStatus{
		Installed:   status.Installed,
		Running:     status.Running,
		ServiceName: status.ServiceName,
		ConfigPath:  status.ConfigPath,
		RPCAddress:  status.RPCAddress,
		LogPath:     status.LogPath,
	}
}
