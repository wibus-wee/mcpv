package automation

import (
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

type State interface {
	Runtime() domain.RuntimeConfig
	Logger() *zap.Logger
}
