package rpc

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mcpcodec"
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
	client := req.GetCaller()
	if client == "" {
		return nil, status.Error(codes.InvalidArgument, "client is required")
	}
	if req.GetPid() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "pid must be > 0")
	}
	registration, err := s.control.RegisterClient(ctx, client, int(req.GetPid()), nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register caller: %v", err)
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
		if errors.Is(err, domain.ErrClientNotRegistered) {
			return nil, status.Error(codes.FailedPrecondition, "client not registered")
		}
		return nil, status.Errorf(codes.Internal, "unregister caller: %v", err)
	}
	return &controlv1.UnregisterCallerResponse{}, nil
}

func (s *ControlService) ListTools(ctx context.Context, req *controlv1.ListToolsRequest) (*controlv1.ListToolsResponse, error) {
	client := req.GetCaller()
	snapshot, err := s.control.ListTools(ctx, client)
	if err != nil {
		return nil, mapClientError("list tools", err)
	}
	protoSnapshot, err := toProtoSnapshot(snapshot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tools: %v", err)
	}
	return &controlv1.ListToolsResponse{
		Snapshot: protoSnapshot,
	}, nil
}

func (s *ControlService) WatchTools(req *controlv1.WatchToolsRequest, stream controlv1.ControlPlaneService_WatchToolsServer) error {
	ctx := stream.Context()
	client := req.GetCaller()
	current, err := s.control.ListTools(ctx, client)
	if err != nil {
		return mapClientError("list tools", err)
	}
	lastETag := req.GetLastEtag()
	if lastETag == "" || lastETag != current.ETag {
		protoSnapshot, err := toProtoSnapshot(current)
		if err != nil {
			return status.Errorf(codes.Internal, "watch tools: %v", err)
		}
		if err := stream.Send(protoSnapshot); err != nil {
			return err
		}
		lastETag = current.ETag
	}

	updates, err := s.control.WatchTools(ctx, client)
	if err != nil {
		return mapClientError("watch tools", err)
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
			protoSnapshot, err := toProtoSnapshot(snapshot)
			if err != nil {
				return status.Errorf(codes.Internal, "watch tools: %v", err)
			}
			if err := stream.Send(protoSnapshot); err != nil {
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
	client := req.GetCaller()
	result, err := s.control.CallTool(ctx, client, req.GetName(), req.GetArgumentsJson(), req.GetRoutingKey())
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
	client := req.GetCaller()
	page, err := s.control.ListResources(ctx, client, req.GetCursor())
	if err != nil {
		return nil, mapListError("list resources", err)
	}
	resourcesSnapshot, err := toProtoResourcesSnapshot(page.Snapshot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list resources: %v", err)
	}
	return &controlv1.ListResourcesResponse{
		Snapshot:   resourcesSnapshot,
		NextCursor: page.NextCursor,
	}, nil
}

func (s *ControlService) WatchResources(req *controlv1.WatchResourcesRequest, stream controlv1.ControlPlaneService_WatchResourcesServer) error {
	ctx := stream.Context()
	lastETag := req.GetLastEtag()

	client := req.GetCaller()
	updates, err := s.control.WatchResources(ctx, client)
	if err != nil {
		return mapClientError("watch resources", err)
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
			protoSnapshot, err := toProtoResourcesSnapshot(snapshot)
			if err != nil {
				return status.Errorf(codes.Internal, "watch resources: %v", err)
			}
			if err := stream.Send(protoSnapshot); err != nil {
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
	client := req.GetCaller()
	result, err := s.control.ReadResource(ctx, client, req.GetUri())
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
	client := req.GetCaller()
	page, err := s.control.ListPrompts(ctx, client, req.GetCursor())
	if err != nil {
		return nil, mapListError("list prompts", err)
	}
	promptsSnapshot, err := toProtoPromptsSnapshot(page.Snapshot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list prompts: %v", err)
	}
	return &controlv1.ListPromptsResponse{
		Snapshot:   promptsSnapshot,
		NextCursor: page.NextCursor,
	}, nil
}

func (s *ControlService) WatchPrompts(req *controlv1.WatchPromptsRequest, stream controlv1.ControlPlaneService_WatchPromptsServer) error {
	ctx := stream.Context()
	lastETag := req.GetLastEtag()

	client := req.GetCaller()
	updates, err := s.control.WatchPrompts(ctx, client)
	if err != nil {
		return mapClientError("watch prompts", err)
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
			protoSnapshot, err := toProtoPromptsSnapshot(snapshot)
			if err != nil {
				return status.Errorf(codes.Internal, "watch prompts: %v", err)
			}
			if err := stream.Send(protoSnapshot); err != nil {
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
	client := req.GetCaller()
	result, err := s.control.GetPrompt(ctx, client, req.GetName(), req.GetArgumentsJson())
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
	case errors.Is(err, domain.ErrClientNotRegistered):
		return status.Error(codes.FailedPrecondition, "client not registered")
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
	case errors.Is(err, domain.ErrClientNotRegistered):
		return status.Error(codes.FailedPrecondition, "client not registered")
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
	case errors.Is(err, domain.ErrClientNotRegistered):
		return status.Error(codes.FailedPrecondition, "client not registered")
	default:
		return status.Errorf(codes.Unavailable, "get prompt: %v", fmt.Sprintf("%T: %v", err, err))
	}
}

func mapListError(op string, err error) error {
	if errors.Is(err, domain.ErrClientNotRegistered) {
		return status.Errorf(codes.FailedPrecondition, "%s: client not registered", op)
	}
	if errors.Is(err, domain.ErrInvalidCursor) {
		return status.Errorf(codes.InvalidArgument, "%s: invalid cursor", op)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

func (s *ControlService) StreamLogs(req *controlv1.StreamLogsRequest, stream controlv1.ControlPlaneService_StreamLogsServer) error {
	ctx := stream.Context()
	minLevel := fromProtoLogLevel(req.GetMinLevel())
	client := req.GetCaller()
	entries, err := s.control.StreamLogs(ctx, client, minLevel)
	if err != nil {
		return mapClientError("stream logs", err)
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

func mapClientError(op string, err error) error {
	if errors.Is(err, domain.ErrClientNotRegistered) {
		return status.Errorf(codes.FailedPrecondition, "%s: client not registered", op)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

func (s *ControlService) WatchRuntimeStatus(req *controlv1.WatchRuntimeStatusRequest, stream controlv1.ControlPlaneService_WatchRuntimeStatusServer) error {
	ctx := stream.Context()
	lastETag := req.GetLastEtag()

	client := req.GetCaller()
	updates, err := s.control.WatchRuntimeStatus(ctx, client)
	if err != nil {
		return mapClientError("watch runtime status", err)
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
			if err := stream.Send(toProtoRuntimeStatusSnapshot(snapshot)); err != nil {
				return err
			}
			lastETag = snapshot.ETag
		}
	}
}

func (s *ControlService) WatchServerInitStatus(req *controlv1.WatchServerInitStatusRequest, stream controlv1.ControlPlaneService_WatchServerInitStatusServer) error {
	ctx := stream.Context()

	client := req.GetCaller()
	updates, err := s.control.WatchServerInitStatus(ctx, client)
	if err != nil {
		return mapClientError("watch server init status", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case snapshot, ok := <-updates:
			if !ok {
				return nil
			}
			if err := stream.Send(toProtoServerInitStatusSnapshot(snapshot)); err != nil {
				return err
			}
		}
	}
}

// AutomaticMCP handles the automatic_mcp tool call for SubAgent.
func (s *ControlService) AutomaticMCP(ctx context.Context, req *controlv1.AutomaticMCPRequest) (*controlv1.AutomaticMCPResponse, error) {
	params := domain.AutomaticMCPParams{
		Query:        req.GetQuery(),
		SessionID:    req.GetSessionId(),
		ForceRefresh: req.GetForceRefresh(),
	}

	result, err := s.control.AutomaticMCP(ctx, req.GetCaller(), params)
	if err != nil {
		return nil, mapClientError("automatic_mcp", err)
	}

	resp, err := toProtoAutomaticMCPResponse(result)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "automatic_mcp: %v", err)
	}
	return resp, nil
}

// AutomaticEval handles the automatic_eval tool call for SubAgent.
func (s *ControlService) AutomaticEval(ctx context.Context, req *controlv1.AutomaticEvalRequest) (*controlv1.AutomaticEvalResponse, error) {
	if req.GetToolName() == "" {
		return nil, status.Error(codes.InvalidArgument, "tool_name is required")
	}

	params := domain.AutomaticEvalParams{
		ToolName:   req.GetToolName(),
		Arguments:  req.GetArgumentsJson(),
		RoutingKey: req.GetRoutingKey(),
	}

	client := req.GetCaller()
	result, err := s.control.AutomaticEval(ctx, client, params)
	if err != nil {
		return nil, mapCallToolError(req.GetToolName(), err)
	}

	return &controlv1.AutomaticEvalResponse{
		ResultJson: result,
	}, nil
}

// IsSubAgentEnabled returns whether the SubAgent is enabled for the caller's profile.
func (s *ControlService) IsSubAgentEnabled(ctx context.Context, req *controlv1.IsSubAgentEnabledRequest) (*controlv1.IsSubAgentEnabledResponse, error) {
	return &controlv1.IsSubAgentEnabledResponse{
		Enabled: s.control.IsSubAgentEnabledForClient(req.GetCaller()),
	}, nil
}

// toProtoAutomaticMCPResponse converts domain.AutomaticMCPResult to proto response.
func toProtoAutomaticMCPResponse(snapshot domain.AutomaticMCPResult) (*controlv1.AutomaticMCPResponse, error) {
	tools := make([][]byte, 0, len(snapshot.Tools))
	for _, tool := range snapshot.Tools {
		raw, err := mcpcodec.MarshalToolDefinition(tool)
		if err != nil {
			return nil, fmt.Errorf("marshal tool %q: %w", tool.Name, err)
		}
		tools = append(tools, raw)
	}
	return &controlv1.AutomaticMCPResponse{
		Etag:           snapshot.ETag,
		ToolsJson:      tools,
		TotalAvailable: int32(snapshot.TotalAvailable),
		Filtered:       int32(snapshot.Filtered),
	}, nil
}
