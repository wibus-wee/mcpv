package ui

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	catalogloader "mcpv/internal/infra/catalog/loader"
	"mcpv/internal/ui/events"
	"mcpv/internal/ui/uiconfig"
)

type TrayController struct {
	mu sync.Mutex

	app     *application.App
	window  application.Window
	manager *Manager
	logger  *zap.Logger

	tray         *application.SystemTray
	dock         *dock.DockService
	settings     TraySettings
	started      bool
	menu         *application.Menu
	menuAttached bool
}

func NewTrayController(app *application.App, window application.Window, manager *Manager, logger *zap.Logger) *TrayController {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TrayController{
		app:     app,
		window:  window,
		manager: manager,
		logger:  logger.Named("tray"),
		settings: TraySettings{
			ClickAction: TrayClickActionOpenMenu,
		},
	}
}

func (c *TrayController) ApplyFromStore(_ context.Context) error {
	if c == nil || c.manager == nil {
		return nil
	}
	store, err := c.manager.UISettingsStore()
	if err != nil {
		return err
	}
	snapshot, err := store.Get(uiconfig.ScopeGlobal, "")
	if err != nil {
		return err
	}
	settings, err := ParseTraySettings(snapshot.Sections[TraySectionKey])
	if err != nil {
		c.logger.Warn("failed to parse tray settings", zap.Error(err))
		settings = DefaultTraySettings()
	}
	return c.apply(settings, true)
}

func (c *TrayController) ApplyFromSnapshot(snapshot uiconfig.Snapshot) error {
	settings, err := ParseTraySettings(snapshot.Sections[TraySectionKey])
	if err != nil {
		c.logger.Warn("failed to parse tray settings", zap.Error(err))
		settings = DefaultTraySettings()
	}
	return c.apply(settings, false)
}

func (c *TrayController) Apply(settings TraySettings) error {
	return c.apply(settings, false)
}

func (c *TrayController) Shutdown() {
	if c == nil {
		return
	}
	c.destroyTray()
}

func (c *TrayController) MarkStarted() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.started = true
	c.mu.Unlock()
	c.refreshMenu(context.Background())
}

func (c *TrayController) ensureTray() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tray != nil || c.app == nil {
		return
	}
	tray := c.app.SystemTray.New()
	tray.SetTooltip("mcpv")
	if len(trayIconPng) > 0 {
		tray.SetTemplateIcon(trayIconPng)
	}
	tray.OnClick(c.handleTrayClick)
	tray.OnRightClick(c.handleTrayRightClick)
	tray.OnDoubleClick(c.handleTrayDoubleClick)
	c.tray = tray
	c.menuAttached = false
}

func (c *TrayController) destroyTray() {
	c.mu.Lock()
	tray := c.tray
	c.tray = nil
	c.menuAttached = false
	c.mu.Unlock()
	if tray != nil {
		tray.Destroy()
	}
}

func (c *TrayController) applyDock(settings TraySettings) {
	if runtime.GOOS != "darwin" {
		return
	}
	c.mu.Lock()
	if c.dock == nil {
		c.dock = dock.New()
	}
	dockService := c.dock
	c.mu.Unlock()

	if settings.Enabled && settings.HideDock {
		dockService.HideAppIcon()
		return
	}
	dockService.ShowAppIcon()
}

// RefreshDockVisibility reapplies the current dock visibility settings.
func (c *TrayController) RefreshDockVisibility() {
	if c == nil {
		return
	}
	c.mu.Lock()
	settings := c.settings
	c.mu.Unlock()
	c.applyDock(settings)
}

func (c *TrayController) handleTrayClick() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c.refreshMenu(ctx)
	switch c.currentClickAction() {
	case TrayClickActionOpenMenu:
		c.openMenu()
	case TrayClickActionToggle:
		c.toggleWindow()
	case TrayClickActionShow:
		c.showWindow()
	default:
		c.openMenu()
	}
}

func (c *TrayController) handleTrayRightClick() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c.refreshMenu(ctx)
	c.openMenu()
}

