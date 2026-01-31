package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"mcpv/internal/domain"
	"mcpv/internal/infra/governance"
	"mcpv/internal/infra/mcpcodec"
	"mcpv/internal/infra/scheduler"
	controlv1 "mcpv/pkg/api/control/v1"
)

type ControlService struct {
	controlv1.UnimplementedControlPlaneServiceServer
	control  ControlPlaneAPI
	executor *governance.Executor
	logger   *zap.Logger
}

func NewControlService(control ControlPlaneAPI, executor *governance.Executor, logger *zap.Logger) *ControlService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ControlService{
		control:  control,
		executor: executor,
		logger:   logger.Named("rpc_control"),
	}
}

func (s *ControlService) GetInfo(ctx context.Context, _ *controlv1.GetInfoRequest) (*controlv1.GetInfoResponse, error) {
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
	if req.GetServer() != "" && len(req.GetTags()) > 0 {
		return nil, status.Error(codes.InvalidArgument, "server and tags are mutually exclusive")
	}
	registration, err := s.control.RegisterClient(ctx, client, int(req.GetPid()), req.GetTags(), req.GetServer())
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
	decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
		Method: "tools/list",
		Caller: client,
	})
	if err != nil {
		return nil, mapGovernanceError(err)
	}
	if !decision.Continue {
		return nil, mapGovernanceDecision(decision)
	}
	snapshot, err := s.control.ListTools(ctx, client)
	if err != nil {
		return nil, mapClientError("list tools", err)
	}
	protoSnapshot, err := toProtoSnapshot(snapshot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tools: %v", err)
	}
	if s.executor != nil {
		raw, err := protojson.Marshal(protoSnapshot)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list tools: response encode failed: %v", err)
		}
		respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
			Method: "tools/list",
			Caller: client,
		}, raw)
		if err != nil {
			return nil, mapGovernanceError(err)
		}
		if !respDecision.Continue {
			return nil, mapGovernanceDecision(respDecision)
		}
		if err := applyProtoMutation(protoSnapshot, respDecision.ResponseJSON); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "list tools: response mutation invalid: %v", err)
		}
	}
	return &controlv1.ListToolsResponse{
		Snapshot: protoSnapshot,
	}, nil
}

func (s *ControlService) WatchTools(req *controlv1.WatchToolsRequest, stream controlv1.ControlPlaneService_WatchToolsServer) error {
	ctx := stream.Context()
	client := req.GetCaller()

	decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
		Method: "tools/list",
		Caller: client,
	})
	if err != nil {
		return mapGovernanceError(err)
	}
	if !decision.Continue {
		return mapGovernanceDecision(decision)
	}

	// WatchTools atomically subscribes and returns the initial snapshot,
	// eliminating the race condition between ListTools and subscription.
	updates, err := s.control.WatchTools(ctx, client)
	if err != nil {
		return mapClientError("watch tools", err)
	}

	// Client's lastETag enables incremental sync optimization
	lastETag := req.GetLastEtag()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case snapshot, ok := <-updates:
			if !ok {
				return nil
			}
			// Skip if client already has this version
			if lastETag != "" && lastETag == snapshot.ETag {
				continue
			}
			protoSnapshot, err := toProtoSnapshot(snapshot)
			if err != nil {
				return status.Errorf(codes.Internal, "watch tools: %v", err)
			}
			if s.executor != nil {
				raw, err := protojson.Marshal(protoSnapshot)
				if err != nil {
					return status.Errorf(codes.Internal, "watch tools: response encode failed: %v", err)
				}
				respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
					Method: "tools/list",
					Caller: client,
				}, raw)
				if err != nil {
					return mapGovernanceError(err)
				}
				if !respDecision.Continue {
					return mapGovernanceDecision(respDecision)
				}
				if err := applyProtoMutation(protoSnapshot, respDecision.ResponseJSON); err != nil {
					return status.Errorf(codes.InvalidArgument, "watch tools: response mutation invalid: %v", err)
				}
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
	toolName := req.GetName()
	var result json.RawMessage
	var err error
	if s.executor != nil {
		result, err = s.executor.Execute(ctx, domain.GovernanceRequest{
			Method:      "tools/call",
			Caller:      client,
			ToolName:    toolName,
			RoutingKey:  req.GetRoutingKey(),
			RequestJSON: req.GetArgumentsJson(),
		}, func(nextCtx context.Context, govReq domain.GovernanceRequest) (json.RawMessage, error) {
			args := govReq.RequestJSON
			if len(args) == 0 {
				args = req.GetArgumentsJson()
			}
			return s.control.CallTool(nextCtx, client, toolName, args, req.GetRoutingKey())
		})
	} else {
		result, err = s.control.CallTool(ctx, client, toolName, req.GetArgumentsJson(), req.GetRoutingKey())
	}
	if err != nil {
		var rej domain.GovernanceRejection
		if errors.As(err, &rej) {
			return nil, mapGovernanceError(err)
		}
		return nil, mapCallToolError(toolName, err)
	}
	if len(result) == 0 {
		return nil, status.Error(codes.Internal, "call tool: empty result")
	}
	return &controlv1.CallToolResponse{
		ResultJson: result,
	}, nil
}

