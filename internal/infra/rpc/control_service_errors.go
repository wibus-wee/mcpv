package rpc

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"mcpv/internal/domain"
)

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