func (c *TrayController) handleTrayDoubleClick() {
	c.showWindow()
}

func (c *TrayController) currentClickAction() TrayClickAction {
	c.mu.Lock()
	action := c.settings.ClickAction
	c.mu.Unlock()
	return normalizeTrayClickAction(action)
}

func (c *TrayController) openMenu() {
	c.mu.Lock()
	tray := c.tray
	c.mu.Unlock()
	if tray != nil {
		tray.OpenMenu()
	}
}

func (c *TrayController) refreshMenu(ctx context.Context) {
	c.mu.Lock()
	tray := c.tray
	started := c.started
	menu := c.menu
	menuAttached := c.menuAttached
	c.mu.Unlock()
	if tray == nil {
		return
	}
	if menu == nil {
		menu = application.NewMenu()
		c.mu.Lock()
		if c.menu == nil {
			c.menu = menu
		} else {
			menu = c.menu
		}
		c.mu.Unlock()
	}

	menu.Clear()
	c.populateMenu(ctx, menu)

	if !menuAttached {
		tray.SetMenu(menu)
		c.mu.Lock()
		c.menuAttached = true
		c.mu.Unlock()
	}
	if started {
		menu.Update()
	}
}

func (c *TrayController) populateMenu(ctx context.Context, menu *application.Menu) {
	if menu == nil {
		return
	}

	// Top-level action
	visible := c.isWindowVisible()
	if !visible {
		menu.Add("Open mcpv").OnClick(func(_ *application.Context) {
			c.showWindow()
		})
	} else {
		menu.Add("Hide mcpv").OnClick(func(_ *application.Context) {
			c.hideWindow()
		})
	}

	menu.AddSeparator()

	// Servers section
	state, stateErr := c.resolveCoreState()
	coreRunning := state == CoreStateRunning || state == CoreStateStarting || state == CoreStateStopping

	c.addServersSection(ctx, menu, coreRunning)

	menu.AddSeparator()

	// Core section
	c.addCoreSection(menu, state, stateErr, coreRunning)

	menu.AddSeparator()

	// Bottom actions
	menu.Add("Open Logs").OnClick(func(_ *application.Context) {
		c.openLogs()
	})
	menu.Add("Settings...").OnClick(func(_ *application.Context) {
		c.openSettings()
	})

	menu.AddSeparator()

	menu.Add("Quit mcpv").OnClick(func(_ *application.Context) {
		if c.app != nil {
			c.app.Quit()
		}
	})
}

func (c *TrayController) addServersSection(ctx context.Context, menu *application.Menu, coreRunning bool) {
	serversMenu := menu.AddSubmenu("Servers")

	serverItems := c.buildServerItems(ctx, coreRunning)
	if len(serverItems) == 0 {
		empty := serversMenu.Add("  No servers configured")
		empty.SetEnabled(false)
		return
	}

	for _, server := range serverItems {
		serverCopy := server
		c.addServerItem(serversMenu, serverCopy)
	}
}

func (c *TrayController) addServerItem(menu *application.Menu, server trayServerItem) {
	// Server submenu (single-level)
	statusIcon := c.getStatusIcon(server.status)
	label := fmt.Sprintf("%s %s", statusIcon, server.name)
	serverMenu := menu.AddSubmenu(label)

	serverMenu.Add("Open in App").OnClick(func(_ *application.Context) {
		c.openServers(server.name)
	})

	if server.status == "running" {
		serverMenu.Add("View Logs").OnClick(func(_ *application.Context) {
			c.openLogs()
		})
	}

	serverMenu.AddSeparator()
	serverMenu.Add(fmt.Sprintf("Status: %s", server.status)).SetEnabled(false)
	serverMenu.Add(fmt.Sprintf("Transport: %s", server.spec.Transport)).SetEnabled(false)

	if server.spec.ActivationMode != "" {
		serverMenu.Add(fmt.Sprintf("Activation: %s", server.spec.ActivationMode)).SetEnabled(false)
	}

	if server.spec.Strategy != "" {
		serverMenu.Add(fmt.Sprintf("Strategy: %s", server.spec.Strategy)).SetEnabled(false)
	}

	if len(server.spec.Tags) > 0 {
		serverMenu.Add(fmt.Sprintf("Tags: %s", strings.Join(server.spec.Tags, ", "))).SetEnabled(false)
	}
}