func (s *ControlService) CallToolTask(ctx context.Context, req *controlv1.CallToolTaskRequest) (*controlv1.CallToolTaskResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	opts := domain.TaskCreateOptions{}
	if req.GetTtlMs() > 0 {
		ttl := req.GetTtlMs()
		opts.TTL = &ttl
	}
	if req.GetPollIntervalMs() > 0 {
		poll := req.GetPollIntervalMs()
		opts.PollInterval = &poll
	}
	client := req.GetCaller()
	task, err := s.control.CallToolTask(ctx, client, req.GetName(), req.GetArgumentsJson(), req.GetRoutingKey(), opts)
	if err != nil {
		return nil, mapCallToolError(req.GetName(), err)
	}
	return &controlv1.CallToolTaskResponse{
		Task: toProtoTask(task),
	}, nil
}

func (s *ControlService) TasksGet(ctx context.Context, req *controlv1.TasksGetRequest) (*controlv1.TasksGetResponse, error) {
	if req.GetTaskId() == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	task, err := s.control.GetTask(ctx, req.GetCaller(), req.GetTaskId())
	if err != nil {
		return nil, mapTaskError("get task", err)
	}
	return &controlv1.TasksGetResponse{Task: toProtoTask(task)}, nil
}

func (s *ControlService) TasksList(ctx context.Context, req *controlv1.TasksListRequest) (*controlv1.TasksListResponse, error) {
	page, err := s.control.ListTasks(ctx, req.GetCaller(), req.GetCursor(), int(req.GetLimit()))
	if err != nil {
		return nil, mapTaskError("list tasks", err)
	}
	tasks := make([]*controlv1.Task, 0, len(page.Tasks))
	for _, task := range page.Tasks {
		tasks = append(tasks, toProtoTask(task))
	}
	return &controlv1.TasksListResponse{
		Tasks:      tasks,
		NextCursor: page.NextCursor,
	}, nil
}

func (s *ControlService) TasksResult(ctx context.Context, req *controlv1.TasksResultRequest) (*controlv1.TasksResultResponse, error) {
	if req.GetTaskId() == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	result, err := s.control.GetTaskResult(ctx, req.GetCaller(), req.GetTaskId())
	if err != nil {
		return nil, mapTaskError("get task result", err)
	}
	return &controlv1.TasksResultResponse{
		Result: toProtoTaskResult(result),
	}, nil
}

