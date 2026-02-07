package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (s *ControlService) ListResources(ctx context.Context, req *controlv1.ListResourcesRequest) (*controlv1.ListResourcesResponse, error) {
	client := req.GetCaller()
	cursor := req.GetCursor()
	if err := s.guard.applyRequest(ctx, domain.GovernanceRequest{
		Method:      "resources/list",
		Caller:      client,
		RequestJSON: mustMarshalJSON(map[string]string{"cursor": cursor}),
	}, "list resources", func(raw []byte) error {
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
	page, err := s.control.ListResources(ctx, client, cursor)
	if err != nil {
		return nil, mapListError("list resources", err)
	}
	resourcesSnapshot, err := toProtoResourcesSnapshot(page.Snapshot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list resources: %v", err)
	}
	if err := s.guard.applyProtoResponse(ctx, domain.GovernanceRequest{
		Method: "resources/list",
		Caller: client,
	}, "list resources", resourcesSnapshot); err != nil {
		return nil, err
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
	if err := s.guard.applyRequest(ctx, domain.GovernanceRequest{
		Method: "resources/list",
		Caller: client,
	}, "watch resources", nil); err != nil {
		return err
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
			if err := s.guard.applyProtoResponse(ctx, domain.GovernanceRequest{
				Method: "resources/list",
				Caller: client,
			}, "watch resources", protoSnapshot); err != nil {
				return err
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
