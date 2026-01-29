package controlplane

import (
	"context"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/subagent"
)

func InitializeSubAgent(ctx context.Context, config domain.SubAgentConfig, controlPlane *ControlPlane, metrics domain.Metrics, logger *zap.Logger) (domain.SubAgent, error) {
	return subagent.NewEinoSubAgent(ctx, config, controlPlane, metrics, logger)
}
