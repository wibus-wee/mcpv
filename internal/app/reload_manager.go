package app

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/notifications"
	"mcpd/internal/infra/telemetry"
)

// ReloadManager coordinates catalog reloads and applies updates.
type ReloadManager struct {
	provider      domain.CatalogProvider
	state         *controlPlaneState
	registry      *clientRegistry
	scheduler     domain.Scheduler
	initManager   *ServerInitializationManager
	metrics       domain.Metrics
	health        *telemetry.HealthTracker
	metadataCache *domain.MetadataCache
	listChanges   *notifications.ListChangeHub
	coreLogger    *zap.Logger
	logger        *zap.Logger
	appliedRev    atomic.Uint64
	started       atomic.Bool
}

// NewReloadManager constructs a reload manager.
func NewReloadManager(
	provider domain.CatalogProvider,
	state *controlPlaneState,
	registry *clientRegistry,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
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
			if err := m.applyUpdate(ctx, update); err != nil {
				m.logger.Warn("config reload apply failed", zap.Error(err))
			}
		}
	}
}

func (m *ReloadManager) applyUpdate(ctx context.Context, update domain.CatalogUpdate) error {
	started := time.Now()
	runtime := m.state.RuntimeState()
	if runtime == nil {
		runtime = buildRuntimeState(&update.Snapshot, m.scheduler, m.metrics, m.health, m.metadataCache, m.listChanges, m.coreLogger)
	} else {
		runtime.UpdateCatalog(update.Snapshot.Catalog, update.Snapshot.Summary.ServerSpecKeys, update.Snapshot.Summary.Runtime)
	}

	if err := m.scheduler.ApplyCatalogDiff(ctx, update.Diff, update.Snapshot.Summary.SpecRegistry); err != nil {
		return err
	}
	if m.initManager != nil {
		m.initManager.ApplyCatalogState(&update.Snapshot)
	}

	m.state.UpdateCatalog(&update.Snapshot, runtime)

	if err := m.registry.ApplyCatalogUpdate(ctx, update); err != nil {
		return err
	}

	m.refreshRuntime(ctx, update, runtime)

	m.logger.Info("config reload applied",
		zap.Uint64("revision", update.Snapshot.Revision),
		zap.Int("servers", update.Snapshot.Summary.TotalServers),
		zap.Int("added", len(update.Diff.AddedSpecKeys)),
		zap.Int("removed", len(update.Diff.RemovedSpecKeys)),
		zap.Int("updated", len(update.Diff.UpdatedSpecKeys)),
		zap.Duration("latency", time.Since(started)),
	)
	m.appliedRev.Store(update.Snapshot.Revision)
	return nil
}

func (m *ReloadManager) refreshRuntime(ctx context.Context, update domain.CatalogUpdate, runtime *runtimeState) {
	if runtime == nil {
		return
	}
	if len(update.Diff.AddedSpecKeys) == 0 && len(update.Diff.UpdatedSpecKeys) == 0 && len(update.Diff.ReplacedSpecKeys) == 0 {
		return
	}
	if runtime.tools != nil {
		if err := runtime.tools.Refresh(ctx); err != nil {
			m.logger.Warn("tool refresh after reload failed", zap.Error(err))
		}
	}
	if runtime.resources != nil {
		if err := runtime.resources.Refresh(ctx); err != nil {
			m.logger.Warn("resource refresh after reload failed", zap.Error(err))
		}
	}
	if runtime.prompts != nil {
		if err := runtime.prompts.Refresh(ctx); err != nil {
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
