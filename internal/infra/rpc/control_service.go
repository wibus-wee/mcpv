package rpc

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpd/internal/domain"
	controlv1 "mcpd/pkg/api/control/v1"
)

type ControlService struct {
	controlv1.UnimplementedControlPlaneServiceServer
	control domain.ControlPlane
	logger  *zap.Logger
}

func NewControlService(control domain.ControlPlane, logger *zap.Logger) *ControlService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ControlService{
		control: control,
		logger:  logger.Named("rpc_control"),
	}
}

func (s *ControlService) GetInfo(ctx context.Context, req *controlv1.GetInfoRequest) (*controlv1.GetInfoResponse, error) {
	info, err := s.control.Info(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get info: %v", err)
	}
	return &controlv1.GetInfoResponse{
		Name:    info.Name,
		Version: info.Version,
		Build:   info.Build,
	}, nil
}

func (s *ControlService) ListTools(ctx context.Context, req *controlv1.ListToolsRequest) (*controlv1.ListToolsResponse, error) {
	snapshot, err := s.control.ListTools(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tools: %v", err)
	}
	return &controlv1.ListToolsResponse{
		Snapshot: toProtoSnapshot(snapshot),
	}, nil
}

func (s *ControlService) WatchTools(req *controlv1.WatchToolsRequest, stream controlv1.ControlPlaneService_WatchToolsServer) error {
	ctx := stream.Context()
	current, err := s.control.ListTools(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "list tools: %v", err)
	}
	lastETag := req.GetLastEtag()
	if lastETag == "" || lastETag != current.ETag {
		if err := stream.Send(toProtoSnapshot(current)); err != nil {
			return err
		}
		lastETag = current.ETag
	}

	updates, err := s.control.WatchTools(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "watch tools: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case snapshot, ok := <-updates:
			if !ok {
				return nil
			}
			if lastETag == snapshot.ETag {
				continue
			}
			if err := stream.Send(toProtoSnapshot(snapshot)); err != nil {
				return err
			}
			lastETag = snapshot.ETag
		}
	}
}

func (s *ControlService) CallTool(ctx context.Context, req *controlv1.CallToolRequest) (*controlv1.CallToolResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	result, err := s.control.CallTool(ctx, req.GetName(), req.GetArgumentsJson(), req.GetRoutingKey())
	if err != nil {
		if errors.Is(err, domain.ErrToolNotFound) {
			return nil, status.Errorf(codes.NotFound, "tool not found: %s", req.GetName())
		}
		return nil, status.Errorf(codes.Internal, "call tool: %v", err)
	}
	if len(result) == 0 {
		return nil, status.Error(codes.Internal, "call tool: empty result")
	}
	return &controlv1.CallToolResponse{
		ResultJson: result,
	}, nil
}

func (s *ControlService) StreamLogs(req *controlv1.StreamLogsRequest, stream controlv1.ControlPlaneService_StreamLogsServer) error {
	ctx := stream.Context()
	minLevel := fromProtoLogLevel(req.GetMinLevel())
	entries, err := s.control.StreamLogs(ctx, minLevel)
	if err != nil {
		return status.Errorf(codes.Internal, "stream logs: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case entry, ok := <-entries:
			if !ok {
				return nil
			}
			if err := stream.Send(toProtoLogEntry(entry)); err != nil {
				return err
			}
		}
	}
}
