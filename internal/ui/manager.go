package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"mcpv/internal/app"
	"mcpv/internal/app/controlplane"
	"mcpv/internal/ui/events"
	"mcpv/internal/ui/types"
	"mcpv/internal/ui/uiconfig"
)

// CoreState represents the lifecycle state of the Core.
type CoreState string

const (
	CoreStateStopped  CoreState = "stopped"
	CoreStateStarting CoreState = "starting"
	CoreStateRunning  CoreState = "running"
	CoreStateStopping CoreState = "stopping"
	CoreStateError    CoreState = "error"
)

const (
	gatewayShutdownTimeout       = 300 * time.Millisecond
	updateCheckerShutdownTimeout = 200 * time.Millisecond
)

// Manager coordinates Core lifecycle and all UI services.
type Manager struct {
	mu sync.RWMutex

	// Wails application reference
	wails *application.App

	// Logger
	logger *zap.Logger

	// Core application and control plane
	coreApp           *app.App
	controlPlane      controlplane.API
	configPath        string
	lastObservability *app.ObservabilityOptions

	// Shared state
	state *SharedState

	// Lifecycle tracking
	coreState      CoreState
	coreCtx        context.Context
	coreCancel     context.CancelFunc
	coreStarted    time.Time
	coreError      error
	watchersCancel context.CancelFunc

	// Update checker
	updateChecker *UpdateChecker

	// Tray controller
	trayController *TrayController

	// Gateway process
	gateway *GatewayProcess

	// UI settings store
	uiSettings *uiconfig.Store
}

// NewManager creates a new Manager instance.
func NewManager(wails *application.App, coreApp *app.App, configPath string) *Manager {
	logger := zap.NewNop()
	defaultCfg := BuildGatewayProcessConfig(DefaultGatewaySettings())
	return &Manager{
		wails:      wails,
		coreApp:    coreApp,
		configPath: configPath,
		state:      NewSharedState(),
		coreState:  CoreStateStopped,
		logger:     logger,
		gateway:    NewGatewayProcess(logger.Named("gateway-process"), defaultCfg),
	}
}

// Start starts the Core and auto-starts Watch subscriptions.
func (m *Manager) Start(ctx context.Context) error {
	return m.startWithConfig(ctx, m.configPath, m.lastObservability)
}

// StartWithOptions starts Core with explicit configuration overrides.
func (m *Manager) StartWithOptions(ctx context.Context, opts types.StartCoreOptions) error {
	configPath, observability := resolveStartOptions(opts, m.configPath)
	return m.startWithConfig(ctx, configPath, observability)
}

func (m *Manager) startWithConfig(ctx context.Context, configPath string, observability *app.ObservabilityOptions) error {
	m.mu.RLock()
	if m.coreState == CoreStateRunning || m.coreState == CoreStateStarting {
		m.mu.RUnlock()
		return NewError(ErrCodeCoreAlreadyRunning, "Core is already running or starting")
	}
	m.mu.RUnlock()

	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return NewError(ErrCodeInvalidConfig, "Config path is required")
	}
	if err := ensureConfigFile(configPath); err != nil {
		return NewErrorWithDetails(ErrCodeInvalidConfig, "Failed to prepare config file", err.Error())
	}

	m.mu.Lock()
	if m.coreState == CoreStateRunning || m.coreState == CoreStateStarting {
		m.mu.Unlock()
		return NewError(ErrCodeCoreAlreadyRunning, "Core is already running or starting")
	}

	m.configPath = configPath
	m.lastObservability = observability

	// Transition to starting state
	m.coreState = CoreStateStarting
	m.coreError = nil
	wails := m.wails

	// Create context for Core lifecycle. Detach from request-scoped context to avoid early cancellation.
	parent := context.Background()
	if ctx != nil {
		parent = context.WithoutCancel(ctx)
	}
	m.coreCtx, m.coreCancel = context.WithCancel(parent)

	cfg := app.ServeConfig{
		ConfigPath:    configPath,
		OnReady:       m.handleControlPlaneReady,
		Observability: observability,
	}

	m.mu.Unlock()

	events.EmitCoreState(wails, string(CoreStateStarting), nil)

	// Start Core in background
	go m.runCore(cfg)

	return nil
}

