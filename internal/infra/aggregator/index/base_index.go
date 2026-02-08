package index

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/aggregator/core"
	"mcpv/internal/infra/telemetry"
)

type BaseHooks[Snapshot any, Target any, Cache any] struct {
	Name              string
	LogLabel          string
	LoggerName        string
	FetchErrorMessage string
	ListChangeKind    domain.ListChangeKind
	ShouldStart       func(cfg domain.RuntimeConfig) bool
	ShouldListChange  func(cfg domain.RuntimeConfig) bool
	EmptySnapshot     func() Snapshot
	CopySnapshot      func(Snapshot) Snapshot
	SnapshotETag      func(Snapshot) string
	BuildSnapshot     func(cache map[string]Cache) (Snapshot, map[string]Target)
	CacheETag         func(cache Cache) string
	FetchServerCache  func(ctx context.Context, serverType string, spec domain.ServerSpec) (Cache, error)
	OnRefreshError    func(serverType string, err error) core.RefreshErrorDecision
}

type BaseIndex[Snapshot any, Target any, Cache any, ServerSnapshot any] struct {
	router        domain.Router
	specs         map[string]domain.ServerSpec
	specKeys      map[string]string
	cfg           domain.RuntimeConfig
	metadataCache *domain.MetadataCache
	logger        *zap.Logger
	health        *telemetry.HealthTracker
	gate          *core.RefreshGate
	listChanges   core.ListChangeSubscriber
	hooks         BaseHooks[Snapshot, Target, Cache]

	// Lock ordering (only if multiple locks are ever held): specsMu -> bootstrapMu -> baseMu -> serverMu.
	// Prefer single-lock sections and avoid holding any lock while calling external components.
	// specsMu guards specs/specKeys/cfg/specKeySet.
	specsMu         sync.RWMutex
	specKeySet      map[string]struct{}
	bootstrapWaiter core.BootstrapWaiter
	// bootstrapMu guards bootstrapWaiter/bootstrapOnce.
	bootstrapMu   sync.RWMutex
	bootstrapOnce sync.Once
	// baseMu guards baseCtx/baseCancel.
	baseMu     sync.RWMutex
	baseCtx    context.Context
	baseCancel context.CancelFunc
	// serverMu guards serverSnapshots.
	serverMu        sync.RWMutex
	serverSnapshots map[string]ServerSnapshot

	index *core.GenericIndex[Snapshot, Target, Cache]
}

