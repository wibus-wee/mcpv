package aggregator

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/telemetry"
)

type refreshErrorDecision int

const (
	refreshErrorLog refreshErrorDecision = iota
	refreshErrorSkip
	refreshErrorDropCache
)

type GenericIndexOptions[Snapshot any, Target any, Cache any] struct {
	Name              string
	LogLabel          string
	FetchErrorMessage string
	HeartbeatName     string
	Specs             map[string]domain.ServerSpec
	Config            domain.RuntimeConfig
	Logger            *zap.Logger
	Health            *telemetry.HealthTracker
	Gate              *RefreshGate
	EmptySnapshot     func() Snapshot
	CopySnapshot      func(Snapshot) Snapshot
	SnapshotETag      func(Snapshot) string
	BuildSnapshot     func(cache map[string]Cache) (Snapshot, map[string]Target)
	Fetch             func(ctx context.Context, serverType string, spec domain.ServerSpec) (Cache, error)
	OnRefreshError    func(serverType string, err error) refreshErrorDecision
	ShouldStart       func(cfg domain.RuntimeConfig) bool
}

type genericIndexState[Snapshot any, Target any] struct {
	snapshot Snapshot
	targets  map[string]Target
}

type GenericIndex[Snapshot any, Target any, Cache any] struct {
	name              string
	logLabel          string
	fetchErrorMessage string
	heartbeatName     string
	specs             map[string]domain.ServerSpec
	cfg               domain.RuntimeConfig
	logger            *zap.Logger
	health            *telemetry.HealthTracker
	gate              *RefreshGate
	emptySnapshot     func() Snapshot
	copySnapshot      func(Snapshot) Snapshot
	snapshotETag      func(Snapshot) string
	buildSnapshot     func(cache map[string]Cache) (Snapshot, map[string]Target)
	fetch             func(ctx context.Context, serverType string, spec domain.ServerSpec) (Cache, error)
	onRefreshError    func(serverType string, err error) refreshErrorDecision
	shouldStart       func(cfg domain.RuntimeConfig) bool

	mu          sync.Mutex
	started     bool
	ticker      *time.Ticker
	stop        chan struct{}
	serverCache map[string]Cache
	subs        map[chan Snapshot]struct{}
	refreshBeat *telemetry.Heartbeat
	state       atomic.Value
}

func NewGenericIndex[Snapshot any, Target any, Cache any](opts GenericIndexOptions[Snapshot, Target, Cache]) *GenericIndex[Snapshot, Target, Cache] {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	shouldStart := opts.ShouldStart
	if shouldStart == nil {
		shouldStart = func(domain.RuntimeConfig) bool { return true }
	}
	heartbeatName := opts.HeartbeatName
	if heartbeatName == "" {
		heartbeatName = opts.Name + ".refresh"
	}
	g := &GenericIndex[Snapshot, Target, Cache]{
		name:              opts.Name,
		logLabel:          opts.LogLabel,
		fetchErrorMessage: opts.FetchErrorMessage,
		heartbeatName:     heartbeatName,
		specs:             opts.Specs,
		cfg:               opts.Config,
		logger:            logger,
		health:            opts.Health,
		gate:              opts.Gate,
		emptySnapshot:     opts.EmptySnapshot,
		copySnapshot:      opts.CopySnapshot,
		snapshotETag:      opts.SnapshotETag,
		buildSnapshot:     opts.BuildSnapshot,
		fetch:             opts.Fetch,
		onRefreshError:    opts.OnRefreshError,
		shouldStart:       shouldStart,
		stop:              make(chan struct{}),
		serverCache:       make(map[string]Cache),
		subs:              make(map[chan Snapshot]struct{}),
	}
	g.state.Store(genericIndexState[Snapshot, Target]{
		snapshot: opts.EmptySnapshot(),
		targets:  make(map[string]Target),
	})
	return g
}

