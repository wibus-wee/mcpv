package rpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
)

func mapRPCError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return domain.E(domain.CodeCanceled, op, "request canceled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.E(domain.CodeDeadlineExceeded, op, "deadline exceeded", err)
	}
	if st, ok := status.FromError(err); ok {
		code := domainCodeFromGRPC(st.Code())
		msg := st.Message()
		return domain.E(code, op, msg, err)
	}
	return domain.Wrap(domain.CodeInternal, op, err)
}

func domainCodeFromGRPC(code codes.Code) domain.ErrorCode {
	switch code {
	case codes.OK:
		return domain.CodeInternal
	case codes.Unknown:
		return domain.CodeInternal
	case codes.InvalidArgument:
		return domain.CodeInvalidArgument
	case codes.AlreadyExists:
		return domain.CodeFailedPrecond
	case codes.NotFound:
		return domain.CodeNotFound
	case codes.ResourceExhausted:
		return domain.CodeUnavailable
	case codes.Aborted:
		return domain.CodeFailedPrecond
	case codes.OutOfRange:
		return domain.CodeInvalidArgument
	case codes.Unavailable:
		return domain.CodeUnavailable
	case codes.FailedPrecondition:
		return domain.CodeFailedPrecond
	case codes.PermissionDenied:
		return domain.CodePermissionDenied
	case codes.Unauthenticated:
		return domain.CodeUnauthenticated
	case codes.Canceled:
		return domain.CodeCanceled
	case codes.DeadlineExceeded:
		return domain.CodeDeadlineExceeded
	case codes.Unimplemented:
		return domain.CodeNotImplemented
	case codes.Internal:
		return domain.CodeInternal
	case codes.DataLoss:
		return domain.CodeInternal
	default:
		return domain.CodeInternal
	}
}
