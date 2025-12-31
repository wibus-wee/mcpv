package app

import (
	"context"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/subagent"
)

func initializeSubAgent(ctx context.Context, config domain.SubAgentConfig, controlPlane *ControlPlane, metrics domain.Metrics, logger *zap.Logger) (domain.SubAgent, error) {
	return subagent.NewEinoSubAgent(ctx, config, controlPlane, metrics, logger)
}