func resolveStartOptions(opts types.StartCoreOptions, fallback string) (string, *app.ObservabilityOptions) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	configPath := strings.TrimSpace(opts.ConfigPath)
	if configPath == "" {
		switch {
		case fallback != "":
			configPath = fallback
		default:
			configPath = defaultConfigPath()
		}
	}
	// if configPath == "" {
	// 	switch mode {
	// 	case "dev":
	// 		configPath = "./dev"
	// 	case "prod":
	// 		configPath = "."
	// 	default:
	// 		configPath = fallback
	// 	}
	// }

	metricsEnabled := opts.MetricsEnabled
	healthzEnabled := opts.HealthzEnabled
	if mode != "" {
		if metricsEnabled == nil {
			metricsEnabled = boolPtr(mode == "dev")
		}
		if healthzEnabled == nil {
			healthzEnabled = boolPtr(mode == "dev")
		}
	}
	if metricsEnabled == nil && healthzEnabled == nil {
		return configPath, nil
	}
	return configPath, &app.ObservabilityOptions{
		MetricsEnabled: metricsEnabled,
		HealthzEnabled: healthzEnabled,
	}
}

func boolPtr(value bool) *bool {
	return &value
}

// runCore executes the Core's Serve method.
func (m *Manager) runCore(cfg app.ServeConfig) {
	m.mu.Lock()
	m.coreStarted = time.Now()
	m.mu.Unlock()
	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("core panic: %v", recovered)
			var wails *application.App
			m.mu.Lock()
			m.coreState = CoreStateError
			m.coreError = err
			wails = m.wails
			m.coreCancel = nil
			m.coreCtx = nil
			m.controlPlane = nil
			m.mu.Unlock()

			events.EmitCoreState(wails, string(CoreStateError), err)
			events.EmitError(wails, ErrCodeCoreFailed, "Core panic", err.Error())
		}
	}()

	err := m.coreApp.Serve(m.coreCtx, cfg)

	var (
		wails     *application.App
		emitState CoreState
		emitErr   error
	)
	m.mu.Lock()
	wails = m.wails

	// Check if this was a clean shutdown or an error.
	if err != nil && m.coreCtx != nil && m.coreCtx.Err() == nil {
		m.coreState = CoreStateError
		m.coreError = err
		emitState = CoreStateError
		emitErr = err
	} else {
		m.coreState = CoreStateStopped
		m.coreError = nil
		emitState = CoreStateStopped
	}

	// Cleanup
	m.coreCancel = nil
	m.coreCtx = nil
	m.controlPlane = nil
	m.mu.Unlock()

	if emitState == CoreStateError {
		events.EmitCoreState(wails, string(CoreStateError), emitErr)
		events.EmitError(wails, ErrCodeCoreFailed, "Core failed", emitErr.Error())
	} else {
		events.EmitCoreState(wails, string(CoreStateStopped), nil)
	}

	m.stopGateway()
}

func (m *Manager) handleControlPlaneReady(cp controlplane.API) {
	m.SetControlPlane(cp)

	if cp == nil {
		return
	}

	m.onCoreReady()
}

// onCoreReady is called when Core reaches running state.
func (m *Manager) onCoreReady() {
	m.mu.Lock()
	if m.coreState == CoreStateRunning {
		m.mu.Unlock()
		return
	}
	m.coreState = CoreStateRunning
	uptime := time.Since(m.coreStarted).Milliseconds()
	wails := m.wails
	m.mu.Unlock()

	// Emit running state
	event := events.CoreStateEvent{
		State:  string(CoreStateRunning),
		Uptime: uptime,
	}
	if wails != nil {
		wails.Event.Emit(events.EventCoreState, event)
	}

	// Auto-start Watch subscriptions
	m.startWatchers()
	m.startGateway()
}