func (s *ControlService) TasksCancel(ctx context.Context, req *controlv1.TasksCancelRequest) (*controlv1.TasksCancelResponse, error) {
	if req.GetTaskId() == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	task, err := s.control.CancelTask(ctx, req.GetCaller(), req.GetTaskId())
	if err != nil {
		return nil, mapTaskError("cancel task", err)
	}
	return &controlv1.TasksCancelResponse{Task: toProtoTask(task)}, nil
}

func (s *ControlService) ListResources(ctx context.Context, req *controlv1.ListResourcesRequest) (*controlv1.ListResourcesResponse, error) {
	client := req.GetCaller()
	cursor := req.GetCursor()
	decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
		Method:      "resources/list",
		Caller:      client,
		RequestJSON: mustMarshalJSON(map[string]string{"cursor": cursor}),
	})
	if err != nil {
		return nil, mapGovernanceError(err)
	}
	if !decision.Continue {
		return nil, mapGovernanceDecision(decision)
	}
	if len(decision.RequestJSON) > 0 {
		var params struct {
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(decision.RequestJSON, &params); err != nil {
			return nil, status.Error(codes.InvalidArgument, "list resources: invalid request mutation")
		}
		cursor = params.Cursor
	}
	page, err := s.control.ListResources(ctx, client, cursor)
	if err != nil {
		return nil, mapListError("list resources", err)
	}
	resourcesSnapshot, err := toProtoResourcesSnapshot(page.Snapshot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list resources: %v", err)
	}
	if s.executor != nil {
		raw, err := protojson.Marshal(resourcesSnapshot)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list resources: response encode failed: %v", err)
		}
		respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
			Method: "resources/list",
			Caller: client,
		}, raw)
		if err != nil {
			return nil, mapGovernanceError(err)
		}
		if !respDecision.Continue {
			return nil, mapGovernanceDecision(respDecision)
		}
		if err := applyProtoMutation(resourcesSnapshot, respDecision.ResponseJSON); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "list resources: response mutation invalid: %v", err)
		}
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
	decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
		Method: "resources/list",
		Caller: client,
	})
	if err != nil {
		return mapGovernanceError(err)
	}
	if !decision.Continue {
		return mapGovernanceDecision(decision)
	}
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
			if s.executor != nil {
				raw, err := protojson.Marshal(protoSnapshot)
				if err != nil {
					return status.Errorf(codes.Internal, "watch resources: response encode failed: %v", err)
				}
				respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
					Method: "resources/list",
					Caller: client,
				}, raw)
				if err != nil {
					return mapGovernanceError(err)
				}
				if !respDecision.Continue {
					return mapGovernanceDecision(respDecision)
				}
				if err := applyProtoMutation(protoSnapshot, respDecision.ResponseJSON); err != nil {
					return status.Errorf(codes.InvalidArgument, "watch resources: response mutation invalid: %v", err)
				}
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
	uri := req.GetUri()
	var result json.RawMessage
	var err error
	if s.executor != nil {
		result, err = s.executor.Execute(ctx, domain.GovernanceRequest{
			Method:      "resources/read",
			Caller:      client,
			ResourceURI: uri,
			RequestJSON: mustMarshalJSON(map[string]string{"uri": uri}),
		}, func(nextCtx context.Context, govReq domain.GovernanceRequest) (json.RawMessage, error) {
			target := uri
			if len(govReq.RequestJSON) > 0 {
				var params struct {
					URI string `json:"uri"`
				}
				if err := json.Unmarshal(govReq.RequestJSON, &params); err != nil || strings.TrimSpace(params.URI) == "" {
					return nil, domain.ErrInvalidRequest
				}
				target = params.URI
			}
			return s.control.ReadResource(nextCtx, client, target)
		})
	} else {
		result, err = s.control.ReadResource(ctx, client, uri)
	}
	if err != nil {
		var rej domain.GovernanceRejection
		if errors.As(err, &rej) {
			return nil, mapGovernanceError(err)
		}
		return nil, mapReadResourceError(uri, err)
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
	cursor := req.GetCursor()
	decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
		Method:      "prompts/list",
		Caller:      client,
		RequestJSON: mustMarshalJSON(map[string]string{"cursor": cursor}),
	})
	if err != nil {
		return nil, mapGovernanceError(err)
	}
	if !decision.Continue {
		return nil, mapGovernanceDecision(decision)
	}
	if len(decision.RequestJSON) > 0 {
		var params struct {
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(decision.RequestJSON, &params); err != nil {
			return nil, status.Error(codes.InvalidArgument, "list prompts: invalid request mutation")
		}
		cursor = params.Cursor
	}
	page, err := s.control.ListPrompts(ctx, client, cursor)
	if err != nil {
		return nil, mapListError("list prompts", err)
	}
	promptsSnapshot, err := toProtoPromptsSnapshot(page.Snapshot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list prompts: %v", err)
	}
	if s.executor != nil {
		raw, err := protojson.Marshal(promptsSnapshot)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list prompts: response encode failed: %v", err)
		}
		respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
			Method: "prompts/list",
			Caller: client,
		}, raw)
		if err != nil {
			return nil, mapGovernanceError(err)
		}
		if !respDecision.Continue {
			return nil, mapGovernanceDecision(respDecision)
		}
		if err := applyProtoMutation(promptsSnapshot, respDecision.ResponseJSON); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "list prompts: response mutation invalid: %v", err)
		}
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
	decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
		Method: "prompts/list",
		Caller: client,
	})
	if err != nil {
		return mapGovernanceError(err)
	}
	if !decision.Continue {
		return mapGovernanceDecision(decision)
	}
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
			if s.executor != nil {
				raw, err := protojson.Marshal(protoSnapshot)
				if err != nil {
					return status.Errorf(codes.Internal, "watch prompts: response encode failed: %v", err)
				}
				respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
					Method: "prompts/list",
					Caller: client,
				}, raw)
				if err != nil {
					return mapGovernanceError(err)
				}
				if !respDecision.Continue {
					return mapGovernanceDecision(respDecision)
				}
				if err := applyProtoMutation(protoSnapshot, respDecision.ResponseJSON); err != nil {
					return status.Errorf(codes.InvalidArgument, "watch prompts: response mutation invalid: %v", err)
				}
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
	promptName := req.GetName()
	var result json.RawMessage
	var err error
	if s.executor != nil {
		result, err = s.executor.Execute(ctx, domain.GovernanceRequest{
			Method:      "prompts/get",
			Caller:      client,
			PromptName:  promptName,
			RequestJSON: req.GetArgumentsJson(),
		}, func(nextCtx context.Context, govReq domain.GovernanceRequest) (json.RawMessage, error) {
			args := govReq.RequestJSON
			if len(args) == 0 {
				args = req.GetArgumentsJson()
			}
			return s.control.GetPrompt(nextCtx, client, promptName, args)
		})
	} else {
		result, err = s.control.GetPrompt(ctx, client, promptName, req.GetArgumentsJson())
	}
	if err != nil {
		var rej domain.GovernanceRejection
		if errors.As(err, &rej) {
			return nil, mapGovernanceError(err)
		}
		return nil, mapGetPromptError(promptName, err)
	}
	if len(result) == 0 {
		return nil, status.Error(codes.Internal, "get prompt: empty result")
	}
	return &controlv1.GetPromptResponse{
		ResultJson: result,
	}, nil
}

