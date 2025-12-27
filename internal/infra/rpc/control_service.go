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

func (s *ControlService) RegisterCaller(ctx context.Context, req *controlv1.RegisterCallerRequest) (*controlv1.RegisterCallerResponse, error) {
	if req.GetCaller() == "" {
		return nil, status.Error(codes.InvalidArgument, "caller is required")
	}
	if req.GetPid() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "pid must be > 0")
	}
	profile, err := s.control.RegisterCaller(ctx, req.GetCaller(), int(req.GetPid()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register caller: %v", err)
	}
	return &controlv1.RegisterCallerResponse{
		Profile: profile,
	}, nil
}

func (s *ControlService) UnregisterCaller(ctx context.Context, req *controlv1.UnregisterCallerRequest) (*controlv1.UnregisterCallerResponse, error) {
	if req.GetCaller() == "" {
		return nil, status.Error(codes.InvalidArgument, "caller is required")
	}
	if err := s.control.UnregisterCaller(ctx, req.GetCaller()); err != nil {
		if errors.Is(err, domain.ErrCallerNotRegistered) {
			return nil, status.Error(codes.FailedPrecondition, "caller not registered")
		}
		return nil, status.Errorf(codes.Internal, "unregister caller: %v", err)
	}
	return &controlv1.UnregisterCallerResponse{}, nil
}

func (s *ControlService) ListTools(ctx context.Context, req *controlv1.ListToolsRequest) (*controlv1.ListToolsResponse, error) {
	snapshot, err := s.control.ListTools(ctx, req.GetCaller())
	if err != nil {
		return nil, mapCallerError("list tools", err)
	}
	return &controlv1.ListToolsResponse{
		Snapshot: toProtoSnapshot(snapshot),
	}, nil
}

func (s *ControlService) WatchTools(req *controlv1.WatchToolsRequest, stream controlv1.ControlPlaneService_WatchToolsServer) error {
	ctx := stream.Context()
	current, err := s.control.ListTools(ctx, req.GetCaller())
	if err != nil {
		return mapCallerError("list tools", err)
	}
	lastETag := req.GetLastEtag()
	if lastETag == "" || lastETag != current.ETag {
		if err := stream.Send(toProtoSnapshot(current)); err != nil {
			return err
		}
		lastETag = current.ETag
	}

	updates, err := s.control.WatchTools(ctx, req.GetCaller())
	if err != nil {
		return mapCallerError("watch tools", err)
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
	result, err := s.control.CallTool(ctx, req.GetCaller(), req.GetName(), req.GetArgumentsJson(), req.GetRoutingKey())
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

func (s *ControlService) ListResources(ctx context.Context, req *controlv1.ListResourcesRequest) (*controlv1.ListResourcesResponse, error) {
	page, err := s.control.ListResources(ctx, req.GetCaller(), req.GetCursor())
	if err != nil {
		return nil, mapListError("list resources", err)
	}
	return &controlv1.ListResourcesResponse{
		Snapshot:   toProtoResourcesSnapshot(page.Snapshot),
		NextCursor: page.NextCursor,
	}, nil
}

func (s *ControlService) WatchResources(req *controlv1.WatchResourcesRequest, stream controlv1.ControlPlaneService_WatchResourcesServer) error {
	ctx := stream.Context()
	lastETag := req.GetLastEtag()

	updates, err := s.control.WatchResources(ctx, req.GetCaller())
	if err != nil {
		return mapCallerError("watch resources", err)
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
			if err := stream.Send(toProtoResourcesSnapshot(snapshot)); err != nil {
				return err
			}
			lastETag = snapshot.ETag
		}
	}
}

func (s *ControlService) ReadResource(ctx context.Context, req *controlv1.ReadResourceRequest) (*controlv1.ReadResourceResponse, error) {
	if req.GetUri() == "" {
		return nil, status.Error(codes.InvalidArgument, "uri is required")
	}
	result, err := s.control.ReadResource(ctx, req.GetCaller(), req.GetUri())
	if err != nil {
		return nil, mapReadResourceError(req.GetUri(), err)
	}
	if len(result) == 0 {
		return nil, status.Error(codes.Internal, "read resource: empty result")
	}
	return &controlv1.ReadResourceResponse{
		ResultJson: result,
	}, nil
}

func (s *ControlService) ListPrompts(ctx context.Context, req *controlv1.ListPromptsRequest) (*controlv1.ListPromptsResponse, error) {
	page, err := s.control.ListPrompts(ctx, req.GetCaller(), req.GetCursor())
	if err != nil {
		return nil, mapListError("list prompts", err)
	}
	return &controlv1.ListPromptsResponse{
		Snapshot:   toProtoPromptsSnapshot(page.Snapshot),
		NextCursor: page.NextCursor,
	}, nil
}

func (s *ControlService) WatchPrompts(req *controlv1.WatchPromptsRequest, stream controlv1.ControlPlaneService_WatchPromptsServer) error {
	ctx := stream.Context()
	lastETag := req.GetLastEtag()

	updates, err := s.control.WatchPrompts(ctx, req.GetCaller())
	if err != nil {
		return mapCallerError("watch prompts", err)
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
			if err := stream.Send(toProtoPromptsSnapshot(snapshot)); err != nil {
				return err
			}
			lastETag = snapshot.ETag
		}
	}
}

