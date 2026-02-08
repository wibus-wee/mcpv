package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	"mcpv/internal/infra/mcpcodec"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (s *ControlService) ListTools(ctx context.Context, req *controlv1.ListToolsRequest) (*controlv1.ListToolsResponse, error) {
	client := req.GetCaller()
	protoSnapshot, _, err := guardedList(guardedListPlan[domain.ToolSnapshot, *controlv1.ToolsSnapshot]{
		ctx:   ctx,
		guard: &s.guard,
		request: domain.GovernanceRequest{
			Method: "tools/list",
			Caller: client,
		},
		responseRequest: domain.GovernanceRequest{
			Method: "tools/list",
			Caller: client,
		},
		op:     "list tools",
		mutate: nil,
		call: func(ctx context.Context) (domain.ToolSnapshot, error) {
			return s.control.ListTools(ctx, client)
		},
		toProto: func(snapshot domain.ToolSnapshot) (*controlv1.ToolsSnapshot, string, error) {
			out, err := toProtoSnapshot(snapshot)
			return out, "", err
		},
		mapError: func(err error) error {
			return statusFromError("list tools", err)
		},
	})
	if err != nil {
		return nil, err
	}
	return &controlv1.ListToolsResponse{
		Snapshot: protoSnapshot,
	}, nil
}

func (s *ControlService) WatchTools(req *controlv1.WatchToolsRequest, stream controlv1.ControlPlaneService_WatchToolsServer) error {
	ctx := stream.Context()
	client := req.GetCaller()
	return guardedWatch(guardedWatchPlan[domain.ToolSnapshot, *controlv1.ToolsSnapshot]{
		ctx:   ctx,
		guard: &s.guard,
		request: domain.GovernanceRequest{
			Method: "tools/list",
			Caller: client,
		},
		op:       "watch tools",
		lastETag: req.GetLastEtag(),
		subscribe: func(ctx context.Context) (<-chan domain.ToolSnapshot, error) {
			return s.control.WatchTools(ctx, client)
		},
		etag: func(snapshot domain.ToolSnapshot) string {
			return snapshot.ETag
		},
		toProto: toProtoSnapshot,
		mapError: func(err error) error {
			return statusFromError("watch tools", err)
		},
		send: stream.Send,
	})
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
		return nil, statusFromError(fmt.Sprintf("call tool %s", toolName), err)
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
		return nil, statusFromError(fmt.Sprintf("call tool task %s", req.GetName()), err)
	}
	return &controlv1.CallToolTaskResponse{
		Task: toProtoTask(task),
	}, nil
}

// AutomaticMCP handles the automatic_mcp tool call for SubAgent.
func (s *ControlService) AutomaticMCP(ctx context.Context, req *controlv1.AutomaticMCPRequest) (*controlv1.AutomaticMCPResponse, error) {
	client := req.GetCaller()
	params := domain.AutomaticMCPParams{
		Query:        req.GetQuery(),
		SessionID:    req.GetSessionId(),
		ForceRefresh: req.GetForceRefresh(),
	}

	if err := s.guard.applyRequest(ctx, domain.GovernanceRequest{
		Method:      "tools/call",
		Caller:      client,
		ToolName:    "mcpv.automatic_mcp",
		RequestJSON: mustMarshalJSON(params),
	}, "automatic_mcp", func(raw []byte) error {
		return json.Unmarshal(raw, &params)
	}); err != nil {
		return nil, err
	}

	result, err := s.control.AutomaticMCP(ctx, client, params)
	if err != nil {
		return nil, statusFromError("automatic_mcp", err)
	}

	resp, err := toProtoAutomaticMCPResponse(result)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "automatic_mcp: %v", err)
	}
	if err := s.guard.applyProtoResponse(ctx, domain.GovernanceRequest{
		Method:   "tools/call",
		Caller:   client,
		ToolName: "mcpv.automatic_mcp",
	}, "automatic_mcp", resp); err != nil {
		return nil, err
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
		return nil, statusFromError(fmt.Sprintf("automatic_eval %s", req.GetToolName()), err)
	}

	return &controlv1.AutomaticEvalResponse{
		ResultJson: result,
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
