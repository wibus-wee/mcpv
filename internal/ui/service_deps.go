package ui

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"mcpd/internal/app"
	"mcpd/internal/domain"
	"mcpd/internal/infra/catalog"
)

const defaultUIClientName = "ui"

// ServiceDeps holds shared dependencies for Wails services.
type ServiceDeps struct {
	mu sync.RWMutex

	coreApp    *app.App
	logger     *zap.Logger
	wails      *application.App
	managerRef *Manager
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

func (d *ServiceDeps) setManager(manager *Manager) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.managerRef = manager
	d.mu.Unlock()
}

func (d *ServiceDeps) manager() *Manager {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.managerRef
}

func (d *ServiceDeps) getControlPlane() (domain.ControlPlane, error) {
	manager := d.manager()
	if manager == nil {
		return nil, NewUIError(ErrCodeInternal, "Manager not initialized")
	}
	return manager.GetControlPlane()
}

func (d *ServiceDeps) extractSpecKeyFromCache(toolName string) string {
	manager := d.manager()
	if manager == nil {
		return ""
	}
	snapshot := manager.GetSharedState().GetToolSnapshot()
	for _, tool := range snapshot.Tools {
		if tool.Name == toolName {
			return tool.SpecKey
		}
	}
	return ""
}

func (d *ServiceDeps) ensureUIClient(ctx context.Context) (string, error) {
	cp, err := d.getControlPlane()
	if err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	pid := os.Getpid()
	if pid <= 0 {
		pid = 1
	}
	_, err = cp.RegisterClient(ctx, defaultUIClientName, pid, nil)
	if err != nil {
		return "", MapDomainError(err)
	}
	return defaultUIClientName, nil
}

func (d *ServiceDeps) catalogEditor() (*catalog.Editor, error) {
	manager := d.manager()
	if manager == nil {
		return nil, NewUIError(ErrCodeInternal, "Manager not initialized")
	}
	path := strings.TrimSpace(manager.GetConfigPath())
	if path == "" {
		return nil, NewUIError(ErrCodeInvalidConfig, "Configuration path is not available")
	}
	return catalog.NewEditor(path, d.loggerNamed("catalog-editor")), nil
}
