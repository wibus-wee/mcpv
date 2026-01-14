package ui

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"mcpd/internal/app"
)

// ServiceRegistry wires all Wails services together.
type ServiceRegistry struct {
	deps *ServiceDeps

	System    *SystemService
	Core      *CoreService
	Discovery *DiscoveryService
	Log       *LogService
	Config    *ConfigService
	Profile   *ProfileService
	Runtime   *RuntimeService
	SubAgent  *SubAgentService
	Debug     *DebugService
}

func NewServiceRegistry(coreApp *app.App, logger *zap.Logger) *ServiceRegistry {
	deps := NewServiceDeps(coreApp, logger)
	return &ServiceRegistry{
		deps:      deps,
		System:    NewSystemService(deps),
		Core:      NewCoreService(deps),
		Discovery: NewDiscoveryService(deps),
		Log:       NewLogService(deps),
		Config:    NewConfigService(deps),
		Profile:   NewProfileService(deps),
		Runtime:   NewRuntimeService(deps),
		SubAgent:  NewSubAgentService(deps),
		Debug:     NewDebugService(deps),
	}
}

func (r *ServiceRegistry) Services() []application.Service {
	return []application.Service{
		application.NewService(r.System),
		application.NewService(r.Core),
		application.NewService(r.Discovery),
		application.NewService(r.Log),
		application.NewService(r.Config),
		application.NewService(r.Profile),
		application.NewService(r.Runtime),
		application.NewService(r.SubAgent),
		application.NewService(r.Debug),
	}
}

func (r *ServiceRegistry) SetManager(manager *Manager) {
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
