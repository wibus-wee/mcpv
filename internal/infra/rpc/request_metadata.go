package rpc

import (
	"context"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry"
)

func withRequestMetadata(ctx context.Context, req domain.GovernanceRequest) domain.GovernanceRequest {
	requestID, ok := telemetry.RequestIDFromContext(ctx)
	if !ok || requestID == "" {
		return req
	}
	if req.Metadata == nil {
		req.Metadata = map[string]string{
			telemetry.FieldRequestID: requestID,
		}
		return req
	}
	if _, exists := req.Metadata[telemetry.FieldRequestID]; exists {
		return req
	}
	req.Metadata[telemetry.FieldRequestID] = requestID
	return req
}