func mapCallToolError(name string, err error) error {
	var protoErr *domain.ProtocolError
	if errors.As(err, &protoErr) {
		return status.Errorf(codes.FailedPrecondition, "call tool requires elicitation: %s", protoErr.Message)
	}
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

func mapGovernanceError(err error) error {
	if err == nil {
		return nil
	}
	var rej domain.GovernanceRejection
	if errors.As(err, &rej) {
		return mapGovernanceRejection(rej.Code, rej.Message, rej.Category, rej.Plugin)
	}
	return status.Errorf(codes.Unavailable, "governance failure: %v", err)
}

func mapGovernanceDecision(decision domain.GovernanceDecision) error {
	if decision.Continue {
		return nil
	}
	return mapGovernanceRejection(decision.RejectCode, decision.RejectMessage, decision.Category, decision.Plugin)
}

func mapGovernanceRejection(code, message string, category domain.PluginCategory, pluginName string) error {
	grpcCode := governanceCode(code)
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "request rejected"
	}
	if category != "" {
		if pluginName != "" {
			msg = fmt.Sprintf("governance rejected by %s/%s: %s", category, pluginName, msg)
		} else {
			msg = fmt.Sprintf("governance rejected by %s: %s", category, msg)
		}
	}
	return status.Error(grpcCode, msg)
}