// startWatchers automatically starts all Watch subscriptions.
func (m *Manager) startWatchers() {
	m.mu.Lock()
	if m.watchersCancel != nil {
		m.mu.Unlock()
		return
	}
	cp := m.controlPlane
	wails := m.wails
	if cp == nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.watchersCancel = cancel
	m.mu.Unlock()

	// Watch runtime status
	go func() {
		updates, err := cp.WatchRuntimeStatusAllServers(ctx)
		if err != nil {
			events.EmitError(wails, ErrCodeInternal, "Failed to start runtime status watcher", err.Error())
			return
		}
		for snapshot := range updates {
			events.EmitRuntimeStatusUpdated(wails, snapshot)
		}
	}()

	// Watch server init status
	go func() {
		updates, err := cp.WatchServerInitStatusAllServers(ctx)
		if err != nil {
			events.EmitError(wails, ErrCodeInternal, "Failed to start server init status watcher", err.Error())
			return
		}
		for snapshot := range updates {
			events.EmitServerInitUpdated(wails, snapshot)
		}
	}()

	// Watch active clients
	go func() {
		updates, err := cp.WatchActiveClients(ctx)
		if err != nil {
			events.EmitError(wails, ErrCodeInternal, "Failed to start active clients watcher", err.Error())
			return
		}
		for snapshot := range updates {
			events.EmitActiveClientsUpdated(wails, snapshot)
		}
	}()
}

// Stop stops the Core gracefully.
func (m *Manager) Stop() error {
	m.mu.Lock()
	if m.coreState != CoreStateRunning {
		m.mu.Unlock()
		return NewError(ErrCodeCoreNotRunning, "Core is not running")
	}

	m.coreState = CoreStateStopping
	wails := m.wails
	watchersCancel := m.watchersCancel
	coreCancel := m.coreCancel
	m.watchersCancel = nil
	m.mu.Unlock()

	m.stopGateway()

	events.EmitCoreState(wails, string(CoreStateStopping), nil)

	// Cancel all active watchers
	m.state.CancelAllWatches()
	if watchersCancel != nil {
		watchersCancel()
	}

	// Cancel Core context to trigger graceful shutdown
	if coreCancel != nil {
		coreCancel()
	}

	return nil
}

// Restart restarts the Core.
func (m *Manager) Restart(ctx context.Context) error {
	// Stop if running
	if err := m.Stop(); err != nil {
		var uiErr *Error
		if !errors.As(err, &uiErr) || uiErr.Code != ErrCodeCoreNotRunning {
			return err
		}
	}

	// Wait for Core to actually stop (with timeout)
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return NewError(ErrCodeInternal, "Timeout waiting for Core to stop")
		case <-ticker.C:
			m.mu.RLock()
			state := m.coreState
			m.mu.RUnlock()

			if state == CoreStateStopped || state == CoreStateError {
				// Core has stopped, now start it
				return m.startWithConfig(ctx, m.configPath, m.lastObservability)
			}
		}
	}
}

// Shutdown performs cleanup on application exit.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	logger := m.logger
	updateChecker := m.updateChecker
	trayController := m.trayController
	uiSettings := m.uiSettings
	gateway := m.gateway
	m.uiSettings = nil

	// Cancel all watchers
	m.state.CancelAllWatches()
	if m.watchersCancel != nil {
		m.watchersCancel()
		m.watchersCancel = nil
	}

	// Stop Core if running
	if m.coreState == CoreStateRunning && m.coreCancel != nil {
		m.coreCancel()
	}
	m.mu.Unlock()

	if gateway != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), gatewayShutdownTimeout)
			err := gateway.Stop(ctx)
			cancel()
			if err != nil && logger != nil {
				logger.Warn("gateway stop failed", zap.Error(err))
			}
		}()
	}
	if trayController != nil {
		trayController.Shutdown()
	}
	if uiSettings != nil {
		_ = uiSettings.Close()
	}
	if updateChecker != nil {
		go func() {
			if err := updateChecker.StopWithTimeout(updateCheckerShutdownTimeout); err != nil && logger != nil {
				logger.Warn("update checker stop timeout", zap.Error(err))
			}
		}()
	}
}

// GetState returns current Core state information.
func (m *Manager) GetState() (CoreState, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var uptime int64
	if m.coreState == CoreStateRunning && !m.coreStarted.IsZero() {
		uptime = time.Since(m.coreStarted).Milliseconds()
	}

	return m.coreState, uptime, m.coreError
}