func (c *TrayController) addCoreSection(menu *application.Menu, state CoreState, stateErr error, coreRunning bool) {
	statusIcon := c.getCoreStatusIcon(state)
	coreMenu := menu.AddSubmenu(fmt.Sprintf("%s Core", statusIcon))

	if stateErr != nil {
		errorItem := coreMenu.Add("Status unavailable")
		errorItem.SetEnabled(false)
		coreMenu.AddSeparator()
	}

	startItem := coreMenu.Add("Start Core")
	startItem.SetEnabled(!coreRunning)
	startItem.OnClick(func(_ *application.Context) {
		c.startCore()
	})

	stopItem := coreMenu.Add("Stop Core")
	stopItem.SetEnabled(coreRunning)
	stopItem.OnClick(func(_ *application.Context) {
		c.stopCore()
	})

	restartItem := coreMenu.Add("Restart Core")
	restartItem.SetEnabled(coreRunning)
	restartItem.OnClick(func(_ *application.Context) {
		c.restartCore()
	})

	reloadItem := coreMenu.Add("Reload Config")
	reloadItem.SetEnabled(coreRunning)
	reloadItem.OnClick(func(_ *application.Context) {
		c.reloadConfig()
	})
}

func (c *TrayController) getStatusIcon(status string) string {
	switch status {
	case "running":
		return "●" // Green dot (color not supported in text)
	case "starting":
		return "◐" // Half circle
	case "disabled":
		return "○" // Empty circle
	case "offline":
		return "○"
	case "idle":
		return "○"
	default:
		return "○"
	}
}

func (c *TrayController) getCoreStatusIcon(state CoreState) string {
	switch state {
	case CoreStateRunning:
		return "●"
	case CoreStateStarting:
		return "◐"
	case CoreStateStopping:
		return "◐"
	case CoreStateStopped:
		return "○"
	case CoreStateError:
		return "○"
	default:
		return "○"
	}
}

type trayServerItem struct {
	name    string
	spec    domain.ServerSpec
	specKey string
	status  string
}

func (c *TrayController) buildServerItems(ctx context.Context, coreRunning bool) []trayServerItem {
	catalog, err := c.loadCatalog(ctx)
	if err != nil {
		c.logger.Debug("tray catalog load failed", zap.Error(err))
		return nil
	}

	poolMap := map[string]domain.PoolInfo{}
	if coreRunning {
		pools, err := c.loadPoolStatus(ctx)
		if err == nil {
			poolMap = pools
		}
	}

	names := make([]string, 0, len(catalog.Specs))
	for name := range catalog.Specs {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]trayServerItem, 0, len(names))
	for _, name := range names {
		spec := catalog.Specs[name]
		specKey := domain.SpecFingerprint(spec)
		pool, ok := poolMap[specKey]
		status := resolveServerStatus(spec, coreRunning, ok, pool)
		items = append(items, trayServerItem{
			name:    spec.Name,
			spec:    spec,
			specKey: specKey,
			status:  status,
		})
	}
	return items
}

func resolveServerStatus(spec domain.ServerSpec, coreRunning bool, hasPool bool, pool domain.PoolInfo) string {
	if spec.Disabled {
		return "disabled"
	}
	if !coreRunning {
		return "offline"
	}
	if !hasPool {
		return "idle"
	}
	if pool.Diagnostics.StartInFlight || pool.Diagnostics.Starting > 0 {
		return "starting"
	}
	if len(pool.Instances) > 0 {
		return "running"
	}
	return "idle"
}