func governanceCode(code string) codes.Code {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "unauthenticated":
		return codes.Unauthenticated
	case "unauthorized":
		return codes.PermissionDenied
	case "rate_limited":
		return codes.ResourceExhausted
	case "invalid_request":
		return codes.InvalidArgument
	default:
		return codes.PermissionDenied
	}
}
func (s *ControlService) requestDecision(ctx context.Context, req domain.GovernanceRequest) (domain.GovernanceDecision, error) {
	if s.executor == nil {
		return domain.GovernanceDecision{Continue: true}, nil
	}
	return s.executor.Request(ctx, req)
}

func (s *ControlService) responseDecision(ctx context.Context, req domain.GovernanceRequest, responseJSON []byte) (domain.GovernanceDecision, error) {
	if s.executor == nil {
		return domain.GovernanceDecision{Continue: true}, nil
	}
	req.ResponseJSON = responseJSON
	return s.executor.Response(ctx, req)
}

func applyProtoMutation(target proto.Message, raw []byte) error {
	if len(raw) == 0 || target == nil {
		return nil
	}
	return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(raw, target)
}

func mustMarshalJSON(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
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

func mapTaskError(op string, err error) error {
	if errors.Is(err, domain.ErrTaskNotFound) {
		return status.Errorf(codes.NotFound, "%s: task not found", op)
	}
	if errors.Is(err, domain.ErrTasksNotImplemented) {
		return status.Errorf(codes.Unimplemented, "%s: tasks not implemented", op)
	}
	if errors.Is(err, domain.ErrInvalidCursor) {
		return status.Errorf(codes.InvalidArgument, "%s: invalid cursor", op)
	}
	if errors.Is(err, domain.ErrClientNotRegistered) {
		return status.Errorf(codes.FailedPrecondition, "%s: client not registered", op)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Errorf(codes.DeadlineExceeded, "%s: deadline exceeded", op)
	}
	if errors.Is(err, context.Canceled) {
		return status.Errorf(codes.Canceled, "%s: canceled", op)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

func toProtoTask(task domain.Task) *controlv1.Task {
	if task.TaskID == "" {
		return &controlv1.Task{}
	}
	ttl := int64(0)
	if task.TTL != nil {
		ttl = *task.TTL
	}
	poll := int64(0)
	if task.PollInterval != nil {
		poll = *task.PollInterval
	}
	return &controlv1.Task{
		TaskId:         task.TaskID,
		Status:         string(task.Status),
		StatusMessage:  task.StatusMessage,
		CreatedAt:      task.CreatedAt.UTC().Format(time.RFC3339Nano),
		LastUpdatedAt:  task.LastUpdatedAt.UTC().Format(time.RFC3339Nano),
		TtlMs:          ttl,
		PollIntervalMs: poll,
	}
}

func toProtoTaskResult(result domain.TaskResult) *controlv1.TaskResult {
	resp := &controlv1.TaskResult{
		Status: string(result.Status),
	}
	if len(result.Result) > 0 {
		resp.ResultJson = result.Result
	}
	if result.Error != nil {
		resp.ErrorCode = result.Error.Code
		resp.ErrorMessage = result.Error.Message
		resp.ErrorDataJson = result.Error.Data
	}
	return resp
}

func (s *ControlService) StreamLogs(req *controlv1.StreamLogsRequest, stream controlv1.ControlPlaneService_StreamLogsServer) error {
	ctx := stream.Context()
	minLevel := fromProtoLogLevel(req.GetMinLevel())
	client := req.GetCaller()
	decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
		Method:      "logging/subscribe",
		Caller:      client,
		RequestJSON: mustMarshalJSON(map[string]any{"minLevel": string(minLevel)}),
	})
	if err != nil {
		return mapGovernanceError(err)
	}
	if !decision.Continue {
		return mapGovernanceDecision(decision)
	}
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
			protoEntry := toProtoLogEntry(entry)
			// TODO：Maybe implement governance for log streaming later
			// Skip plugin processing for log streaming to avoid infinite loops
			// Plugins should not process logging operations
			// if s.executor != nil {
			// 	raw, err := protojson.Marshal(protoEntry)
			// 	if err != nil {
			// 		return status.Errorf(codes.Internal, "stream logs: response encode failed: %v", err)
			// 	}
			// 	respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
			// 		Method: "logging/subscribe",
			// 		Caller: client,
			// 	}, raw)
			// 	if err != nil {
			// 		return mapGovernanceError(err)
			// 	}
			// 	if !respDecision.Continue {
			// 		return mapGovernanceDecision(respDecision)
			// 	}
			// 	if err := applyProtoMutation(protoEntry, respDecision.ResponseJSON); err != nil {
			// 		return status.Errorf(codes.InvalidArgument, "stream logs: response mutation invalid: %v", err)
			// 	}
			// }
			if err := stream.Send(protoEntry); err != nil {
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
	decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
		Method: "mcpv/runtime/watch",
		Caller: client,
	})
	if err != nil {
		return mapGovernanceError(err)
	}
	if !decision.Continue {
		return mapGovernanceDecision(decision)
	}
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
			protoSnapshot := toProtoRuntimeStatusSnapshot(snapshot)
			if s.executor != nil {
				raw, err := protojson.Marshal(protoSnapshot)
				if err != nil {
					return status.Errorf(codes.Internal, "watch runtime status: response encode failed: %v", err)
				}
				respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
					Method: "mcpv/runtime/watch",
					Caller: client,
				}, raw)
				if err != nil {
					return mapGovernanceError(err)
				}
				if !respDecision.Continue {
					return mapGovernanceDecision(respDecision)
				}
				if err := applyProtoMutation(protoSnapshot, respDecision.ResponseJSON); err != nil {
					return status.Errorf(codes.InvalidArgument, "watch runtime status: response mutation invalid: %v", err)
				}
			}
			if err := stream.Send(protoSnapshot); err != nil {
				return err
			}
			lastETag = snapshot.ETag
		}
	}
}