func (s *ControlService) GetPrompt(ctx context.Context, req *controlv1.GetPromptRequest) (*controlv1.GetPromptResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	result, err := s.control.GetPrompt(ctx, req.GetCaller(), req.GetName(), req.GetArgumentsJson())
	if err != nil {
		return nil, mapGetPromptError(req.GetName(), err)
	}
	if len(result) == 0 {
		return nil, status.Error(codes.Internal, "get prompt: empty result")
	}
	return &controlv1.GetPromptResponse{
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
	case errors.Is(err, scheduler.ErrUnknownSpecKey):
		return status.Errorf(codes.InvalidArgument, "call tool: %v", err)
	case errors.Is(err, scheduler.ErrNoCapacity), errors.Is(err, scheduler.ErrStickyBusy):
		return status.Errorf(codes.Unavailable, "call tool unavailable: %v", err)
	case errors.Is(err, domain.ErrCallerNotRegistered):
		return status.Error(codes.FailedPrecondition, "caller not registered")
	default:
		return status.Errorf(codes.Unavailable, "call tool: %v", fmt.Sprintf("%T: %v", err, err))
	}
}

func mapReadResourceError(uri string, err error) error {
	switch {
	case errors.Is(err, domain.ErrResourceNotFound):
		return status.Errorf(codes.NotFound, "resource not found: %s", uri)
	case errors.Is(err, domain.ErrInvalidRequest), errors.Is(err, domain.ErrMethodNotAllowed):
		return status.Errorf(codes.InvalidArgument, "read resource: %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		return status.Errorf(codes.DeadlineExceeded, "read resource deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Errorf(codes.Canceled, "read resource canceled")
	case errors.Is(err, scheduler.ErrUnknownSpecKey):
		return status.Errorf(codes.InvalidArgument, "read resource: %v", err)
	case errors.Is(err, scheduler.ErrNoCapacity), errors.Is(err, scheduler.ErrStickyBusy):
		return status.Errorf(codes.Unavailable, "read resource unavailable: %v", err)
	case errors.Is(err, domain.ErrCallerNotRegistered):
		return status.Error(codes.FailedPrecondition, "caller not registered")
	default:
		return status.Errorf(codes.Unavailable, "read resource: %v", fmt.Sprintf("%T: %v", err, err))
	}
}

func mapGetPromptError(name string, err error) error {
	switch {
	case errors.Is(err, domain.ErrPromptNotFound):
		return status.Errorf(codes.NotFound, "prompt not found: %s", name)
	case errors.Is(err, domain.ErrInvalidRequest), errors.Is(err, domain.ErrMethodNotAllowed):
		return status.Errorf(codes.InvalidArgument, "get prompt: %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		return status.Errorf(codes.DeadlineExceeded, "get prompt deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Errorf(codes.Canceled, "get prompt canceled")
	case errors.Is(err, scheduler.ErrUnknownSpecKey):
		return status.Errorf(codes.InvalidArgument, "get prompt: %v", err)
	case errors.Is(err, scheduler.ErrNoCapacity), errors.Is(err, scheduler.ErrStickyBusy):
		return status.Errorf(codes.Unavailable, "get prompt unavailable: %v", err)
	case errors.Is(err, domain.ErrCallerNotRegistered):
		return status.Error(codes.FailedPrecondition, "caller not registered")
	default:
		return status.Errorf(codes.Unavailable, "get prompt: %v", fmt.Sprintf("%T: %v", err, err))
	}
}

func mapListError(op string, err error) error {
	if errors.Is(err, domain.ErrCallerNotRegistered) {
		return status.Errorf(codes.FailedPrecondition, "%s: caller not registered", op)
	}
	if errors.Is(err, domain.ErrInvalidCursor) {
		return status.Errorf(codes.InvalidArgument, "%s: invalid cursor", op)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

func (s *ControlService) StreamLogs(req *controlv1.StreamLogsRequest, stream controlv1.ControlPlaneService_StreamLogsServer) error {
	ctx := stream.Context()
	minLevel := fromProtoLogLevel(req.GetMinLevel())
	entries, err := s.control.StreamLogs(ctx, req.GetCaller(), minLevel)
	if err != nil {
		return mapCallerError("stream logs", err)
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

func mapCallerError(op string, err error) error {
	if errors.Is(err, domain.ErrCallerNotRegistered) {
		return status.Errorf(codes.FailedPrecondition, "%s: caller not registered", op)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}
