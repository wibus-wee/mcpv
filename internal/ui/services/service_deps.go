package services

import (
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"mcpv/internal/app"
	"mcpv/internal/app/controlplane"
	catalogeditor "mcpv/internal/infra/catalog/editor"
	"mcpv/internal/ui"
)

// ServiceDeps holds shared dependencies for Wails services.
type ServiceDeps struct {
	mu sync.RWMutex

	coreApp    *app.App
	logger     *zap.Logger
	wails      *application.App
	managerRef *ui.Manager
}

func NewServiceDeps(coreApp *app.App, logger *zap.Logger) *ServiceDeps {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ServiceDeps{
		coreApp: coreApp,
		logger:  logger,
	}
}

func (d *ServiceDeps) loggerNamed(name string) *zap.Logger {
	if d == nil || d.logger == nil {
		return zap.NewNop()
	}
	if strings.TrimSpace(name) == "" {
		return d.logger
	}
	return d.logger.Named(name)
}

func (d *ServiceDeps) setWailsApp(wails *application.App) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.wails = wails
	d.mu.Unlock()
}

func (d *ServiceDeps) wailsApp() *application.App {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.wails
}

func (d *ServiceDeps) setManager(manager *ui.Manager) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.managerRef = manager
	d.mu.Unlock()
}

func (d *ServiceDeps) manager() *ui.Manager {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.managerRef
}

func (d *ServiceDeps) getControlPlane() (controlplane.API, error) {
	manager := d.manager()
	if manager == nil {
		return nil, ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	return manager.GetControlPlane()
}

func (d *ServiceDeps) catalogEditor() (*catalogeditor.Editor, error) {
	manager := d.manager()
	if manager == nil {
		return nil, ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	path := strings.TrimSpace(manager.GetConfigPath())
	if path == "" {
		return nil, ui.NewError(ui.ErrCodeInvalidConfig, "Configuration path is not available")
	}
	return catalogeditor.NewEditor(path, d.loggerNamed("catalog-editor")), nil
}

func (d *ServiceDeps) getCoreApp() (*app.App, error) {
	manager := d.manager()
	if manager == nil {
		return nil, ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	coreApp := manager.GetCoreApp()
	if coreApp == nil {
		return nil, ui.NewError(ui.ErrCodeInternal, "Core app not initialized")
	}
	return coreApp, nil
}

func (d *ServiceDeps) updateChecker() (*ui.UpdateChecker, error) {
	manager := d.manager()
	if manager == nil {
		return nil, ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	checker := manager.UpdateChecker()
	if checker == nil {
		return nil, ui.NewError(ui.ErrCodeInternal, "Update checker not initialized")
	}
	return checker, nil
}
