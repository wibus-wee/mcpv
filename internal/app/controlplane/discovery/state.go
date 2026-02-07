package discovery

import (
	"go.uber.org/zap"

	"mcpv/internal/app/runtime"
	"mcpv/internal/domain"
)

type State interface {
	RuntimeState() *runtime.State
	ServerSpecKeys() map[string]string
	SpecRegistry() map[string]domain.ServerSpec
	Logger() *zap.Logger
}
