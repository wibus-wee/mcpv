package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
	"mcpv/internal/infra/scheduler"
)

func mapCallToolError(name string, err error) error {
	var protoErr *domain.ProtocolError
	if errors.As(err, &protoErr) {
		return status.Errorf(codes.FailedPrecondition, "call tool requires elicitation: %s", protoErr.Message)
	}
	switch {
	case errors.Is(err, domain.ErrToolNotFound):
		return status.Errorf(codes.NotFound, "tool not found: %s", name)
	case errors.Is(err, domain.ErrInvalidRequest), errors.Is(err, domain.ErrMethodNotAllowed):
		return status.Errorf(codes.InvalidArgument, "call tool: %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		return status.Errorf(codes.DeadlineExceeded, "call tool deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Errorf(codes.Canceled, "call tool canceled")
	case errors.Is(err, scheduler.ErrUnknownSpecKey):
		return status.Errorf(codes.InvalidArgument, "call tool: %v", err)
	case errors.Is(err, scheduler.ErrNoCapacity), errors.Is(err, scheduler.ErrStickyBusy):
		return status.Errorf(codes.Unavailable, "call tool unavailable: %v", err)
	case errors.Is(err, domain.ErrClientNotRegistered):
		return status.Error(codes.FailedPrecondition, "client not registered")
	default:
		return status.Errorf(codes.Unavailable, "call tool: %v", fmt.Sprintf("%T: %v", err, err))
	}
}

func mapReadResourceError(uri string, err error) error {
	switch {
	case errors.Is(err, domain.ErrResourceNotFound):
		return status.Errorf(codes.NotFound, "resource not found: %s", uri)
	case errors.Is(err, domain.ErrInvalidRequest), errors.Is(err, domain.ErrMethodNotAllowed):
		return status.Errorf(codes.InvalidArgument, "read resource: %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		return status.Errorf(codes.DeadlineExceeded, "read resource deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Errorf(codes.Canceled, "read resource canceled")
	case errors.Is(err, scheduler.ErrUnknownSpecKey):
		return status.Errorf(codes.InvalidArgument, "read resource: %v", err)
	case errors.Is(err, scheduler.ErrNoCapacity), errors.Is(err, scheduler.ErrStickyBusy):
		return status.Errorf(codes.Unavailable, "read resource unavailable: %v", err)
	case errors.Is(err, domain.ErrClientNotRegistered):
		return status.Error(codes.FailedPrecondition, "client not registered")
	default:
		return status.Errorf(codes.Unavailable, "read resource: %v", fmt.Sprintf("%T: %v", err, err))
	}
}

func mapGetPromptError(name string, err error) error {
	switch {
	case errors.Is(err, domain.ErrPromptNotFound):
		return status.Errorf(codes.NotFound, "prompt not found: %s", name)
	case errors.Is(err, domain.ErrInvalidRequest), errors.Is(err, domain.ErrMethodNotAllowed):
		return status.Errorf(codes.InvalidArgument, "get prompt: %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		return status.Errorf(codes.DeadlineExceeded, "get prompt deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Errorf(codes.Canceled, "get prompt canceled")
	case errors.Is(err, scheduler.ErrUnknownSpecKey):
		return status.Errorf(codes.InvalidArgument, "get prompt: %v", err)
	case errors.Is(err, scheduler.ErrNoCapacity), errors.Is(err, scheduler.ErrStickyBusy):
		return status.Errorf(codes.Unavailable, "get prompt unavailable: %v", err)
	case errors.Is(err, domain.ErrClientNotRegistered):
		return status.Error(codes.FailedPrecondition, "client not registered")
	default:
		return status.Errorf(codes.Unavailable, "get prompt: %v", fmt.Sprintf("%T: %v", err, err))
	}
}

func mapGovernanceError(err error) error {
	if err == nil {
		return nil
	}
	var rej domain.GovernanceRejection
	if errors.As(err, &rej) {
		return mapGovernanceRejection(rej.Code, rej.Message, rej.Category, rej.Plugin)
	}
	return status.Errorf(codes.Unavailable, "governance failure: %v", err)
}

func mapGovernanceDecision(decision domain.GovernanceDecision) error {
	if decision.Continue {
		return nil
	}
	return mapGovernanceRejection(decision.RejectCode, decision.RejectMessage, decision.Category, decision.Plugin)
}

func mapGovernanceRejection(code, message string, category domain.PluginCategory, pluginName string) error {
	grpcCode := governanceCode(code)
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "request rejected"
	}
	if category != "" {
		if pluginName != "" {
			msg = fmt.Sprintf("governance rejected by %s/%s: %s", category, pluginName, msg)
		} else {
			msg = fmt.Sprintf("governance rejected by %s: %s", category, msg)
		}
	}
	return status.Error(grpcCode, msg)
}

func governanceCode(code string) codes.Code {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "unauthenticated":
		return codes.Unauthenticated
	case "unauthorized":
		return codes.PermissionDenied
	case "rate_limited":
		return codes.ResourceExhausted
	case "invalid_request":
		return codes.InvalidArgument
	default:
		return codes.PermissionDenied
	}
}

func mapListError(op string, err error) error {
	if errors.Is(err, domain.ErrClientNotRegistered) {
		return status.Errorf(codes.FailedPrecondition, "%s: client not registered", op)
	}
	if errors.Is(err, domain.ErrInvalidCursor) {
		return status.Errorf(codes.InvalidArgument, "%s: invalid cursor", op)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

func mapTaskError(op string, err error) error {
	if errors.Is(err, domain.ErrTaskNotFound) {
		return status.Errorf(codes.NotFound, "%s: task not found", op)
	}
	if errors.Is(err, domain.ErrTasksNotImplemented) {
		return status.Errorf(codes.Unimplemented, "%s: tasks not implemented", op)
	}
	if errors.Is(err, domain.ErrInvalidCursor) {
		return status.Errorf(codes.InvalidArgument, "%s: invalid cursor", op)
	}
	if errors.Is(err, domain.ErrClientNotRegistered) {
		return status.Errorf(codes.FailedPrecondition, "%s: client not registered", op)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Errorf(codes.DeadlineExceeded, "%s: deadline exceeded", op)
	}
	if errors.Is(err, context.Canceled) {
		return status.Errorf(codes.Canceled, "%s: canceled", op)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}

func mapClientError(op string, err error) error {
	if errors.Is(err, domain.ErrClientNotRegistered) {
		return status.Errorf(codes.FailedPrecondition, "%s: client not registered", op)
	}
	return status.Errorf(codes.Internal, "%s: %v", op, err)
}