// GetControlPlane returns the ControlPlane API from Core.
// Returns error if Core is not running.
func (m *Manager) GetControlPlane() (controlplane.API, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.coreState != CoreStateRunning {
		return nil, NewError(ErrCodeCoreNotRunning, "Core is not running")
	}

	if m.controlPlane == nil {
		return nil, NewError(ErrCodeInternal, "ControlPlane not initialized")
	}

	return m.controlPlane, nil
}

// SetControlPlane sets the ControlPlane instance.
// This should be called after Core successfully starts.
func (m *Manager) SetControlPlane(cp controlplane.API) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.controlPlane = cp
}

// NotifyCoreReady should be called by external code when Core signals it's ready
// This is a callback hook for the actual readiness detection.
func (m *Manager) NotifyCoreReady() {
	m.onCoreReady()
}

// GetSharedState returns the shared state instance.
func (m *Manager) GetSharedState() *SharedState {
	return m.state
}

// SetWailsApp sets the Wails application instance
// This allows setting the app after Manager creation (for dependency injection).
func (m *Manager) SetWailsApp(wails *application.App) {
	m.mu.Lock()
	updateChecker := m.updateChecker
	defer m.mu.Unlock()
	m.wails = wails
	if updateChecker != nil {
		updateChecker.SetWailsApp(wails)
	}
}

// GetConfigPath returns the configuration path.
func (m *Manager) GetConfigPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configPath
}

// GetCoreApp returns the core application instance.
func (m *Manager) GetCoreApp() *app.App {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.coreApp
}

// SetLogger updates the manager logger and dependent components.
func (m *Manager) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	m.mu.Lock()
	m.logger = logger
	gateway := m.gateway
	m.mu.Unlock()
	if gateway != nil {
		gateway.SetLogger(logger.Named("gateway-process"))
	}
}

// SetUpdateChecker wires the update checker into the manager.
func (m *Manager) SetUpdateChecker(checker *UpdateChecker) {
	m.mu.Lock()
	m.updateChecker = checker
	wails := m.wails
	m.mu.Unlock()

	if checker != nil && wails != nil {
		checker.SetWailsApp(wails)
	}
	if checker != nil {
		settings := m.loadUpdateSettings()
		checker.SetOptions(settings.ToUpdateCheckOptions())
	}
}

// UpdateChecker returns the update checker if configured.
func (m *Manager) UpdateChecker() *UpdateChecker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.updateChecker
}

// SetTrayController wires the tray controller into the manager.
func (m *Manager) SetTrayController(controller *TrayController) {
	m.mu.Lock()
	m.trayController = controller
	m.mu.Unlock()
}

// TrayController returns the tray controller if configured.
func (m *Manager) TrayController() *TrayController {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.trayController
}

func (m *Manager) startGateway() {
	m.mu.RLock()
	gateway := m.gateway
	logger := m.logger
	wails := m.wails
	m.mu.RUnlock()

	if gateway == nil {
		return
	}
	settings := m.loadGatewaySettings()
	if err := m.applyGatewaySettings(context.Background(), settings); err != nil {
		if logger != nil {
			logger.Warn("gateway start failed", zap.Error(err))
		}
		events.EmitError(wails, ErrCodeInternal, "Gateway failed to start", err.Error())
	}
}

func (m *Manager) stopGateway() {
	m.mu.RLock()
	gateway := m.gateway
	logger := m.logger
	m.mu.RUnlock()

	if gateway == nil {
		return
	}
	if err := gateway.Stop(context.Background()); err != nil && logger != nil {
		logger.Warn("gateway stop failed", zap.Error(err))
	}
}

func (m *Manager) ApplyGatewaySettings(settings GatewaySettings) error {
	return m.applyGatewaySettings(context.Background(), settings)
}