func (c *TrayController) coreState() (CoreState, error) {
	if c.manager == nil {
		return CoreStateStopped, fmt.Errorf("manager not set")
	}
	state, _, err := c.manager.GetState()
	return state, err
}

func (c *TrayController) resolveCoreState() (CoreState, error) {
	state, err := c.coreState()
	if err == nil {
		return state, nil
	}
	if c.manager == nil {
		return CoreStateStopped, err
	}
	if _, cpErr := c.manager.GetControlPlane(); cpErr == nil {
		return CoreStateRunning, nil
	}
	return state, err
}

func (c *TrayController) loadCatalog(ctx context.Context) (domain.Catalog, error) {
	if c.manager != nil {
		cp, err := c.manager.GetControlPlane()
		if err == nil {
			return cp.GetCatalog(), nil
		}
	}
	if c.manager == nil {
		return domain.Catalog{}, fmt.Errorf("manager not set")
	}
	path := strings.TrimSpace(c.manager.GetConfigPath())
	if path == "" {
		return domain.Catalog{}, fmt.Errorf("config path empty")
	}
	loader := catalogloader.NewLoader(c.logger)
	return loader.Load(ctx, path)
}

func (c *TrayController) loadPoolStatus(ctx context.Context) (map[string]domain.PoolInfo, error) {
	if c.manager == nil {
		return nil, fmt.Errorf("manager not set")
	}
	cp, err := c.manager.GetControlPlane()
	if err != nil {
		return nil, err
	}
	pools, err := cp.GetPoolStatus(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]domain.PoolInfo, len(pools))
	for _, pool := range pools {
		byKey[pool.SpecKey] = pool
	}
	return byKey, nil
}

func (c *TrayController) startCore() {
	if c.manager == nil {
		return
	}
	if err := c.manager.Start(context.Background()); err != nil {
		c.logger.Warn("tray start core failed", zap.Error(err))
	}
}

func (c *TrayController) stopCore() {
	if c.manager == nil {
		return
	}
	if err := c.manager.Stop(); err != nil {
		c.logger.Warn("tray stop core failed", zap.Error(err))
	}
}

func (c *TrayController) restartCore() {
	if c.manager == nil {
		return
	}
	if err := c.manager.Restart(context.Background()); err != nil {
		c.logger.Warn("tray restart core failed", zap.Error(err))
	}
}

func (c *TrayController) reloadConfig() {
	if c.manager == nil {
		return
	}
	if err := c.manager.ReloadConfig(context.Background()); err != nil {
		c.logger.Warn("tray reload config failed", zap.Error(err))
	}
}

func (c *TrayController) openSettings() {
	c.openPath("/settings/advanced", nil)
}

func (c *TrayController) openLogs() {
	c.openPath("/logs", nil)
}

func (c *TrayController) openServers(serverName string) {
	params := map[string]string{}
	if strings.TrimSpace(serverName) != "" {
		params["server"] = serverName
	}
	c.openPath("/servers", params)
}

func (c *TrayController) openPath(path string, params map[string]string) {
	c.showWindow()
	if c.app != nil {
		events.EmitDeepLink(c.app, path, params)
	}
}

func (c *TrayController) showWindow() {
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window == nil {
		return
	}
	window.Show().Focus()
}

func (c *TrayController) hideWindow() {
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window == nil {
		return
	}
	window.Hide()
}

func (c *TrayController) toggleWindow() {
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window == nil {
		return
	}
	if window.IsVisible() {
		window.Hide()
		return
	}
	window.Show().Focus()
}

func (c *TrayController) isWindowVisible() bool {
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window == nil {
		return false
	}
	return window.IsVisible()
}

func (c *TrayController) apply(settings TraySettings, allowHide bool) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	c.settings = settings
	c.mu.Unlock()

	if !settings.Enabled {
		c.destroyTray()
		c.applyDock(settings)
		return nil
	}

	c.ensureTray()
	c.applyDock(settings)
	c.refreshMenu(context.Background())

	if settings.StartHidden && allowHide {
		c.hideWindow()
	}

	return nil
}
