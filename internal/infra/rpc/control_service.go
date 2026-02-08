package rpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/infra/governance"
	controlv1 "mcpv/pkg/api/control/v1"
)

type ControlService struct {
	controlv1.UnimplementedControlPlaneServiceServer
	control  ControlPlaneAPI
	executor *governance.Executor
	guard    governanceGuard
	logger   *zap.Logger
}

func NewControlService(control ControlPlaneAPI, executor *governance.Executor, logger *zap.Logger) *ControlService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ControlService{
		control:  control,
		executor: executor,
		guard:    newGovernanceGuard(executor),
		logger:   logger.Named("rpc_control"),
	}
}

func (s *ControlService) GetInfo(ctx context.Context, _ *controlv1.GetInfoRequest) (*controlv1.GetInfoResponse, error) {
	info, err := s.control.Info(ctx)
	if err != nil {
		return nil, statusFromError("get info", err)
	}
	return &controlv1.GetInfoResponse{
		Name:    info.Name,
		Version: info.Version,
		Build:   info.Build,
	}, nil
}

func (s *ControlService) RegisterCaller(ctx context.Context, req *controlv1.RegisterCallerRequest) (*controlv1.RegisterCallerResponse, error) {
	client := req.GetCaller()
	if client == "" {
		return nil, status.Error(codes.InvalidArgument, "client is required")
	}
	if req.GetPid() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "pid must be > 0")
	}
	if req.GetServer() != "" && len(req.GetTags()) > 0 {
		return nil, status.Error(codes.InvalidArgument, "server and tags are mutually exclusive")
	}
	registration, err := s.control.RegisterClient(ctx, client, int(req.GetPid()), req.GetTags(), req.GetServer())
	if err != nil {
		return nil, statusFromError("register caller", err)
	}
	return &controlv1.RegisterCallerResponse{
		Profile: registration.Client,
	}, nil
}

func (s *ControlService) UnregisterCaller(ctx context.Context, req *controlv1.UnregisterCallerRequest) (*controlv1.UnregisterCallerResponse, error) {
	client := req.GetCaller()
	if client == "" {
		return nil, status.Error(codes.InvalidArgument, "client is required")
	}
	if err := s.control.UnregisterClient(ctx, client); err != nil {
		return nil, statusFromError("unregister caller", err)
	}
	return &controlv1.UnregisterCallerResponse{}, nil
}

// IsSubAgentEnabled returns whether the SubAgent is enabled for the caller's profile.
func (s *ControlService) IsSubAgentEnabled(_ context.Context, req *controlv1.IsSubAgentEnabledRequest) (*controlv1.IsSubAgentEnabledResponse, error) {
	return &controlv1.IsSubAgentEnabledResponse{
		Enabled: s.control.IsSubAgentEnabledForClient(req.GetCaller()),
	}, nil
}