func (g *GenericIndex[Snapshot, Target, Cache]) Start(ctx context.Context) {
	if !g.shouldStart(g.cfg) {
		return
	}

	g.mu.Lock()
	if g.started {
		g.mu.Unlock()
		return
	}
	g.started = true
	if g.stop == nil {
		g.stop = make(chan struct{})
	}
	g.mu.Unlock()

	interval := time.Duration(g.cfg.ToolRefreshSeconds) * time.Second
	if interval > 0 && g.health != nil && g.refreshBeat == nil {
		g.refreshBeat = g.health.Register(g.heartbeatName, interval*3)
	}
	if g.refreshBeat != nil {
		g.refreshBeat.Beat()
	}
	if err := g.Refresh(ctx); err != nil {
		g.logger.Warn("initial "+g.logLabel+" refresh failed", zap.Error(err))
	}
	if interval <= 0 {
		return
	}

	g.mu.Lock()
	if g.ticker != nil {
		g.mu.Unlock()
		return
	}
	g.ticker = time.NewTicker(interval)
	g.mu.Unlock()

	go func() {
		for {
			select {
			case <-g.ticker.C:
				if g.refreshBeat != nil {
					g.refreshBeat.Beat()
				}
				if err := g.Refresh(ctx); err != nil {
					g.logger.Warn(g.logLabel+" refresh failed", zap.Error(err))
				}
			case <-g.stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (g *GenericIndex[Snapshot, Target, Cache]) Stop() {
	g.mu.Lock()
	if g.ticker != nil {
		g.ticker.Stop()
		g.ticker = nil
	}
	if g.refreshBeat != nil {
		g.refreshBeat.Stop()
		g.refreshBeat = nil
	}
	if g.stop != nil {
		close(g.stop)
		g.stop = nil
	}
	g.started = false
	g.mu.Unlock()
}

func (g *GenericIndex[Snapshot, Target, Cache]) Snapshot() Snapshot {
	state := g.state.Load().(genericIndexState[Snapshot, Target])
	return g.copySnapshot(state.snapshot)
}

func (g *GenericIndex[Snapshot, Target, Cache]) Subscribe(ctx context.Context) <-chan Snapshot {
	ch := make(chan Snapshot, 1)

	g.mu.Lock()
	g.subs[ch] = struct{}{}
	g.mu.Unlock()

	state := g.state.Load().(genericIndexState[Snapshot, Target])
	g.sendSnapshot(ch, state.snapshot)

	go func() {
		<-ctx.Done()
		g.mu.Lock()
		delete(g.subs, ch)
		g.mu.Unlock()
	}()

	return ch
}

func (g *GenericIndex[Snapshot, Target, Cache]) Resolve(key string) (Target, bool) {
	state := g.state.Load().(genericIndexState[Snapshot, Target])
	target, ok := state.targets[key]
	return target, ok
}

func (g *GenericIndex[Snapshot, Target, Cache]) Refresh(ctx context.Context) error {
	if err := g.gate.Acquire(ctx); err != nil {
		return err
	}
	defer g.gate.Release()

	serverTypes := sortedServerTypes(g.specs)
	if len(serverTypes) == 0 {
		return nil
	}

	type refreshResult struct {
		serverType string
		cache      Cache
		err        error
	}

	results := make(chan refreshResult, len(serverTypes))
	timeout := refreshTimeout(g.cfg)
	workerCount := refreshWorkerCount(g.cfg, len(serverTypes))
	if workerCount == 0 {
		return nil
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case serverType, ok := <-jobs:
					if !ok {
						return
					}
					spec := g.specs[serverType]
					fetchCtx, cancel := context.WithTimeout(ctx, timeout)
					cache, err := g.fetch(fetchCtx, serverType, spec)
					cancel()
					results <- refreshResult{
						serverType: serverType,
						cache:      cache,
						err:        err,
					}
				}
			}
		}()
	}

	go func() {
		for _, serverType := range serverTypes {
			jobs <- serverType
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.err != nil {
			decision := refreshErrorLog
			if g.onRefreshError != nil {
				decision = g.onRefreshError(res.serverType, res.err)
			}
			switch decision {
			case refreshErrorSkip:
				continue
			case refreshErrorDropCache:
				g.deleteCache(res.serverType)
				g.rebuildSnapshot()
				continue
			default:
				g.logger.Warn(g.fetchErrorMessage, zap.String("serverType", res.serverType), zap.Error(res.err))
				continue
			}
		}

		g.mu.Lock()
		g.serverCache[res.serverType] = res.cache
		g.mu.Unlock()
		g.rebuildSnapshot()
	}
	return nil
}

func (g *GenericIndex[Snapshot, Target, Cache]) deleteCache(serverType string) {
	g.mu.Lock()
	delete(g.serverCache, serverType)
	g.mu.Unlock()
}

func (g *GenericIndex[Snapshot, Target, Cache]) rebuildSnapshot() {
	cache := g.copyServerCache()
	snapshot, targets := g.buildSnapshot(cache)
	etag := g.snapshotETag(snapshot)
	state := g.state.Load().(genericIndexState[Snapshot, Target])
	if etag == g.snapshotETag(state.snapshot) {
		return
	}

	g.state.Store(genericIndexState[Snapshot, Target]{
		snapshot: snapshot,
		targets:  targets,
	})
	g.broadcast(snapshot)
}

func (g *GenericIndex[Snapshot, Target, Cache]) broadcast(snapshot Snapshot) {
	subs := g.copySubscribers()
	for _, ch := range subs {
		g.sendSnapshot(ch, snapshot)
	}
}

func (g *GenericIndex[Snapshot, Target, Cache]) copyServerCache() map[string]Cache {
	g.mu.Lock()
	defer g.mu.Unlock()

	out := make(map[string]Cache, len(g.serverCache))
	for key, cache := range g.serverCache {
		out[key] = cache
	}
	return out
}

func (g *GenericIndex[Snapshot, Target, Cache]) copySubscribers() []chan Snapshot {
	g.mu.Lock()
	defer g.mu.Unlock()

	out := make([]chan Snapshot, 0, len(g.subs))
	for ch := range g.subs {
		out = append(out, ch)
	}
	return out
}

func (g *GenericIndex[Snapshot, Target, Cache]) sendSnapshot(ch chan Snapshot, snapshot Snapshot) {
	select {
	case ch <- g.copySnapshot(snapshot):
	default:
	}
}
