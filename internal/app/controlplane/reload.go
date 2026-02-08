package controlplane

import (
	"context"
	"errors"
	"sort"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	reloadpkg "mcpv/internal/app/controlplane/reload"
	appRuntime "mcpv/internal/app/runtime"
	"mcpv/internal/domain"
	"mcpv/internal/infra/notifications"
	"mcpv/internal/infra/pipeline"
	pluginmanager "mcpv/internal/infra/plugin/manager"
	"mcpv/internal/infra/telemetry"
)

// ReloadManager coordinates catalog reloads and applies updates.
type ReloadManager struct {
	provider      domain.CatalogProvider
	state         *State
	registry      *ClientRegistry
	scheduler     domain.Scheduler
	startup       *bootstrap.ServerStartupOrchestrator
	pluginManager *pluginmanager.Manager
	pipeline      *pipeline.Engine
	metrics       domain.Metrics
	health        *telemetry.HealthTracker
	metadataCache *domain.MetadataCache
	listChanges   *notifications.ListChangeHub
	coreLogger    *zap.Logger
	logger        *zap.Logger
	observer      *reloadpkg.Observer
	transaction   *reloadpkg.Transaction
	appliedRev    atomic.Uint64
	started       atomic.Bool
}

// NewReloadManager constructs a reload manager.
func NewReloadManager(
	provider domain.CatalogProvider,
	state *State,
	registry *ClientRegistry,
	scheduler domain.Scheduler,
	startup *bootstrap.ServerStartupOrchestrator,
	pluginManager *pluginmanager.Manager,
	pipelineEngine *pipeline.Engine,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	metadataCache *domain.MetadataCache,
	listChanges *notifications.ListChangeHub,
	logger *zap.Logger,
) *ReloadManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	coreLogger := logger
	reloadLogger := logger.Named("reload")
	observer := reloadpkg.NewObserver(metrics, coreLogger, reloadLogger)
	transaction := reloadpkg.NewTransaction(observer, reloadLogger)
	return &ReloadManager{
		provider:      provider,
		state:         state,
		registry:      registry,
		scheduler:     scheduler,
		startup:       startup,
		pluginManager: pluginManager,
		pipeline:      pipelineEngine,
		metrics:       metrics,
		health:        health,
		metadataCache: metadataCache,
		listChanges:   listChanges,
		coreLogger:    coreLogger,
		logger:        reloadLogger,
		observer:      observer,
		transaction:   transaction,
	}
}

// Start begins watching for catalog updates.
func (m *ReloadManager) Start(ctx context.Context) error {
	updates, err := m.provider.Watch(ctx)
	if err != nil {
		return err
	}
	if snapshot, err := m.provider.Snapshot(ctx); err == nil {
		m.appliedRev.Store(snapshot.Revision)
	}
	m.started.Store(true)
	go m.run(ctx, updates)
	return nil
}

