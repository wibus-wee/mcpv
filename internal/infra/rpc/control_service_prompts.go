package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (s *ControlService) ListPrompts(ctx context.Context, req *controlv1.ListPromptsRequest) (*controlv1.ListPromptsResponse, error) {
	client := req.GetCaller()
	cursor := req.GetCursor()
	promptsSnapshot, nextCursor, err := guardedList(
		ctx,
		&s.guard,
		domain.GovernanceRequest{
			Method:      "prompts/list",
			Caller:      client,
			RequestJSON: mustMarshalJSON(map[string]string{"cursor": cursor}),
		},
		domain.GovernanceRequest{
			Method: "prompts/list",
			Caller: client,
		},
		"list prompts",
		func(raw []byte) error {
			var params struct {
				Cursor string `json:"cursor"`
			}
			if err := json.Unmarshal(raw, &params); err != nil {
				return err
			}
			cursor = params.Cursor
			return nil
		},
		func(ctx context.Context) (domain.PromptPage, error) {
			return s.control.ListPrompts(ctx, client, cursor)
		},
		func(page domain.PromptPage) (*controlv1.PromptsSnapshot, string, error) {
			out, err := toProtoPromptsSnapshot(page.Snapshot)
			return out, page.NextCursor, err
		},
		func(err error) error {
			return statusFromError("list prompts", err)
		},
	)
	if err != nil {
		return nil, err
	}
	return &controlv1.ListPromptsResponse{
		Snapshot:   promptsSnapshot,
		NextCursor: nextCursor,
	}, nil
}

func (s *ControlService) WatchPrompts(req *controlv1.WatchPromptsRequest, stream controlv1.ControlPlaneService_WatchPromptsServer) error {
	ctx := stream.Context()
	client := req.GetCaller()
	return guardedWatch(
		ctx,
		&s.guard,
		domain.GovernanceRequest{
			Method: "prompts/list",
			Caller: client,
		},
		"watch prompts",
		req.GetLastEtag(),
		func(ctx context.Context) (<-chan domain.PromptSnapshot, error) {
			return s.control.WatchPrompts(ctx, client)
		},
		func(snapshot domain.PromptSnapshot) string {
			return snapshot.ETag
		},
		func(last string, snapshot domain.PromptSnapshot) bool {
			return last == snapshot.ETag
		},
		toProtoPromptsSnapshot,
		func(err error) error {
			return statusFromError("watch prompts", err)
		},
		stream.Send,
	)
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
		return nil, statusFromError(fmt.Sprintf("get prompt %s", promptName), err)
	}
	if len(result) == 0 {
		return nil, status.Error(codes.Internal, "get prompt: empty result")
	}
	return &controlv1.GetPromptResponse{
		ResultJson: result,
	}, nil
}
