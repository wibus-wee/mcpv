package rpc

import (
	"context"
	"encoding/json"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (s *ControlService) ListPrompts(ctx context.Context, req *controlv1.ListPromptsRequest) (*controlv1.ListPromptsResponse, error) {
	client := req.GetCaller()
	cursor := req.GetCursor()
	if err := s.guard.applyRequest(ctx, domain.GovernanceRequest{
		Method:      "prompts/list",
		Caller:      client,
		RequestJSON: mustMarshalJSON(map[string]string{"cursor": cursor}),
	}, "list prompts", func(raw []byte) error {
		var params struct {
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(raw, &params); err != nil {
			return err
		}
		cursor = params.Cursor
		return nil
	}); err != nil {
		return nil, err
	}
	page, err := s.control.ListPrompts(ctx, client, cursor)
	if err != nil {
		return nil, mapListError("list prompts", err)
	}
	promptsSnapshot, err := toProtoPromptsSnapshot(page.Snapshot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list prompts: %v", err)
	}
	if err := s.guard.applyProtoResponse(ctx, domain.GovernanceRequest{
		Method: "prompts/list",
		Caller: client,
	}, "list prompts", promptsSnapshot); err != nil {
		return nil, err
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
	if err := s.guard.applyRequest(ctx, domain.GovernanceRequest{
		Method: "prompts/list",
		Caller: client,
	}, "watch prompts", nil); err != nil {
		return err
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
			if err := s.guard.applyProtoResponse(ctx, domain.GovernanceRequest{
				Method: "prompts/list",
				Caller: client,
			}, "watch prompts", protoSnapshot); err != nil {
				return err
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
