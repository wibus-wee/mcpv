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

type ReloadManager struct {
	provider    domain.CatalogProvider
	state       *controlPlaneState
	registry    *callerRegistry
	scheduler   domain.Scheduler
	initManager *ServerInitializationManager
	metrics     domain.Metrics
	health      *telemetry.HealthTracker
	listChanges *notifications.ListChangeHub
	coreLogger  *zap.Logger
	logger      *zap.Logger
	appliedRev  atomic.Uint64
	started     atomic.Bool
}

func NewReloadManager(
	provider domain.CatalogProvider,
	state *controlPlaneState,
	registry *callerRegistry,
	scheduler domain.Scheduler,
	initManager *ServerInitializationManager,
	metrics domain.Metrics,
	health *telemetry.HealthTracker,
	listChanges *notifications.ListChangeHub,
	logger *zap.Logger,
) *ReloadManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ReloadManager{
		provider:    provider,
		state:       state,
		registry:    registry,
		scheduler:   scheduler,
		initManager: initManager,
		metrics:     metrics,
		health:      health,
		listChanges: listChanges,
		coreLogger:  logger,
		logger:      logger.Named("reload"),
	}
}

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
	prevProfiles := m.state.Profiles()
	nextProfiles := make(map[string]*profileRuntime, len(update.Snapshot.Summary.Profiles))
	removedRuntimes := make([]*profileRuntime, 0)

	for name, cfg := range update.Snapshot.Summary.Profiles {
		if runtime, ok := prevProfiles[name]; ok {
			runtime.UpdateCatalog(cfg)
			nextProfiles[name] = runtime
			continue
		}
		nextProfiles[name] = buildProfileRuntime(name, cfg, m.scheduler, m.metrics, m.health, m.listChanges, m.coreLogger)
	}

	var removed []string
	for name, runtime := range prevProfiles {
		if _, ok := nextProfiles[name]; ok {
			continue
		}
		removed = append(removed, name)
		removedRuntimes = append(removedRuntimes, runtime)
	}

	if err := m.scheduler.ApplyCatalogDiff(ctx, update.Diff, update.Snapshot.Summary.SpecRegistry); err != nil {
		return err
	}
	if m.initManager != nil {
		m.initManager.ApplyCatalogState(&update.Snapshot)
	}

	m.state.UpdateCatalog(&update.Snapshot, nextProfiles)

	if err := m.registry.ApplyCatalogUpdate(ctx, update); err != nil {
		return err
	}

	m.refreshProfiles(ctx, update, nextProfiles)
	for _, runtime := range removedRuntimes {
		runtime.Deactivate()
	}

	m.logger.Info("config reload applied",
		zap.Uint64("revision", update.Snapshot.Revision),
		zap.Int("profiles", len(update.Snapshot.Summary.Profiles)),
		zap.Int("servers", update.Snapshot.Summary.TotalServers),
		zap.Int("added", len(update.Diff.AddedSpecKeys)),
		zap.Int("removed", len(update.Diff.RemovedSpecKeys)),
		zap.Int("updated", len(update.Diff.UpdatedSpecKeys)),
		zap.Duration("latency", time.Since(started)),
	)
	if len(removed) > 0 {
		m.logger.Info("profiles removed", zap.Strings("profiles", removed))
	}
	m.appliedRev.Store(update.Snapshot.Revision)
	return nil
}

func (m *ReloadManager) refreshProfiles(ctx context.Context, update domain.CatalogUpdate, profiles map[string]*profileRuntime) {
	changed := make(map[string]struct{})
	for _, name := range update.Diff.AddedProfiles {
		changed[name] = struct{}{}
	}
	for _, name := range update.Diff.UpdatedProfiles {
		changed[name] = struct{}{}
	}

	for name := range changed {
		runtime, ok := profiles[name]
		if !ok {
			continue
		}
		if runtime.tools != nil {
			if err := runtime.tools.Refresh(ctx); err != nil {
				m.logger.Warn("tool refresh after reload failed", zap.String("profile", name), zap.Error(err))
			}
		}
		if runtime.resources != nil {
			if err := runtime.resources.Refresh(ctx); err != nil {
				m.logger.Warn("resource refresh after reload failed", zap.String("profile", name), zap.Error(err))
			}
		}
		if runtime.prompts != nil {
			if err := runtime.prompts.Refresh(ctx); err != nil {
				m.logger.Warn("prompt refresh after reload failed", zap.String("profile", name), zap.Error(err))
			}
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