func (s *ControlService) WatchServerInitStatus(req *controlv1.WatchServerInitStatusRequest, stream controlv1.ControlPlaneService_WatchServerInitStatusServer) error {
	ctx := stream.Context()

	client := req.GetCaller()
	decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
		Method: "mcpv/server_init/watch",
		Caller: client,
	})
	if err != nil {
		return mapGovernanceError(err)
	}
	if !decision.Continue {
		return mapGovernanceDecision(decision)
	}
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
			protoSnapshot := toProtoServerInitStatusSnapshot(snapshot)
			if s.executor != nil {
				raw, err := protojson.Marshal(protoSnapshot)
				if err != nil {
					return status.Errorf(codes.Internal, "watch server init status: response encode failed: %v", err)
				}
				respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
					Method: "mcpv/server_init/watch",
					Caller: client,
				}, raw)
				if err != nil {
					return mapGovernanceError(err)
				}
				if !respDecision.Continue {
					return mapGovernanceDecision(respDecision)
				}
				if err := applyProtoMutation(protoSnapshot, respDecision.ResponseJSON); err != nil {
					return status.Errorf(codes.InvalidArgument, "watch server init status: response mutation invalid: %v", err)
				}
			}
			if err := stream.Send(protoSnapshot); err != nil {
				return err
			}
		}
	}
}

