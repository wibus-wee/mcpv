package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"mcpd/internal/app"
	"mcpd/internal/domain"
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

// Manager coordinates Core lifecycle and all UI services.
type Manager struct {
	mu sync.RWMutex

	// Wails application reference
	wails *application.App

	// Core application and control plane
	coreApp           *app.App
	controlPlane      domain.ControlPlane
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

	// Service references (will be set in Phase 3)
	// toolService     *ToolService
	// resourceService *ResourceService
	// promptService   *PromptService
	// logService      *LogService
}

// NewManager creates a new Manager instance.
func NewManager(wails *application.App, coreApp *app.App, configPath string) *Manager {
	return &Manager{
		wails:      wails,
		coreApp:    coreApp,
		configPath: configPath,
		state:      NewSharedState(),
		coreState:  CoreStateStopped,
	}
}

// SetServices registers service references with the manager
// This will be implemented in Phase 3 when services are created
// func (m *Manager) SetServices(tool *ToolService, resource *ResourceService, prompt *PromptService, log *LogService) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.toolService = tool
// 	m.resourceService = resource
// 	m.promptService = prompt
// 	m.logService = log
// }

// Start starts the Core and auto-starts Watch subscriptions.
func (m *Manager) Start(ctx context.Context) error {
	return m.startWithConfig(ctx, m.configPath, m.lastObservability)
}

// StartWithOptions starts Core with explicit configuration overrides.
func (m *Manager) StartWithOptions(ctx context.Context, opts StartCoreOptions) error {
	configPath, observability := resolveStartOptions(opts, m.configPath)
	return m.startWithConfig(ctx, configPath, observability)
}

func (m *Manager) startWithConfig(ctx context.Context, configPath string, observability *app.ObservabilityOptions) error {
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

	emitCoreState(wails, string(CoreStateStarting), nil)

	// Start Core in background
	go m.runCore(cfg)

	return nil
}

func resolveStartOptions(opts StartCoreOptions, fallback string) (string, *app.ObservabilityOptions) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	configPath := strings.TrimSpace(opts.ConfigPath)
	if configPath == "" {
		switch {
		case fallback != "":
			configPath = fallback
		default:
			configPath = "." // TODO: dynamic resolve with user data.
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

			emitCoreState(wails, string(CoreStateError), err)
			emitError(wails, ErrCodeCoreFailed, "Core panic", err.Error())
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
		emitCoreState(wails, string(CoreStateError), emitErr)
		emitError(wails, ErrCodeCoreFailed, "Core failed", emitErr.Error())
	} else {
		emitCoreState(wails, string(CoreStateStopped), nil)
	}
}

func (m *Manager) handleControlPlaneReady(cp domain.ControlPlane) {
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
	event := CoreStateEvent{
		State:  string(CoreStateRunning),
		Uptime: uptime,
	}
	if wails != nil {
		wails.Event.Emit(EventCoreState, event)
	}

	// Auto-start Watch subscriptions
	m.startWatchers()
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
			emitError(wails, ErrCodeInternal, "Failed to start runtime status watcher", err.Error())
			return
		}
		for snapshot := range updates {
			emitRuntimeStatusUpdated(wails, snapshot)
		}
	}()

	// Watch server init status
	go func() {
		updates, err := cp.WatchServerInitStatusAllServers(ctx)
		if err != nil {
			emitError(wails, ErrCodeInternal, "Failed to start server init status watcher", err.Error())
			return
		}
		for snapshot := range updates {
			emitServerInitUpdated(wails, snapshot)
		}
	}()

	// Watch active clients
	go func() {
		updates, err := cp.WatchActiveClients(ctx)
		if err != nil {
			emitError(wails, ErrCodeInternal, "Failed to start active clients watcher", err.Error())
			return
		}
		for snapshot := range updates {
			emitActiveClientsUpdated(wails, snapshot)
		}
	}()

	// Placeholder for other watchers (tools, resources, prompts, logs) that will be added in Phase 3
	// ctx := context.Background()

	// // Start tool watcher
	// if m.toolService != nil {
	// 	go func() {
	// 		if err := m.toolService.WatchTools(ctx); err != nil {
	// 			emitError(m.wails, ErrCodeInternal, "Failed to start tool watcher", err.Error())
	// 		}
	// 	}()
	// }

	// // Start resource watcher
	// if m.resourceService != nil {
	// 	go func() {
	// 		if err := m.resourceService.WatchResources(ctx); err != nil {
	// 			emitError(m.wails, ErrCodeInternal, "Failed to start resource watcher", err.Error())
	// 		}
	// 	}()
	// }

	// // Start prompt watcher
	// if m.promptService != nil {
	// 	go func() {
	// 		if err := m.promptService.WatchPrompts(ctx); err != nil {
	// 			emitError(m.wails, ErrCodeInternal, "Failed to start prompt watcher", err.Error())
	// 		}
	// 	}()
	// }

	// // Start log streamer
	// if m.logService != nil {
	// 	go func() {
	// 		if err := m.logService.StreamLogs(ctx); err != nil {
	// 			emitError(m.wails, ErrCodeInternal, "Failed to start log streamer", err.Error())
	// 		}
	// 	}()
	// }
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

	emitCoreState(wails, string(CoreStateStopping), nil)

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
	defer m.mu.Unlock()

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

// GetControlPlane returns the ControlPlane interface from Core
// Returns error if Core is not running.
func (m *Manager) GetControlPlane() (domain.ControlPlane, error) {
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

// SetControlPlane sets the ControlPlane instance
// This should be called after Core successfully starts.
func (m *Manager) SetControlPlane(cp domain.ControlPlane) {
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
	defer m.mu.Unlock()
	m.wails = wails
}

// GetConfigPath returns the configuration path.
func (m *Manager) GetConfigPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configPath
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
