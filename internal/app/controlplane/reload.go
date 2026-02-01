package controlplane

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	appRuntime "mcpv/internal/app/runtime"
	"mcpv/internal/domain"
	"mcpv/internal/infra/notifications"
	"mcpv/internal/infra/pipeline"
	"mcpv/internal/infra/plugin"
	"mcpv/internal/infra/telemetry"
)

// ReloadManager coordinates catalog reloads and applies updates.
type ReloadManager struct {
	provider      domain.CatalogProvider
	state         *State
	registry      *ClientRegistry
	scheduler     domain.Scheduler
	initManager   *bootstrap.ServerInitializationManager
	pluginManager *plugin.Manager
	pipeline      *pipeline.Engine
	metrics       domain.Metrics
	health        *telemetry.HealthTracker
	metadataCache *domain.MetadataCache
	listChanges   *notifications.ListChangeHub
	coreLogger    *zap.Logger
	logger        *zap.Logger
	appliedRev    atomic.Uint64
	started       atomic.Bool
}

type reloadApplyError struct {
	stage string
	err   error
}

func (e reloadApplyError) Error() string {
	return fmt.Sprintf("%s: %v", e.stage, e.err)
}

func (e reloadApplyError) Unwrap() error {
	return e.err
}

// NewReloadManager constructs a reload manager.
func NewReloadManager(
	provider domain.CatalogProvider,
	state *State,
	registry *ClientRegistry,
	scheduler domain.Scheduler,
	initManager *bootstrap.ServerInitializationManager,
	pluginManager *plugin.Manager,
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
	return &ReloadManager{
		provider:      provider,
		state:         state,
		registry:      registry,
		scheduler:     scheduler,
		initManager:   initManager,
		pluginManager: pluginManager,
		pipeline:      pipelineEngine,
		metrics:       metrics,
		health:        health,
		metadataCache: metadataCache,
		listChanges:   listChanges,
		coreLogger:    logger,
		logger:        logger.Named("reload"),
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
		m.recordReloadFailure(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return err
	}
	if err := m.provider.Reload(ctx); err != nil {
		if errors.Is(err, domain.ErrReloadRestartRequired) {
			m.recordReloadRestart(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		} else {
			m.recordReloadFailure(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		}
		return err
	}
	if !m.started.Load() {
		m.recordReloadSuccess(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return nil
	}
	next, err := m.provider.Snapshot(ctx)
	if err != nil {
		m.recordReloadFailure(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return err
	}
	if next.Revision == prev.Revision {
		m.recordReloadSuccess(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return nil
	}
	if err := m.waitForRevision(ctx, next.Revision); err != nil {
		m.recordReloadFailure(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
		return err
	}
	m.recordReloadSuccess(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
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
				m.recordReloadFailure(update.Source, domain.ReloadActionEntry)
				m.recordReloadActionFailures(update.Source, update.Diff)
				m.logger.Warn("config reload apply failed", zap.Error(err))
				duration := time.Since(started)
				m.handleApplyError(update, err, duration)
				continue
			}
			duration := time.Since(started)
			reloadMode := resolveReloadMode(update.Snapshot.Summary.Runtime.ReloadMode)
			m.observeReloadApply(reloadMode, domain.ReloadApplyResultSuccess, "none", duration)
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

type reloadStep struct {
	name     string
	apply    func(context.Context) error
	rollback func(context.Context) error
}

func (m *ReloadManager) applyUpdate(ctx context.Context, update domain.CatalogUpdate) error {
	started := time.Now()
	prevSnapshot, err := m.currentSnapshot()
	if err != nil {
		return err
	}
	diff := update.Diff
	if diff.RuntimeDiff.RequiresRestart() {
		m.recordReloadRestart(update.Source, domain.ReloadActionEntry)
		return reloadApplyError{stage: "restart_required", err: domain.ErrReloadRestartRequired}
	}

	addedServers := serverNamesForSpecKeys(update.Snapshot.Catalog, diff.AddedSpecKeys)
	removedServers := serverNamesForSpecKeys(prevSnapshot.Catalog, diff.RemovedSpecKeys)
	updatedServers := serverNamesForSpecKeys(update.Snapshot.Catalog, diff.UpdatedSpecKeys)
	replacedServers := serverNamesForSpecKeys(prevSnapshot.Catalog, diff.ReplacedSpecKeys)
	changedFields := diffChangedFields(diff)
	runtimeOnly := diff.IsRuntimeOnly()

	steps := m.buildReloadSteps(prevSnapshot, update)
	reloadMode := resolveReloadMode(update.Snapshot.Summary.Runtime.ReloadMode)
	if err := m.applyTransaction(ctx, steps, reloadMode); err != nil {
		return err
	}

	m.refreshRuntime(ctx, update, m.state.RuntimeState())

	m.recordReloadSuccess(update.Source, domain.ReloadActionEntry)
	for range update.Diff.AddedSpecKeys {
		m.recordReloadSuccess(update.Source, domain.ReloadActionServerAdd)
	}
	for range update.Diff.RemovedSpecKeys {
		m.recordReloadSuccess(update.Source, domain.ReloadActionServerRemove)
	}
	for range update.Diff.UpdatedSpecKeys {
		m.recordReloadSuccess(update.Source, domain.ReloadActionServerUpdate)
	}
	for range update.Diff.ReplacedSpecKeys {
		m.recordReloadSuccess(update.Source, domain.ReloadActionServerReplace)
		m.recordReloadRestart(update.Source, domain.ReloadActionServerReplace)
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

func (m *ReloadManager) buildReloadSteps(prev domain.CatalogState, update domain.CatalogUpdate) []reloadStep {
	diff := update.Diff
	runtimeOnly := diff.IsRuntimeOnly()
	reverseDiff := domain.DiffCatalogStates(update.Snapshot, prev)

	steps := make([]reloadStep, 0, 2)
	if !runtimeOnly || m.initManager != nil {
		steps = append(steps, reloadStep{
			name: "scheduler_apply",
			apply: func(ctx context.Context) error {
				if !runtimeOnly && m.scheduler != nil {
					if err := m.scheduler.ApplyCatalogDiff(ctx, diff, update.Snapshot.Summary.SpecRegistry); err != nil {
						if rollbackErr := m.scheduler.ApplyCatalogDiff(ctx, reverseDiff, prev.Summary.SpecRegistry); rollbackErr != nil {
							return errors.Join(err, rollbackErr)
						}
						return err
					}
				}
				if m.initManager != nil {
					m.initManager.ApplyCatalogState(&update.Snapshot)
				}
				return nil
			},
			rollback: func(ctx context.Context) error {
				var rollbackErr error
				if !runtimeOnly && m.scheduler != nil {
					if err := m.scheduler.ApplyCatalogDiff(ctx, reverseDiff, prev.Summary.SpecRegistry); err != nil {
						rollbackErr = err
					}
				}
				if m.initManager != nil {
					m.initManager.ApplyCatalogState(&prev)
				}
				return rollbackErr
			},
		})
	}

	steps = append(steps, m.buildStateRegistryStep(prev, update, reverseDiff))
	if diff.PluginsChanged && (m.pluginManager != nil || m.pipeline != nil) {
		prevPlugins := prev.Summary.Plugins
		nextPlugins := update.Snapshot.Summary.Plugins
		steps = append(steps, reloadStep{
			name: "plugins",
			apply: func(ctx context.Context) error {
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
			rollback: func(ctx context.Context) error {
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

func (m *ReloadManager) buildStateRegistryStep(prev domain.CatalogState, update domain.CatalogUpdate, reverseDiff domain.CatalogDiff) reloadStep {
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

	return reloadStep{
		name: "state_registry",
		apply: func(ctx context.Context) error {
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
		rollback: func(ctx context.Context) error {
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

func (m *ReloadManager) buildRuntimeConfigStep(prev, next domain.RuntimeConfig) reloadStep {
	return reloadStep{
		name: "runtime_config",
		apply: func(ctx context.Context) error {
			return m.applyRuntimeConfig(ctx, prev, next)
		},
		rollback: func(ctx context.Context) error {
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

func (m *ReloadManager) applyTransaction(ctx context.Context, steps []reloadStep, mode domain.ReloadMode) error {
	applied := make([]reloadStep, 0, len(steps))
	for _, step := range steps {
		if err := step.apply(ctx); err != nil {
			applyErr := wrapReloadStage(step.name, err)
			rollbackStart := time.Now()
			if rollbackErr := m.rollbackSteps(ctx, applied); rollbackErr != nil {
				rollbackDuration := time.Since(rollbackStart)
				m.observeReloadRollback(mode, domain.ReloadRollbackResultFailure, step.name, rollbackDuration)
				m.logger.Warn("config reload rollback failed",
					zap.String("failure_stage", step.name),
					zap.Duration("latency", rollbackDuration),
					zap.Error(rollbackErr),
				)
				return errors.Join(applyErr, wrapReloadStage("rollback", rollbackErr))
			}
			rollbackDuration := time.Since(rollbackStart)
			m.observeReloadRollback(mode, domain.ReloadRollbackResultSuccess, step.name, rollbackDuration)
			m.logger.Info("config reload rolled back",
				zap.String("failure_stage", step.name),
				zap.Duration("latency", rollbackDuration),
			)
			return applyErr
		}
		applied = append(applied, step)
	}
	return nil
}

func (m *ReloadManager) rollbackSteps(ctx context.Context, steps []reloadStep) error {
	if len(steps) == 0 {
		return nil
	}
	var rollbackErr error
	for i := len(steps) - 1; i >= 0; i-- {
		step := steps[i]
		if step.rollback == nil {
			continue
		}
		if err := step.rollback(ctx); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	return rollbackErr
}

func (m *ReloadManager) currentSnapshot() (domain.CatalogState, error) {
	catalog := m.state.Catalog()
	revision := m.appliedRev.Load()
	return domain.NewCatalogState(catalog, revision, time.Now())
}

func wrapReloadStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	var applyErr reloadApplyError
	if errors.As(err, &applyErr) {
		return err
	}
	return reloadApplyError{stage: stage, err: err}
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

func (m *ReloadManager) recordReloadSuccess(source domain.CatalogUpdateSource, action domain.ReloadAction) {
	if m.metrics == nil {
		return
	}
	m.metrics.RecordReloadSuccess(source, action)
}

func (m *ReloadManager) recordReloadFailure(source domain.CatalogUpdateSource, action domain.ReloadAction) {
	if m.metrics == nil {
		return
	}
	m.metrics.RecordReloadFailure(source, action)
}

func (m *ReloadManager) recordReloadRestart(source domain.CatalogUpdateSource, action domain.ReloadAction) {
	if m.metrics == nil {
		return
	}
	m.metrics.RecordReloadRestart(source, action)
}

func (m *ReloadManager) recordReloadActionFailures(source domain.CatalogUpdateSource, diff domain.CatalogDiff) {
	for range diff.AddedSpecKeys {
		m.recordReloadFailure(source, domain.ReloadActionServerAdd)
	}
	for range diff.RemovedSpecKeys {
		m.recordReloadFailure(source, domain.ReloadActionServerRemove)
	}
	for range diff.UpdatedSpecKeys {
		m.recordReloadFailure(source, domain.ReloadActionServerUpdate)
	}
	for range diff.ReplacedSpecKeys {
		m.recordReloadFailure(source, domain.ReloadActionServerReplace)
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

func (m *ReloadManager) handleApplyError(update domain.CatalogUpdate, err error, duration time.Duration) {
	reloadMode := resolveReloadMode(update.Snapshot.Summary.Runtime.ReloadMode)
	stage := reloadFailureStage(err)
	fields := []zap.Field{
		zap.Uint64("revision", update.Snapshot.Revision),
		zap.Int("servers", update.Snapshot.Summary.TotalServers),
		zap.Int("added", len(update.Diff.AddedSpecKeys)),
		zap.Int("removed", len(update.Diff.RemovedSpecKeys)),
		zap.Int("updated", len(update.Diff.UpdatedSpecKeys)),
		zap.String("reload_mode", string(reloadMode)),
		zap.String("failure_stage", stage),
		zap.String("failure_summary", err.Error()),
		zap.Duration("latency", duration),
		zap.Error(err),
	}
	m.observeReloadApply(reloadMode, domain.ReloadApplyResultFailure, stage, duration)
	if reloadMode == domain.ReloadModeStrict {
		m.coreLogger.Fatal("config reload apply failed; shutting down", fields...)
	}
	m.logger.Warn("config reload apply failed", fields...)
}

func (m *ReloadManager) observeReloadApply(mode domain.ReloadMode, result domain.ReloadApplyResult, summary string, duration time.Duration) {
	if m.metrics == nil {
		return
	}
	m.metrics.ObserveReloadApply(domain.ReloadApplyMetric{
		Mode:     mode,
		Result:   result,
		Summary:  summary,
		Duration: duration,
	})
}

func (m *ReloadManager) observeReloadRollback(mode domain.ReloadMode, result domain.ReloadRollbackResult, summary string, duration time.Duration) {
	if m.metrics == nil {
		return
	}
	m.metrics.ObserveReloadRollback(domain.ReloadRollbackMetric{
		Mode:     mode,
		Result:   result,
		Summary:  summary,
		Duration: duration,
	})
}

func resolveReloadMode(mode domain.ReloadMode) domain.ReloadMode {
	if mode == "" {
		return domain.DefaultReloadMode
	}
	return mode
}

func reloadFailureStage(err error) string {
	var applyErr reloadApplyError
	if errors.As(err, &applyErr) && applyErr.stage != "" {
		return applyErr.stage
	}
	return "unknown"
}
