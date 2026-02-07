package rpc

import (
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
	lastETag := req.GetLastEtag()

	client := req.GetCaller()
	if err := s.guard.applyRequest(ctx, domain.GovernanceRequest{
		Method: "mcpv/runtime/watch",
		Caller: client,
	}, "watch runtime status", nil); err != nil {
		return err
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
			if err := s.guard.applyProtoResponse(ctx, domain.GovernanceRequest{
				Method: "mcpv/runtime/watch",
				Caller: client,
			}, "watch runtime status", protoSnapshot); err != nil {
				return err
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
	if err := s.guard.applyRequest(ctx, domain.GovernanceRequest{
		Method: "mcpv/server_init/watch",
		Caller: client,
	}, "watch server init status", nil); err != nil {
		return err
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
			if err := s.guard.applyProtoResponse(ctx, domain.GovernanceRequest{
				Method: "mcpv/server_init/watch",
				Caller: client,
			}, "watch server init status", protoSnapshot); err != nil {
				return err
			}
			if err := stream.Send(protoSnapshot); err != nil {
				return err
			}
		}
	}
}
