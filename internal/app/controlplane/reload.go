package controlplane

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/app/bootstrap"
	appRuntime "mcpv/internal/app/runtime"
	"mcpv/internal/domain"
	"mcpv/internal/infra/notifications"
	"mcpv/internal/infra/telemetry"
)

// ReloadManager coordinates catalog reloads and applies updates.
type ReloadManager struct {
	provider      domain.CatalogProvider
	state         *State
	registry      *ClientRegistry
	scheduler     domain.Scheduler
	initManager   *bootstrap.ServerInitializationManager
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
		return err
	}
	if err := m.provider.Reload(ctx); err != nil {
		return err
	}
	if !m.started.Load() {
		return nil
	}
	next, err := m.provider.Snapshot(ctx)
	if err != nil {
		return err
	}
	if next.Revision == prev.Revision {
		return nil
	}
	return m.waitForRevision(ctx, next.Revision)
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
				zap.String("reload_mode", string(reloadMode)),
				zap.Duration("latency", duration),
			)
		}
	}
}

func (m *ReloadManager) applyUpdate(ctx context.Context, update domain.CatalogUpdate) error {
	runtime := m.state.RuntimeState()
	if runtime == nil {
		runtime = appRuntime.NewState(&update.Snapshot, m.scheduler, m.metrics, m.health, m.metadataCache, m.listChanges, m.coreLogger)
	} else {
		runtime.UpdateCatalog(update.Snapshot.Catalog, update.Snapshot.Summary.ServerSpecKeys, update.Snapshot.Summary.Runtime)
	}

	if err := m.scheduler.ApplyCatalogDiff(ctx, update.Diff, update.Snapshot.Summary.SpecRegistry); err != nil {
		return reloadApplyError{stage: "apply_catalog_diff", err: err}
	}
	if m.initManager != nil {
		m.initManager.ApplyCatalogState(&update.Snapshot)
	}

	m.state.UpdateCatalog(&update.Snapshot, runtime)

	if err := m.registry.ApplyCatalogUpdate(ctx, update); err != nil {
		return reloadApplyError{stage: "registry_update", err: err}
	}

	m.refreshRuntime(ctx, update, runtime)

	m.appliedRev.Store(update.Snapshot.Revision)
	return nil
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
