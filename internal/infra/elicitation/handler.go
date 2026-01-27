package elicitation

import (
	"context"

	"go.uber.org/zap"

	"mcpd/internal/domain"
)

// DefaultHandler returns a conservative response for elicitation requests.
type DefaultHandler struct {
	logger *zap.Logger
}

// NewDefaultHandler constructs a default elicitation handler.
func NewDefaultHandler(logger *zap.Logger) *DefaultHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DefaultHandler{logger: logger.Named("elicitation")}
}

// Elicit responds with a cancel action to avoid blocking unattended clients.
func (h *DefaultHandler) Elicit(ctx context.Context, params *domain.ElicitationRequest) (*domain.ElicitationResult, error) {
	if params == nil {
		return &domain.ElicitationResult{Action: "cancel"}, nil
	}
	h.logger.Debug("elicitation request received",
		zap.String("mode", params.Mode),
		zap.String("message", params.Message),
	)
	return &domain.ElicitationResult{
		Action: "cancel",
	}, nil
}
