package rpc

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"mcpv/internal/domain"
	"mcpv/internal/infra/governance"
)

type governanceGuard struct {
	executor *governance.Executor
}

func newGovernanceGuard(executor *governance.Executor) governanceGuard {
	return governanceGuard{executor: executor}
}

func (g *governanceGuard) applyRequest(ctx context.Context, req domain.GovernanceRequest, op string, mutate func([]byte) error) error {
	if g == nil || g.executor == nil {
		return nil
	}
	decision, err := g.executor.Request(ctx, req)
	if err != nil {
		return mapGovernanceError(err)
	}
	if !decision.Continue {
		return mapGovernanceDecision(decision)
	}
	if len(decision.RequestJSON) == 0 || mutate == nil {
		return nil
	}
	if err := mutate(decision.RequestJSON); err != nil {
		return status.Errorf(codes.InvalidArgument, "%s: invalid request mutation", op)
	}
	return nil
}

func (g *governanceGuard) applyProtoResponse(ctx context.Context, req domain.GovernanceRequest, op string, target proto.Message) error {
	if g == nil || g.executor == nil {
		return nil
	}
	if target == nil {
		return nil
	}
	raw, err := protojson.Marshal(target)
	if err != nil {
		return status.Errorf(codes.Internal, "%s: response encode failed: %v", op, err)
	}
	req.ResponseJSON = raw
	decision, err := g.executor.Response(ctx, req)
	if err != nil {
		return mapGovernanceError(err)
	}
	if !decision.Continue {
		return mapGovernanceDecision(decision)
	}
	if err := applyProtoMutation(target, decision.ResponseJSON); err != nil {
		return status.Errorf(codes.InvalidArgument, "%s: response mutation invalid: %v", op, err)
	}
	return nil
}

func applyProtoMutation(target proto.Message, raw []byte) error {
	if len(raw) == 0 || target == nil {
		return nil
	}
	return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(raw, target)
}

func mustMarshalJSON(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}
