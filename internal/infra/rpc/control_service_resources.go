package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (s *ControlService) ListResources(ctx context.Context, req *controlv1.ListResourcesRequest) (*controlv1.ListResourcesResponse, error) {
	client := req.GetCaller()
	cursor := req.GetCursor()
	resourcesSnapshot, nextCursor, err := guardedList(guardedListPlan[domain.ResourcePage, *controlv1.ResourcesSnapshot]{
		ctx:   ctx,
		guard: &s.guard,
		request: domain.GovernanceRequest{
			Method:      "resources/list",
			Caller:      client,
			RequestJSON: mustMarshalJSON(map[string]string{"cursor": cursor}),
		},
		responseRequest: domain.GovernanceRequest{
			Method: "resources/list",
			Caller: client,
		},
		op: "list resources",
		mutate: func(raw []byte) error {
			var params struct {
				Cursor string `json:"cursor"`
			}
			if err := json.Unmarshal(raw, &params); err != nil {
				return err
			}
			cursor = params.Cursor
			return nil
		},
		call: func(ctx context.Context) (domain.ResourcePage, error) {
			return s.control.ListResources(ctx, client, cursor)
		},
		toProto: func(page domain.ResourcePage) (*controlv1.ResourcesSnapshot, string, error) {
			out, err := toProtoResourcesSnapshot(page.Snapshot)
			return out, page.NextCursor, err
		},
		mapError: func(err error) error {
			return statusFromError("list resources", err)
		},
	})
	if err != nil {
		return nil, err
	}
	return &controlv1.ListResourcesResponse{
		Snapshot:   resourcesSnapshot,
		NextCursor: nextCursor,
	}, nil
}

func (s *ControlService) WatchResources(req *controlv1.WatchResourcesRequest, stream controlv1.ControlPlaneService_WatchResourcesServer) error {
	ctx := stream.Context()
	client := req.GetCaller()
	return guardedWatch(guardedWatchPlan[domain.ResourceSnapshot, *controlv1.ResourcesSnapshot]{
		ctx:   ctx,
		guard: &s.guard,
		request: domain.GovernanceRequest{
			Method: "resources/list",
			Caller: client,
		},
		op:       "watch resources",
		lastETag: req.GetLastEtag(),
		subscribe: func(ctx context.Context) (<-chan domain.ResourceSnapshot, error) {
			return s.control.WatchResources(ctx, client)
		},
		etag: func(snapshot domain.ResourceSnapshot) string {
			return snapshot.ETag
		},
		toProto: toProtoResourcesSnapshot,
		mapError: func(err error) error {
			return statusFromError("watch resources", err)
		},
		send: stream.Send,
	})
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
		return nil, statusFromError(fmt.Sprintf("read resource %s", uri), err)
	}
	if len(result) == 0 {
		return nil, status.Error(codes.Internal, "read resource: empty result")
	}
	return &controlv1.ReadResourceResponse{
		ResultJson: result,
	}, nil
}
