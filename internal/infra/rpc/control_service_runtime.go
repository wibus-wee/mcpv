package rpc

import (
	"context"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (s *ControlService) StreamLogs(req *controlv1.StreamLogsRequest, stream controlv1.ControlPlaneService_StreamLogsServer) error {
	ctx := stream.Context()
	minLevel := fromProtoLogLevel(req.GetMinLevel())
	client := req.GetCaller()
	if err := s.guard.applyRequest(ctx, domain.GovernanceRequest{
		Method:      "logging/subscribe",
		Caller:      client,
		RequestJSON: mustMarshalJSON(map[string]any{"minLevel": string(minLevel)}),
	}, "stream logs", nil); err != nil {
		return err
	}
	entries, err := s.control.StreamLogs(ctx, client, minLevel)
	if err != nil {
		return statusFromError("stream logs", err)
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
			// TODO: Maybe implement governance for log streaming later.
			// Skip plugin processing for log streaming to avoid infinite loops.
			// Plugins should not process logging operations.
			if err := stream.Send(protoEntry); err != nil {
				return err
			}
		}
	}
}

func (s *ControlService) WatchRuntimeStatus(req *controlv1.WatchRuntimeStatusRequest, stream controlv1.ControlPlaneService_WatchRuntimeStatusServer) error {
	ctx := stream.Context()

	client := req.GetCaller()
	return guardedWatch(guardedWatchPlan[domain.RuntimeStatusSnapshot, *controlv1.RuntimeStatusSnapshot]{
		ctx:   ctx,
		guard: &s.guard,
		request: domain.GovernanceRequest{
			Method: "mcpv/runtime/watch",
			Caller: client,
		},
		op:       "watch runtime status",
		lastETag: req.GetLastEtag(),
		subscribe: func(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
			return s.control.WatchRuntimeStatus(ctx, client)
		},
		etag: func(snapshot domain.RuntimeStatusSnapshot) string {
			return snapshot.ETag
		},
		toProto: func(snapshot domain.RuntimeStatusSnapshot) (*controlv1.RuntimeStatusSnapshot, error) {
			return toProtoRuntimeStatusSnapshot(snapshot), nil
		},
		mapError: func(err error) error {
			return statusFromError("watch runtime status", err)
		},
		send: stream.Send,
	})
}

func (s *ControlService) WatchServerInitStatus(req *controlv1.WatchServerInitStatusRequest, stream controlv1.ControlPlaneService_WatchServerInitStatusServer) error {
	ctx := stream.Context()

	client := req.GetCaller()
	return guardedWatch(guardedWatchPlan[domain.ServerInitStatusSnapshot, *controlv1.ServerInitStatusSnapshot]{
		ctx:   ctx,
		guard: &s.guard,
		request: domain.GovernanceRequest{
			Method: "mcpv/server_init/watch",
			Caller: client,
		},
		op:       "watch server init status",
		lastETag: "",
		subscribe: func(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
			return s.control.WatchServerInitStatus(ctx, client)
		},
		etag: nil,
		toProto: func(snapshot domain.ServerInitStatusSnapshot) (*controlv1.ServerInitStatusSnapshot, error) {
			return toProtoServerInitStatusSnapshot(snapshot), nil
		},
		mapError: func(err error) error {
			return statusFromError("watch server init status", err)
		},
		send: stream.Send,
	})
}
