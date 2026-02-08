package rpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"mcpv/internal/domain"
)

func guardedList[T any, P proto.Message](
	ctx context.Context,
	guard *governanceGuard,
	req domain.GovernanceRequest,
	respReq domain.GovernanceRequest,
	op string,
	mutate func([]byte) error,
	call func(context.Context) (T, error),
	toProto func(T) (P, string, error),
	mapErr func(error) error,
) (P, string, error) {
	var zero P
	if err := guard.applyRequest(ctx, req, op, mutate); err != nil {
		return zero, "", err
	}
	result, err := call(ctx)
	if err != nil {
		return zero, "", mapErr(err)
	}
	protoSnapshot, nextCursor, err := toProto(result)
	if err != nil {
		return zero, "", status.Errorf(codes.Internal, "%s: %v", op, err)
	}
	if err := guard.applyProtoResponse(ctx, respReq, op, protoSnapshot); err != nil {
		return zero, "", err
	}
	return protoSnapshot, nextCursor, nil
}

func guardedWatch[T any, P proto.Message](
	ctx context.Context,
	guard *governanceGuard,
	req domain.GovernanceRequest,
	op string,
	lastETag string,
	subscribe func(context.Context) (<-chan T, error),
	etag func(T) string,
	shouldSkip func(string, T) bool,
	toProto func(T) (P, error),
	mapErr func(error) error,
	send func(P) error,
) error {
	if err := guard.applyRequest(ctx, req, op, nil); err != nil {
		return err
	}
	updates, err := subscribe(ctx)
	if err != nil {
		return mapErr(err)
	}
	if shouldSkip == nil {
		shouldSkip = func(last string, snapshot T) bool {
			if last == "" {
				return false
			}
			return last == etag(snapshot)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case snapshot, ok := <-updates:
			if !ok {
				return nil
			}
			if shouldSkip(lastETag, snapshot) {
				continue
			}
			protoSnapshot, err := toProto(snapshot)
			if err != nil {
				return status.Errorf(codes.Internal, "%s: %v", op, err)
			}
			if err := guard.applyProtoResponse(ctx, req, op, protoSnapshot); err != nil {
				return err
			}
			if err := send(protoSnapshot); err != nil {
				return err
			}
			lastETag = etag(snapshot)
		}
	}
}