// Reload forces a catalog reload and waits for application.
func (m *ReloadManager) Reload(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	prev, err := m.provider.Snapshot(ctx)
	if err != nil {
		m.observer.RecordReloadFailure(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return err
	}
	if err := m.provider.Reload(ctx); err != nil {
		if errors.Is(err, domain.ErrReloadRestartRequired) {
			m.observer.RecordReloadRestart(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		} else {
			m.observer.RecordReloadFailure(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		}
		return err
	}
	if !m.started.Load() {
		m.observer.RecordReloadSuccess(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return nil
	}
	next, err := m.provider.Snapshot(ctx)
	if err != nil {
		m.observer.RecordReloadFailure(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return err
	}
	if next.Revision == prev.Revision {
		m.observer.RecordReloadSuccess(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return nil
	}
	if err := m.waitForRevision(ctx, next.Revision); err != nil {
		m.observer.RecordReloadFailure(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return err
	}
	m.observer.RecordReloadSuccess(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
	return nil
}

func (m *ReloadManager) run(ctx context.Context, updates <-chan domain.CatalogUpdate) {
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if update.Diff.IsEmpty() {
				continue
			}
			started := time.Now()
			if err := m.applyUpdate(ctx, update); err != nil {
				m.observer.RecordReloadFailure(update.Source, domain.ReloadActionEntry)
				m.observer.RecordReloadActionFailures(update.Source, update.Diff)
				m.logger.Warn("config reload apply failed", zap.Error(err))
				duration := time.Since(started)
				m.observer.HandleApplyError(update, err, duration)
				continue
			}
			duration := time.Since(started)
			reloadMode := reloadpkg.ResolveMode(update.Snapshot.Summary.Runtime.ReloadMode)
			m.observer.ObserveReloadApply(reloadMode, domain.ReloadApplyResultSuccess, "none", duration)
			m.logger.Info("config reload applied",
				zap.Uint64("revision", update.Snapshot.Revision),
				zap.Int("servers", update.Snapshot.Summary.TotalServers),
				zap.Int("added", len(update.Diff.AddedSpecKeys)),
				zap.Int("removed", len(update.Diff.RemovedSpecKeys)),
				zap.Int("updated", len(update.Diff.UpdatedSpecKeys)),
				zap.Int("plugins_added", len(update.Diff.AddedPlugins)),
				zap.Int("plugins_removed", len(update.Diff.RemovedPlugins)),
				zap.Int("plugins_updated", len(update.Diff.UpdatedPlugins)),
				zap.String("reload_mode", string(reloadMode)),
				zap.Duration("latency", duration),
			)
		}
	}
}

func (m *ReloadManager) applyUpdate(ctx context.Context, update domain.CatalogUpdate) error {
	started := time.Now()
	prevSnapshot, err := m.currentSnapshot()
	if err != nil {
		return err
	}
	diff := update.Diff
	if diff.RuntimeDiff.RequiresRestart() {
		m.observer.RecordReloadRestart(update.Source, domain.ReloadActionEntry)
		return reloadpkg.ApplyError{Stage: "restart_required", Err: domain.ErrReloadRestartRequired}
	}

	addedServers := serverNamesForSpecKeys(update.Snapshot.Catalog, diff.AddedSpecKeys)
	removedServers := serverNamesForSpecKeys(prevSnapshot.Catalog, diff.RemovedSpecKeys)
	updatedServers := serverNamesForSpecKeys(update.Snapshot.Catalog, diff.UpdatedSpecKeys)
	replacedServers := serverNamesForSpecKeys(prevSnapshot.Catalog, diff.ReplacedSpecKeys)
	changedFields := diffChangedFields(diff)
	runtimeOnly := diff.IsRuntimeOnly()

	steps := m.buildReloadSteps(prevSnapshot, update)
	reloadMode := reloadpkg.ResolveMode(update.Snapshot.Summary.Runtime.ReloadMode)
	if err := m.transaction.Apply(ctx, steps, reloadMode); err != nil {
		return err
	}

	m.refreshRuntime(ctx, update, m.state.RuntimeState())

	m.observer.RecordReloadSuccess(update.Source, domain.ReloadActionEntry)
	for range update.Diff.AddedSpecKeys {
		m.observer.RecordReloadSuccess(update.Source, domain.ReloadActionServerAdd)
	}
	for range update.Diff.RemovedSpecKeys {
		m.observer.RecordReloadSuccess(update.Source, domain.ReloadActionServerRemove)
	}
	for range update.Diff.UpdatedSpecKeys {
		m.observer.RecordReloadSuccess(update.Source, domain.ReloadActionServerUpdate)
	}
	for range update.Diff.ReplacedSpecKeys {
		m.observer.RecordReloadSuccess(update.Source, domain.ReloadActionServerReplace)
		m.observer.RecordReloadRestart(update.Source, domain.ReloadActionServerReplace)
	}

	m.logger.Info("config reload applied",
		zap.Uint64("revision", update.Snapshot.Revision),
		zap.String("source", string(update.Source)),
		zap.Int("servers", update.Snapshot.Summary.TotalServers),
		zap.Int("added", len(update.Diff.AddedSpecKeys)),
		zap.Int("removed", len(update.Diff.RemovedSpecKeys)),
		zap.Int("updated", len(update.Diff.UpdatedSpecKeys)),
		zap.Int("replaced", len(update.Diff.ReplacedSpecKeys)),
		zap.Strings("servers_added", addedServers),
		zap.Strings("servers_removed", removedServers),
		zap.Strings("servers_updated", updatedServers),
		zap.Strings("servers_replaced", replacedServers),
		zap.Strings("changed_fields", changedFields),
		zap.Int("tools_only", len(update.Diff.ToolsOnlySpecKeys)),
		zap.Int("runtime_behavior", len(update.Diff.RuntimeBehaviorSpecKeys)),
		zap.Int("restart_required", len(update.Diff.RestartRequiredSpecKeys)),
		zap.Bool("runtime_only", runtimeOnly),
		zap.Duration("latency", time.Since(started)),
	)
	m.appliedRev.Store(update.Snapshot.Revision)
	return nil
}

func (m *ReloadManager) buildReloadSteps(prev domain.CatalogState, update domain.CatalogUpdate) []reloadpkg.Step {
	diff := update.Diff
	runtimeOnly := diff.IsRuntimeOnly()
	reverseDiff := domain.DiffCatalogStates(update.Snapshot, prev)

	steps := make([]reloadpkg.Step, 0, 2)
	if !runtimeOnly || m.startup != nil {
		steps = append(steps, reloadpkg.Step{
			Name: "scheduler_apply",
			Apply: func(ctx context.Context) error {
				if !runtimeOnly && m.scheduler != nil {
					if err := m.scheduler.ApplyCatalogDiff(ctx, diff, update.Snapshot.Summary.SpecRegistry); err != nil {
						if rollbackErr := m.scheduler.ApplyCatalogDiff(ctx, reverseDiff, prev.Summary.SpecRegistry); rollbackErr != nil {
							return errors.Join(err, rollbackErr)
						}
						return err
					}
				}
				if m.startup != nil {
					m.startup.ApplyCatalogState(&update.Snapshot)
				}
				return nil
			},
			Rollback: func(ctx context.Context) error {
				var rollbackErr error
				if !runtimeOnly && m.scheduler != nil {
					if err := m.scheduler.ApplyCatalogDiff(ctx, reverseDiff, prev.Summary.SpecRegistry); err != nil {
						rollbackErr = err
					}
				}
				if m.startup != nil {
					m.startup.ApplyCatalogState(&prev)
				}
				return rollbackErr
			},
		})
	}

	steps = append(steps, m.buildStateRegistryStep(prev, update, reverseDiff))
	if diff.PluginsChanged && (m.pluginManager != nil || m.pipeline != nil) {
		prevPlugins := prev.Summary.Plugins
		nextPlugins := update.Snapshot.Summary.Plugins
		steps = append(steps, reloadpkg.Step{
			Name: "plugins",
			Apply: func(ctx context.Context) error {
				if m.pluginManager != nil {
					if err := m.pluginManager.Apply(ctx, nextPlugins); err != nil {
						if rollbackErr := m.pluginManager.Apply(ctx, prevPlugins); rollbackErr != nil {
							return errors.Join(err, rollbackErr)
						}
						return err
					}
				}
				if m.pipeline != nil {
					m.pipeline.Update(nextPlugins)
				}
				return nil
			},
			Rollback: func(ctx context.Context) error {
				var rollbackErr error
				if m.pipeline != nil {
					m.pipeline.Update(prevPlugins)
				}
				if m.pluginManager != nil {
					if err := m.pluginManager.Apply(ctx, prevPlugins); err != nil {
						rollbackErr = err
					}
				}
				return rollbackErr
			},
		})
	}
	if diff.RuntimeChanged {
		steps = append(steps, m.buildRuntimeConfigStep(prev.Summary.Runtime, update.Snapshot.Summary.Runtime))
	}
	return steps
}

func (m *ReloadManager) buildStateRegistryStep(prev domain.CatalogState, update domain.CatalogUpdate, reverseDiff domain.CatalogDiff) reloadpkg.Step {
	diff := update.Diff
	prevRuntime := m.state.RuntimeState()
	runtime := prevRuntime
	runtimeCreated := false
	if runtime == nil {
		runtime = appRuntime.NewState(&update.Snapshot, m.scheduler, m.metrics, m.health, m.metadataCache, m.listChanges, m.coreLogger)
		runtimeCreated = true
	}
	shouldUpdateRuntime := !runtimeCreated && (diff.RuntimeChanged || diff.HasSpecChanges())
	rollbackUpdate := domain.CatalogUpdate{
		Snapshot: prev,
		Diff:     reverseDiff,
		Source:   update.Source,
	}

	return reloadpkg.Step{
		Name: "state_registry",
		Apply: func(ctx context.Context) error {
			if shouldUpdateRuntime {
				runtime.UpdateCatalog(update.Snapshot.Catalog, update.Snapshot.Summary.ServerSpecKeys, update.Snapshot.Summary.Runtime)
			}
			m.state.UpdateCatalog(&update.Snapshot, runtime)
			if m.registry == nil {
				return nil
			}
			if err := m.registry.ApplyCatalogUpdate(ctx, update); err != nil {
				if shouldUpdateRuntime {
					runtime.UpdateCatalog(prev.Catalog, prev.Summary.ServerSpecKeys, prev.Summary.Runtime)
				}
				m.state.UpdateCatalog(&prev, prevRuntime)
				if rollbackErr := m.registry.ApplyCatalogUpdate(ctx, rollbackUpdate); rollbackErr != nil {
					return errors.Join(err, rollbackErr)
				}
				return err
			}
			return nil
		},
		Rollback: func(ctx context.Context) error {
			var rollbackErr error
			if shouldUpdateRuntime {
				runtime.UpdateCatalog(prev.Catalog, prev.Summary.ServerSpecKeys, prev.Summary.Runtime)
			}
			m.state.UpdateCatalog(&prev, prevRuntime)
			if m.registry != nil {
				if err := m.registry.ApplyCatalogUpdate(ctx, rollbackUpdate); err != nil {
					rollbackErr = err
				}
			}
			return rollbackErr
		},
	}
}

func (m *ReloadManager) buildRuntimeConfigStep(prev, next domain.RuntimeConfig) reloadpkg.Step {
	return reloadpkg.Step{
		Name: "runtime_config",
		Apply: func(ctx context.Context) error {
			return m.applyRuntimeConfig(ctx, prev, next)
		},
		Rollback: func(ctx context.Context) error {
			return m.applyRuntimeConfig(ctx, next, prev)
		},
	}
}

func (m *ReloadManager) applyRuntimeConfig(ctx context.Context, prev, next domain.RuntimeConfig) error {
	runtime := m.state.RuntimeState()
	if runtime != nil {
		if err := runtime.ApplyRuntimeConfig(ctx, prev, next); err != nil {
			return err
		}
	}
	if m.registry != nil && prev.ClientCheckInterval() != next.ClientCheckInterval() {
		if err := m.registry.UpdateRuntimeConfig(ctx, prev, next); err != nil {
			return err
		}
	}
	if m.scheduler != nil && prev.PingIntervalSeconds != next.PingIntervalSeconds {
		m.scheduler.StopPingManager()
		if nextInterval := next.PingInterval(); nextInterval > 0 {
			m.scheduler.StartPingManager(nextInterval)
		}
	}
	return nil
}

func (m *ReloadManager) currentSnapshot() (domain.CatalogState, error) {
	catalog := m.state.Catalog()
	revision := m.appliedRev.Load()
	return domain.NewCatalogState(catalog, revision, time.Now())
}

func (m *ReloadManager) refreshRuntime(ctx context.Context, update domain.CatalogUpdate, runtime *appRuntime.State) {
	if runtime == nil {
		return
	}
	if len(update.Diff.AddedSpecKeys) == 0 && len(update.Diff.UpdatedSpecKeys) == 0 && len(update.Diff.ReplacedSpecKeys) == 0 {
		return
	}
	if runtime.Tools() != nil {
		if err := runtime.Tools().Refresh(ctx); err != nil {
			m.logger.Warn("tool refresh after reload failed", zap.Error(err))
		}
	}
	if runtime.Resources() != nil {
		if err := runtime.Resources().Refresh(ctx); err != nil {
			m.logger.Warn("resource refresh after reload failed", zap.Error(err))
		}
	}
	if runtime.Prompts() != nil {
		if err := runtime.Prompts().Refresh(ctx); err != nil {
			m.logger.Warn("prompt refresh after reload failed", zap.Error(err))
		}
	}
}

func (m *ReloadManager) waitForRevision(ctx context.Context, revision uint64) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if m.appliedRev.Load() >= revision {
		return nil
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if m.appliedRev.Load() >= revision {
				return nil
			}
		}
	}
}

func serverNamesForSpecKeys(catalog domain.Catalog, specKeys []string) []string {
	if len(specKeys) == 0 {
		return nil
	}
	lookup := make(map[string]struct{}, len(specKeys))
	for _, specKey := range specKeys {
		lookup[specKey] = struct{}{}
	}
	names := make([]string, 0, len(specKeys))
	for name, spec := range catalog.Specs {
		if spec.Disabled {
			continue
		}
		if _, ok := lookup[domain.SpecFingerprint(spec)]; ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func diffChangedFields(diff domain.CatalogDiff) []string {
	fields := make([]string, 0, 6)
	if len(diff.AddedSpecKeys) > 0 {
		fields = append(fields, "servers_added")
	}
	if len(diff.RemovedSpecKeys) > 0 {
		fields = append(fields, "servers_removed")
	}
	if len(diff.UpdatedSpecKeys) > 0 {
		fields = append(fields, "servers_updated")
	}
	if len(diff.ReplacedSpecKeys) > 0 {
		fields = append(fields, "servers_replaced")
	}
	if diff.TagsChanged {
		fields = append(fields, "tags")
	}
	if diff.RuntimeChanged {
		fields = append(fields, "runtime")
	}
	if diff.PluginsChanged {
		fields = append(fields, "plugins")
	}
	return fields
}
