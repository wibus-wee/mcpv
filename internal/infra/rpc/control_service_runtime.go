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
	return guardedWatch(
		ctx,
		&s.guard,
		domain.GovernanceRequest{
			Method: "mcpv/runtime/watch",
			Caller: client,
		},
		"watch runtime status",
		req.GetLastEtag(),
		func(ctx context.Context) (<-chan domain.RuntimeStatusSnapshot, error) {
			return s.control.WatchRuntimeStatus(ctx, client)
		},
		func(snapshot domain.RuntimeStatusSnapshot) string {
			return snapshot.ETag
		},
		func(last string, snapshot domain.RuntimeStatusSnapshot) bool {
			return last == snapshot.ETag
		},
		func(snapshot domain.RuntimeStatusSnapshot) (*controlv1.RuntimeStatusSnapshot, error) {
			return toProtoRuntimeStatusSnapshot(snapshot), nil
		},
		func(err error) error {
			return statusFromError("watch runtime status", err)
		},
		stream.Send,
	)
}

func (s *ControlService) WatchServerInitStatus(req *controlv1.WatchServerInitStatusRequest, stream controlv1.ControlPlaneService_WatchServerInitStatusServer) error {
	ctx := stream.Context()

	client := req.GetCaller()
	return guardedWatch(
		ctx,
		&s.guard,
		domain.GovernanceRequest{
			Method: "mcpv/server_init/watch",
			Caller: client,
		},
		"watch server init status",
		"",
		func(ctx context.Context) (<-chan domain.ServerInitStatusSnapshot, error) {
			return s.control.WatchServerInitStatus(ctx, client)
		},
		func(domain.ServerInitStatusSnapshot) string {
			return ""
		},
		func(string, domain.ServerInitStatusSnapshot) bool {
			return false
		},
		func(snapshot domain.ServerInitStatusSnapshot) (*controlv1.ServerInitStatusSnapshot, error) {
			return toProtoServerInitStatusSnapshot(snapshot), nil
		},
		func(err error) error {
			return statusFromError("watch server init status", err)
		},
		stream.Send,
	)
}