// AutomaticMCP handles the automatic_mcp tool call for SubAgent.
func (s *ControlService) AutomaticMCP(ctx context.Context, req *controlv1.AutomaticMCPRequest) (*controlv1.AutomaticMCPResponse, error) {
	client := req.GetCaller()
	params := domain.AutomaticMCPParams{
		Query:        req.GetQuery(),
		SessionID:    req.GetSessionId(),
		ForceRefresh: req.GetForceRefresh(),
	}

	if s.executor != nil {
		decision, err := s.requestDecision(ctx, domain.GovernanceRequest{
			Method:      "tools/call",
			Caller:      client,
			ToolName:    "mcpv.automatic_mcp",
			RequestJSON: mustMarshalJSON(params),
		})
		if err != nil {
			return nil, mapGovernanceError(err)
		}
		if !decision.Continue {
			return nil, mapGovernanceDecision(decision)
		}
		if len(decision.RequestJSON) > 0 {
			if err := json.Unmarshal(decision.RequestJSON, &params); err != nil {
				return nil, status.Error(codes.InvalidArgument, "automatic_mcp: invalid request mutation")
			}
		}
	}

	result, err := s.control.AutomaticMCP(ctx, client, params)
	if err != nil {
		return nil, mapClientError("automatic_mcp", err)
	}

	resp, err := toProtoAutomaticMCPResponse(result)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "automatic_mcp: %v", err)
	}
	if s.executor != nil {
		raw, err := protojson.Marshal(resp)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "automatic_mcp: response encode failed: %v", err)
		}
		respDecision, err := s.responseDecision(ctx, domain.GovernanceRequest{
			Method:   "tools/call",
			Caller:   client,
			ToolName: "mcpv.automatic_mcp",
		}, raw)
		if err != nil {
			return nil, mapGovernanceError(err)
		}
		if !respDecision.Continue {
			return nil, mapGovernanceDecision(respDecision)
		}
		if err := applyProtoMutation(resp, respDecision.ResponseJSON); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "automatic_mcp: response mutation invalid: %v", err)
		}
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
	var result json.RawMessage
	var err error
	if s.executor != nil {
		result, err = s.executor.Execute(ctx, domain.GovernanceRequest{
			Method:      "tools/call",
			Caller:      client,
			ToolName:    "mcpv.automatic_eval",
			RoutingKey:  params.RoutingKey,
			RequestJSON: mustMarshalJSON(params),
		}, func(nextCtx context.Context, govReq domain.GovernanceRequest) (json.RawMessage, error) {
			evalParams := params
			if len(govReq.RequestJSON) > 0 {
				if err := json.Unmarshal(govReq.RequestJSON, &evalParams); err != nil {
					return nil, domain.ErrInvalidRequest
				}
			}
			return s.control.AutomaticEval(nextCtx, client, evalParams)
		})
	} else {
		result, err = s.control.AutomaticEval(ctx, client, params)
	}
	if err != nil {
		var rej domain.GovernanceRejection
		if errors.As(err, &rej) {
			return nil, mapGovernanceError(err)
		}
		return nil, mapCallToolError(req.GetToolName(), err)
	}

	return &controlv1.AutomaticEvalResponse{
		ResultJson: result,
	}, nil
}

// IsSubAgentEnabled returns whether the SubAgent is enabled for the caller's profile.
func (s *ControlService) IsSubAgentEnabled(_ context.Context, req *controlv1.IsSubAgentEnabledRequest) (*controlv1.IsSubAgentEnabledResponse, error) {
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