func NewBaseIndex[Snapshot any, Target any, Cache any, ServerSnapshot any](
	router domain.Router,
	specs map[string]domain.ServerSpec,
	specKeys map[string]string,
	cfg domain.RuntimeConfig,
	metadataCache *domain.MetadataCache,
	logger *zap.Logger,
	health *telemetry.HealthTracker,
	gate *core.RefreshGate,
	listChanges core.ListChangeSubscriber,
	hooks BaseHooks[Snapshot, Target, Cache],
) *BaseIndex[Snapshot, Target, Cache, ServerSnapshot] {
	if logger == nil {
		logger = zap.NewNop()
	}
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	if hooks.ShouldStart == nil {
		hooks.ShouldStart = func(domain.RuntimeConfig) bool { return true }
	}
	if hooks.ShouldListChange == nil {
		hooks.ShouldListChange = func(domain.RuntimeConfig) bool { return true }
	}
	namedLogger := logger
	if hooks.LoggerName != "" {
		namedLogger = logger.Named(hooks.LoggerName)
	} else if hooks.LogLabel != "" {
		namedLogger = logger.Named(hooks.LogLabel)
	}
	idx := &BaseIndex[Snapshot, Target, Cache, ServerSnapshot]{
		router:          router,
		specs:           specs,
		specKeys:        specKeys,
		cfg:             cfg,
		metadataCache:   metadataCache,
		logger:          namedLogger,
		health:          health,
		gate:            gate,
		listChanges:     listChanges,
		hooks:           hooks,
		specKeySet:      core.SpecKeySet(specKeys),
		serverSnapshots: map[string]ServerSnapshot{},
	}
	idx.index = core.NewGenericIndex(core.GenericIndexOptions[Snapshot, Target, Cache]{
		Name:              hooks.Name,
		LogLabel:          hooks.LogLabel,
		FetchErrorMessage: hooks.FetchErrorMessage,
		Specs:             specs,
		Config:            cfg,
		Logger:            idx.logger,
		Health:            health,
		Gate:              gate,
		EmptySnapshot:     hooks.EmptySnapshot,
		CopySnapshot:      hooks.CopySnapshot,
		SnapshotETag:      hooks.SnapshotETag,
		BuildSnapshot:     hooks.BuildSnapshot,
		CacheETag:         hooks.CacheETag,
		Fetch:             hooks.FetchServerCache,
		OnRefreshError:    hooks.OnRefreshError,
		ShouldStart:       hooks.ShouldStart,
	})
	return idx
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Start(ctx context.Context) {
	baseCtx := b.setBaseContext(ctx)
	b.index.Start(baseCtx)
	b.startListChangeListener(baseCtx)
	b.startBootstrapRefresh(baseCtx)
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Stop() {
	b.index.Stop()
	b.clearBaseContext()
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Refresh(ctx context.Context) error {
	return b.index.Refresh(ctx)
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Snapshot() Snapshot {
	return b.index.Snapshot()
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Subscribe(ctx context.Context) <-chan Snapshot {
	return b.index.Subscribe(ctx)
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Resolve(key string) (Target, bool) {
	return b.index.Resolve(key)
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) UpdateSpecs(specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig) {
	specsCopy := core.CopyServerSpecs(specs)
	specKeysCopy := core.CopySpecKeys(specKeys)
	specKeySetCopy := core.SpecKeySet(specKeysCopy)

	b.specsMu.Lock()
	prevCfg := b.cfg
	b.specs = specsCopy
	b.specKeys = specKeysCopy
	b.specKeySet = specKeySetCopy
	b.cfg = cfg
	b.specsMu.Unlock()

	b.index.UpdateSpecs(specsCopy, cfg)

	baseCtx := b.baseContext()
	if baseCtx == nil {
		return
	}
	if prevCfg.ToolRefreshInterval() != cfg.ToolRefreshInterval() || b.hooks.ShouldStart(prevCfg) != b.hooks.ShouldStart(cfg) {
		b.index.Stop()
		b.index.Start(baseCtx)
	}
	if !b.hooks.ShouldListChange(prevCfg) && b.hooks.ShouldListChange(cfg) {
		b.startListChangeListener(baseCtx)
	}
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) ApplyRuntimeConfig(cfg domain.RuntimeConfig) {
	b.specsMu.Lock()
	prevCfg := b.cfg
	specsCopy := core.CopyServerSpecs(b.specs)
	b.cfg = cfg
	b.specsMu.Unlock()

	b.index.UpdateSpecs(specsCopy, cfg)

	baseCtx := b.baseContext()
	if baseCtx == nil {
		return
	}
	if prevCfg.ToolRefreshInterval() != cfg.ToolRefreshInterval() || b.hooks.ShouldStart(prevCfg) != b.hooks.ShouldStart(cfg) {
		b.index.Stop()
		b.index.Start(baseCtx)
	}
	if !b.hooks.ShouldListChange(prevCfg) && b.hooks.ShouldListChange(cfg) {
		b.startListChangeListener(baseCtx)
	}
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) SetBootstrapWaiter(waiter core.BootstrapWaiter) {
	b.bootstrapMu.Lock()
	b.bootstrapWaiter = waiter
	b.bootstrapMu.Unlock()
	if baseCtx := b.baseContext(); baseCtx != nil {
		b.startBootstrapRefresh(baseCtx)
	}
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) startListChangeListener(ctx context.Context) {
	if b.listChanges == nil {
		return
	}
	b.specsMu.RLock()
	shouldListChange := b.hooks.ShouldListChange(b.cfg)
	b.specsMu.RUnlock()
	if !shouldListChange {
		return
	}
	if b.hooks.ListChangeKind == "" {
		return
	}
	ch := b.listChanges.Subscribe(ctx, b.hooks.ListChangeKind)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				b.specsMu.RLock()
				shouldListChange := b.hooks.ShouldListChange(b.cfg)
				specs := b.specs
				specKeySet := b.specKeySet
				b.specsMu.RUnlock()
				if !shouldListChange {
					continue
				}
				if !core.ListChangeApplies(specs, specKeySet, event) {
					continue
				}
				if err := b.index.Refresh(ctx); err != nil {
					b.logger.Warn(b.hooks.LogLabel+" refresh after list change failed", zap.Error(err))
				}
			}
		}
	}()
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) startBootstrapRefresh(ctx context.Context) {
	b.bootstrapMu.RLock()
	waiter := b.bootstrapWaiter
	b.bootstrapMu.RUnlock()
	if waiter == nil {
		return
	}
	if ctx == nil {
		return
	}
	b.bootstrapOnce.Do(func() {
		go func() {
			if err := waiter(ctx); err != nil {
				b.logger.Warn(b.hooks.LogLabel+" bootstrap wait failed", zap.Error(err))
				return
			}

			b.specsMu.RLock()
			cfg := b.cfg
			b.specsMu.RUnlock()
			refreshCtx, cancel := core.WithRefreshTimeout(ctx, cfg)
			defer cancel()
			if err := b.index.Refresh(refreshCtx); err != nil {
				b.logger.Warn(b.hooks.LogLabel+" refresh after bootstrap failed", zap.Error(err))
			}
		}()
	})
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) setBaseContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	b.baseMu.Lock()
	if b.baseCtx == nil {
		baseCtx, cancel := context.WithCancel(ctx)
		b.baseCtx = baseCtx
		b.baseCancel = cancel
	}
	baseCtx := b.baseCtx
	b.baseMu.Unlock()
	return baseCtx
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) baseContext() context.Context {
	b.baseMu.RLock()
	defer b.baseMu.RUnlock()
	return b.baseCtx
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) clearBaseContext() {
	b.baseMu.Lock()
	if b.baseCancel != nil {
		b.baseCancel()
	}
	b.baseCtx = nil
	b.baseCancel = nil
	b.baseMu.Unlock()
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) SnapshotForServer(serverName string) (ServerSnapshot, bool) {
	if serverName == "" {
		var zero ServerSnapshot
		return zero, false
	}
	b.serverMu.RLock()
	entry, ok := b.serverSnapshots[serverName]
	b.serverMu.RUnlock()
	return entry, ok
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) StoreServerSnapshots(snapshots map[string]ServerSnapshot) {
	b.serverMu.Lock()
	b.serverSnapshots = snapshots
	b.serverMu.Unlock()
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) SpecsSnapshot() (map[string]domain.ServerSpec, map[string]string, domain.RuntimeConfig) {
	b.specsMu.RLock()
	defer b.specsMu.RUnlock()
	return b.specs, b.specKeys, b.cfg
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) SpecKeySet() map[string]struct{} {
	b.specsMu.RLock()
	defer b.specsMu.RUnlock()
	return b.specKeySet
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) MetadataCache() *domain.MetadataCache {
	return b.metadataCache
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Logger() *zap.Logger {
	return b.logger
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Router() domain.Router {
	return b.router
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Gate() *core.RefreshGate {
	return b.gate
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Health() *telemetry.HealthTracker {
	return b.health
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) Waiter() core.BootstrapWaiter {
	b.bootstrapMu.RLock()
	defer b.bootstrapMu.RUnlock()
	return b.bootstrapWaiter
}

func (b *BaseIndex[Snapshot, Target, Cache, ServerSnapshot]) ClockNow() time.Time {
	return time.Now()
}
