package services

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"mcpv/internal/app"
	"mcpv/internal/ui"
)

// ServiceRegistry wires all Wails services together.
type ServiceRegistry struct {
	deps *ServiceDeps

	System      *SystemService
	Core        *CoreService
	Discovery   *DiscoveryService
	Log         *LogService
	Config      *ConfigService
	Server      *ServerService
	Runtime     *RuntimeService
	SubAgent    *SubAgentService
	Proxy       *ProxyService
	Debug       *DebugService
	Plugin      *PluginService
	McpTransfer *McpTransferService
}

func NewServiceRegistry(coreApp *app.App, logger *zap.Logger) *ServiceRegistry {
	deps := NewServiceDeps(coreApp, logger)
	return &ServiceRegistry{
		deps:        deps,
		System:      NewSystemService(deps),
		Core:        NewCoreService(deps),
		Discovery:   NewDiscoveryService(deps),
		Log:         NewLogService(deps),
		Config:      NewConfigService(deps),
		Server:      NewServerService(deps),
		Runtime:     NewRuntimeService(deps),
		SubAgent:    NewSubAgentService(deps),
		Proxy:       NewProxyService(deps),
		Debug:       NewDebugService(deps),
		Plugin:      NewPluginService(deps),
		McpTransfer: NewMcpTransferService(deps),
	}
}

func (r *ServiceRegistry) Services() []application.Service {
	return []application.Service{
		application.NewService(r.System),
		application.NewService(r.Core),
		application.NewService(r.Discovery),
		application.NewService(r.Log),
		application.NewService(r.Config),
		application.NewService(r.Server),
		application.NewService(r.Runtime),
		application.NewService(r.SubAgent),
		application.NewService(r.Proxy),
		application.NewService(r.Plugin),
		application.NewService(r.Debug),
		application.NewService(r.McpTransfer),
	}
}

func (r *ServiceRegistry) SetManager(manager *ui.Manager) {
	if r == nil || r.deps == nil {
		return
	}
	r.deps.setManager(manager)
}

func (r *ServiceRegistry) SetWailsApp(wails *application.App) {
	if r == nil || r.deps == nil {
		return
	}
	r.deps.setWailsApp(wails)
}