func (m *Manager) applyGatewaySettings(ctx context.Context, settings GatewaySettings) error {
	m.mu.RLock()
	gateway := m.gateway
	logger := m.logger
	coreState := m.coreState
	m.mu.RUnlock()
	if gateway == nil {
		return nil
	}

	cfg := BuildGatewayProcessConfig(settings)
	prev := gateway.Config()
	gateway.UpdateConfig(cfg)

	if coreState != CoreStateRunning {
		return gateway.Stop(ctx)
	}
	if !cfg.Enabled {
		return gateway.Stop(ctx)
	}
	if !gatewayConfigEqual(prev, cfg) {
		if err := gateway.Stop(ctx); err != nil && logger != nil {
			logger.Warn("gateway restart stop failed", zap.Error(err))
		}
	}
	return gateway.Start(ctx)
}

func (m *Manager) loadGatewaySettings() GatewaySettings {
	store, err := m.UISettingsStore()
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to open ui settings store", zap.Error(err))
		}
		return DefaultGatewaySettings()
	}
	snapshot, err := store.Get(uiconfig.ScopeGlobal, "")
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to read ui settings", zap.Error(err))
		}
		return DefaultGatewaySettings()
	}
	raw := snapshot.Sections[GatewaySectionKey]
	settings, err := ParseGatewaySettings(raw)
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to parse gateway settings", zap.Error(err))
		}
		return DefaultGatewaySettings()
	}
	return settings
}

func (m *Manager) loadUpdateSettings() UpdateSettings {
	store, err := m.UISettingsStore()
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to open ui settings store", zap.Error(err))
		}
		return DefaultUpdateSettings()
	}
	snapshot, err := store.Get(uiconfig.ScopeGlobal, "")
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to read ui settings", zap.Error(err))
		}
		return DefaultUpdateSettings()
	}
	raw := snapshot.Sections[UpdateSectionKey]
	settings, err := ParseUpdateSettings(raw)
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to parse update settings", zap.Error(err))
		}
		return DefaultUpdateSettings()
	}
	return settings
}

func gatewayConfigEqual(a, b GatewayProcessConfig) bool {
	if a.Enabled != b.Enabled ||
		a.BinaryPath != b.BinaryPath ||
		a.HealthURL != b.HealthURL ||
		a.HealthTimeout != b.HealthTimeout ||
		a.StopTimeout != b.StopTimeout {
		return false
	}
	if !stringSliceEqual(a.Args, b.Args) {
		return false
	}
	if !stringSliceEqual(a.Env, b.Env) {
		return false
	}
	return true
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// UISettingsStore returns the UI settings store, initializing it on demand.
func (m *Manager) UISettingsStore() (*uiconfig.Store, error) {
	if m == nil {
		return nil, errors.New("manager is nil")
	}
	m.mu.RLock()
	store := m.uiSettings
	m.mu.RUnlock()
	if store != nil {
		return store, nil
	}

	path := uiconfig.ResolveDefaultPath()
	opened, err := uiconfig.OpenStore(path)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if m.uiSettings == nil {
		m.uiSettings = opened
		m.mu.Unlock()
		return opened, nil
	}
	existing := m.uiSettings
	m.mu.Unlock()
	_ = opened.Close()
	return existing, nil
}

func (m *Manager) ReloadConfig(ctx context.Context) error {
	m.mu.RLock()
	state := m.coreState
	coreApp := m.coreApp
	m.mu.RUnlock()

	if state != CoreStateRunning {
		return NewError(ErrCodeCoreNotRunning, "Core is not running")
	}
	if coreApp == nil {
		return NewError(ErrCodeInternal, "Core app not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := coreApp.ReloadConfig(ctx); err != nil {
		return NewErrorWithDetails(ErrCodeInvalidConfig, "Configuration reload failed", err.Error())
	}
	return nil
}

// HandleDeepLink handles a custom URL protocol invocation.
func (m *Manager) HandleDeepLink(rawURL string) error {
	link, err := ParseDeepLink(rawURL)
	if err != nil {
		return NewErrorWithDetails(ErrCodeInvalidRequest, "Invalid deep link", err.Error())
	}

	m.mu.RLock()
	wails := m.wails
	m.mu.RUnlock()

	events.EmitDeepLink(wails, link.Path(), link.Params())
	return nil
}
