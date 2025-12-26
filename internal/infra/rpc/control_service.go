package rpc

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpd/internal/domain"
	"mcpd/internal/infra/scheduler"
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
		return nil, mapCallToolError(req.GetName(), err)
	}
	if len(result) == 0 {
		return nil, status.Error(codes.Internal, "call tool: empty result")
	}
	return &controlv1.CallToolResponse{
		ResultJson: result,
	}, nil
}

func mapCallToolError(name string, err error) error {
	switch {
	case errors.Is(err, domain.ErrToolNotFound):
		return status.Errorf(codes.NotFound, "tool not found: %s", name)
	case errors.Is(err, domain.ErrInvalidRequest), errors.Is(err, domain.ErrMethodNotAllowed):
		return status.Errorf(codes.InvalidArgument, "call tool: %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		return status.Errorf(codes.DeadlineExceeded, "call tool deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Errorf(codes.Canceled, "call tool canceled")
	case errors.Is(err, scheduler.ErrUnknownServerType):
		return status.Errorf(codes.InvalidArgument, "call tool: %v", err)
	case errors.Is(err, scheduler.ErrNoCapacity), errors.Is(err, scheduler.ErrStickyBusy):
		return status.Errorf(codes.Unavailable, "call tool unavailable: %v", err)
	default:
		return status.Errorf(codes.Unavailable, "call tool: %v", fmt.Sprintf("%T: %v", err, err))
	}
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
