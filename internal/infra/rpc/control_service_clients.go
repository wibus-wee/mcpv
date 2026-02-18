package rpc

import (
	"context"
	"time"

	"mcpv/internal/domain"
	controlv1 "mcpv/pkg/api/control/v1"
)

func (s *ControlService) ListActiveClients(ctx context.Context, _ *controlv1.ListActiveClientsRequest) (*controlv1.ListActiveClientsResponse, error) {
	clients, err := s.control.ListActiveClients(ctx)
	if err != nil {
		return nil, statusFromError("list active clients", err)
	}
	snapshot := domain.ActiveClientSnapshot{
		Clients:     clients,
		GeneratedAt: time.Now(),
	}
	return &controlv1.ListActiveClientsResponse{
		Snapshot: toProtoActiveClientsSnapshot(snapshot),
	}, nil
}

func (s *ControlService) WatchActiveClients(_ *controlv1.WatchActiveClientsRequest, stream controlv1.ControlPlaneService_WatchActiveClientsServer) error {
	ctx := stream.Context()
	updates, err := s.control.WatchActiveClients(ctx)
	if err != nil {
		return statusFromError("watch active clients", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case snapshot, ok := <-updates:
			if !ok {
				return nil
			}
			if err := stream.Send(toProtoActiveClientsSnapshot(snapshot)); err != nil {
				return err
			}
		}
	}
}
