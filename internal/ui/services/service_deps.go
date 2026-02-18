package services

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"mcpv/internal/app"
	"mcpv/internal/app/controlplane"
	catalogeditor "mcpv/internal/infra/catalog/editor"
	"mcpv/internal/infra/rpc"
	"mcpv/internal/ui"
	"mcpv/internal/ui/uiconfig"
)

// ServiceDeps holds shared dependencies for Wails services.
type ServiceDeps struct {
	mu sync.RWMutex

	coreApp    *app.App
	logger     *zap.Logger
	wails      *application.App
	managerRef *ui.Manager

	remoteMu          sync.Mutex
	remoteControl     *rpc.RemoteControlPlane
	remoteFingerprint string
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

	settings, err := d.coreConnectionSettings()
	if err == nil && settings.Mode == ui.CoreConnectionModeRemote {
		return d.getRemoteControlPlane(settings)
	}

	d.closeRemoteControlPlane()
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

func (d *ServiceDeps) uiSettingsStore() (*uiconfig.Store, error) {
	manager := d.manager()
	if manager == nil {
		return nil, ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	store, err := manager.UISettingsStore()
	if err != nil {
		return nil, ui.NewErrorWithDetails(ui.ErrCodeInternal, "Failed to open UI settings store", err.Error())
	}
	return store, nil
}

func (d *ServiceDeps) trayController() *ui.TrayController {
	manager := d.manager()
	if manager == nil {
		return nil
	}
	return manager.TrayController()
}

func (d *ServiceDeps) coreConnectionSettings() (ui.CoreConnectionSettings, error) {
	store, err := d.uiSettingsStore()
	if err != nil {
		return ui.DefaultCoreConnectionSettings(), err
	}
	snapshot, err := store.Get(uiconfig.ScopeGlobal, "")
	if err != nil {
		return ui.DefaultCoreConnectionSettings(), err
	}
	settings, err := ui.ParseCoreConnectionSettings(snapshot.Sections[ui.CoreConnectionSectionKey])
	if err != nil {
		return ui.DefaultCoreConnectionSettings(), err
	}
	return settings, nil
}

func (d *ServiceDeps) isRemoteMode() bool {
	settings, err := d.coreConnectionSettings()
	if err != nil {
		return false
	}
	return settings.Mode == ui.CoreConnectionModeRemote
}

func (d *ServiceDeps) getRemoteControlPlane(settings ui.CoreConnectionSettings) (*rpc.RemoteControlPlane, error) {
	if strings.TrimSpace(settings.Address) == "" {
		return nil, ui.NewError(ui.ErrCodeInvalidConfig, "Remote RPC address is required")
	}

	fingerprint := coreConnectionFingerprint(settings)

	d.remoteMu.Lock()
	if d.remoteControl != nil && d.remoteFingerprint == fingerprint {
		cp := d.remoteControl
		d.remoteMu.Unlock()
		return cp, nil
	}
	old := d.remoteControl
	d.remoteControl = nil
	d.remoteFingerprint = ""
	d.remoteMu.Unlock()

	if old != nil {
		_ = old.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	cp, err := rpc.NewRemoteControlPlane(ctx, rpc.RemoteControlPlaneConfig{
		ClientConfig: rpc.ClientConfig{
			Address:                 settings.Address,
			MaxRecvMsgSize:          settings.MaxRecvMsgSize,
			MaxSendMsgSize:          settings.MaxSendMsgSize,
			KeepaliveTimeSeconds:    settings.KeepaliveTimeSeconds,
			KeepaliveTimeoutSeconds: settings.KeepaliveTimeoutSeconds,
			TLS:                     settings.TLS,
			Auth:                    settings.Auth,
		},
		Caller: settings.Caller,
	}, d.loggerNamed("remote-control-plane"))
	if err != nil {
		return nil, ui.NewErrorWithDetails(ui.ErrCodeCoreFailed, "Failed to connect to remote core", err.Error())
	}

	d.remoteMu.Lock()
	d.remoteControl = cp
	d.remoteFingerprint = fingerprint
	d.remoteMu.Unlock()

	return cp, nil
}

func (d *ServiceDeps) closeRemoteControlPlane() {
	d.remoteMu.Lock()
	cp := d.remoteControl
	d.remoteControl = nil
	d.remoteFingerprint = ""
	d.remoteMu.Unlock()
	if cp != nil {
		_ = cp.Close()
	}
}

func (d *ServiceDeps) remoteControlPlane() (*rpc.RemoteControlPlane, error) {
	settings, err := d.coreConnectionSettings()
	if err != nil {
		return nil, err
	}
	if settings.Mode != ui.CoreConnectionModeRemote {
		return nil, ui.NewError(ui.ErrCodeInvalidState, "Remote mode is not enabled")
	}
	return d.getRemoteControlPlane(settings)
}

func coreConnectionFingerprint(settings ui.CoreConnectionSettings) string {
	payload := map[string]any{
		"mode":                    settings.Mode,
		"address":                 settings.Address,
		"caller":                  settings.Caller,
		"maxRecvMsgSize":          settings.MaxRecvMsgSize,
		"maxSendMsgSize":          settings.MaxSendMsgSize,
		"keepaliveTimeSeconds":    settings.KeepaliveTimeSeconds,
		"keepaliveTimeoutSeconds": settings.KeepaliveTimeoutSeconds,
		"tls": map[string]any{
			"enabled":  settings.TLS.Enabled,
			"certFile": settings.TLS.CertFile,
			"keyFile":  settings.TLS.KeyFile,
			"caFile":   settings.TLS.CAFile,
		},
		"auth": map[string]any{
			"enabled":  settings.Auth.Enabled,
			"mode":     settings.Auth.Mode,
			"token":    settings.Auth.Token,
			"tokenEnv": settings.Auth.TokenEnv,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}
