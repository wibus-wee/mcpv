package rpc

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	"mcpv/internal/infra/scheduler"
)

func statusFromError(op string, err error) error {
	if err == nil {
		return nil
	}
	var protoErr *domain.ProtocolError
	if errors.As(err, &protoErr) {
		msg := protoErr.Message
		if op == "" {
			return status.Errorf(codes.FailedPrecondition, "request requires elicitation: %s", msg)
		}
		return status.Errorf(codes.FailedPrecondition, "%s requires elicitation: %s", op, msg)
	}
	var rej domain.GovernanceRejection
	if errors.As(err, &rej) {
		return mapGovernanceError(err)
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return status.Errorf(codes.DeadlineExceeded, "%s: deadline exceeded", op)
	case errors.Is(err, context.Canceled):
		return status.Errorf(codes.Canceled, "%s: canceled", op)
	case errors.Is(err, scheduler.ErrUnknownSpecKey):
		return status.Errorf(codes.InvalidArgument, "%s: %v", op, err)
	case errors.Is(err, scheduler.ErrNoCapacity), errors.Is(err, scheduler.ErrStickyBusy):
		return status.Errorf(codes.Unavailable, "%s: %v", op, err)
	case errors.Is(err, scheduler.ErrNotImplemented):
		return status.Errorf(codes.Unimplemented, "%s: %v", op, err)
	}

	if code, ok := domain.CodeFrom(err); ok {
		return statusFromCode(op, code, err)
	}
	return statusFromCode(op, domain.CodeInternal, err)
}

func statusFromCode(op string, code domain.ErrorCode, err error) error {
	grpcCode := grpcCodeFromDomain(code)
	msg := err.Error()
	if op != "" {
		msg = fmt.Sprintf("%s: %v", op, err)
	}
	return status.Error(grpcCode, msg)
}

func grpcCodeFromDomain(code domain.ErrorCode) codes.Code {
	switch code {
	case domain.CodeInvalidArgument:
		return codes.InvalidArgument
	case domain.CodeNotFound:
		return codes.NotFound
	case domain.CodeUnavailable:
		return codes.Unavailable
	case domain.CodeFailedPrecond:
		return codes.FailedPrecondition
	case domain.CodePermissionDenied:
		return codes.PermissionDenied
	case domain.CodeUnauthenticated:
		return codes.Unauthenticated
	case domain.CodeCanceled:
		return codes.Canceled
	case domain.CodeDeadlineExceeded:
		return codes.DeadlineExceeded
	case domain.CodeNotImplemented:
		return codes.Unimplemented
	case domain.CodeInternal:
		return codes.Internal
	default:
		return codes.Internal
	}
}
