package rpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"mcpv/internal/domain"
)

type guardedListPlan[T any, P proto.Message] struct {
	ctx             context.Context
	guard           *governanceGuard
	request         domain.GovernanceRequest
	responseRequest domain.GovernanceRequest
	op              string
	mutate          func([]byte) error
	call            func(context.Context) (T, error)
	toProto         func(T) (P, string, error)
	mapError        func(error) error
}

func guardedList[T any, P proto.Message](plan guardedListPlan[T, P]) (P, string, error) {
	var zero P
	if err := plan.guard.applyRequest(plan.ctx, plan.request, plan.op, plan.mutate); err != nil {
		return zero, "", err
	}
	result, err := plan.call(plan.ctx)
	if err != nil {
		return zero, "", plan.mapError(err)
	}
	protoSnapshot, nextCursor, err := plan.toProto(result)
	if err != nil {
		return zero, "", status.Errorf(codes.Internal, "%s: %v", plan.op, err)
	}
	if err := plan.guard.applyProtoResponse(plan.ctx, plan.responseRequest, plan.op, protoSnapshot); err != nil {
		return zero, "", err
	}
	return protoSnapshot, nextCursor, nil
}

type guardedWatchPlan[T any, P proto.Message] struct {
	ctx        context.Context
	guard      *governanceGuard
	request    domain.GovernanceRequest
	op         string
	lastETag   string
	subscribe  func(context.Context) (<-chan T, error)
	etag       func(T) string
	shouldSkip func(string, T) bool
	toProto    func(T) (P, error)
	mapError   func(error) error
	send       func(P) error
}

func guardedWatch[T any, P proto.Message](plan guardedWatchPlan[T, P]) error {
	if err := plan.guard.applyRequest(plan.ctx, plan.request, plan.op, nil); err != nil {
		return err
	}
	updates, err := plan.subscribe(plan.ctx)
	if err != nil {
		return plan.mapError(err)
	}
	if plan.shouldSkip == nil {
		plan.shouldSkip = func(last string, snapshot T) bool {
			if last == "" || plan.etag == nil {
				return false
			}
			return last == plan.etag(snapshot)
		}
	}

	for {
		select {
		case <-plan.ctx.Done():
			return plan.ctx.Err()
		case snapshot, ok := <-updates:
			if !ok {
				return nil
			}
			if plan.shouldSkip(plan.lastETag, snapshot) {
				continue
			}
			protoSnapshot, err := plan.toProto(snapshot)
			if err != nil {
				return status.Errorf(codes.Internal, "%s: %v", plan.op, err)
			}
			if err := plan.guard.applyProtoResponse(plan.ctx, plan.request, plan.op, protoSnapshot); err != nil {
				return err
			}
			if err := plan.send(protoSnapshot); err != nil {
				return err
			}
			if plan.etag != nil {
				plan.lastETag = plan.etag(snapshot)
			}
		}
	}
}
