package catalog

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	infraCatalog "mcpv/internal/infra/catalog"
)

const defaultReloadDebounce = 200 * time.Millisecond

// DynamicCatalogProvider loads and watches catalog updates.
type DynamicCatalogProvider struct {
	logger      *zap.Logger
	loader      *infraCatalog.Loader
	configPath  string
	allowCreate bool

	state    atomic.Value
	revision atomic.Uint64

	subsMu sync.Mutex
	subs   map[chan domain.CatalogUpdate]struct{}

	reloadMu  sync.Mutex
	watchOnce sync.Once
	watchCtx  context.Context
}

// NewDynamicCatalogProvider loads a catalog and watches for updates.
func NewDynamicCatalogProvider(ctx context.Context, configPath string, logger *zap.Logger) (*DynamicCatalogProvider, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	loader := infraCatalog.NewLoader(logger)
	catalogData, err := loader.Load(ctx, configPath)
	if err != nil {
		return nil, err
	}
	state, err := domain.NewCatalogState(catalogData, 1, time.Now())
	if err != nil {
		return nil, err
	}

	provider := &DynamicCatalogProvider{
		logger:      logger.Named("catalog_provider"),
		loader:      loader,
		configPath:  configPath,
		allowCreate: false,
		subs:        make(map[chan domain.CatalogUpdate]struct{}),
		watchCtx:    ctx,
	}
	provider.state.Store(state)
	provider.revision.Store(state.Revision)
	return provider, nil
}

// Snapshot returns the current catalog snapshot.
func (p *DynamicCatalogProvider) Snapshot(ctx context.Context) (domain.CatalogState, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return domain.CatalogState{}, err
		}
	}
	state := p.state.Load().(domain.CatalogState)
	return state, nil
}

// Watch subscribes to catalog updates.
func (p *DynamicCatalogProvider) Watch(ctx context.Context) (<-chan domain.CatalogUpdate, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ch := make(chan domain.CatalogUpdate, 1)
	p.subsMu.Lock()
	p.subs[ch] = struct{}{}
	p.subsMu.Unlock()

	p.watchOnce.Do(func() {
		go p.runWatcher(p.watchCtx)
	})

	go func() {
		<-ctx.Done()
		p.subsMu.Lock()
		delete(p.subs, ch)
		p.subsMu.Unlock()
	}()

	return ch, nil
}

// Reload forces a catalog reload.
func (p *DynamicCatalogProvider) Reload(ctx context.Context) error {
	return p.reload(ctx, domain.CatalogUpdateSourceManual)
}

func (p *DynamicCatalogProvider) reload(ctx context.Context, source domain.CatalogUpdateSource) error {
	p.reloadMu.Lock()
	defer p.reloadMu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}

	prev := p.state.Load().(domain.CatalogState)
	catalogData, err := p.loader.Load(ctx, p.configPath)
	if err != nil {
		return err
	}

	nextRevision := p.revision.Load() + 1
	next, err := domain.NewCatalogState(catalogData, nextRevision, time.Now())
	if err != nil {
		return err
	}

	diff := domain.DiffCatalogStates(prev, next)
	if diff.IsEmpty() {
		return nil
	}

	p.revision.Store(nextRevision)
	p.state.Store(next)
	p.broadcast(domain.CatalogUpdate{
		Snapshot: next,
		Diff:     diff,
		Source:   source,
	})
	return nil
}

func (p *DynamicCatalogProvider) broadcast(update domain.CatalogUpdate) {
	subs := p.copySubscribers()
	for _, ch := range subs {
		select {
		case ch <- update:
		default:
		}
	}
}

func (p *DynamicCatalogProvider) copySubscribers() []chan domain.CatalogUpdate {
	p.subsMu.Lock()
	defer p.subsMu.Unlock()

	out := make([]chan domain.CatalogUpdate, 0, len(p.subs))
	for ch := range p.subs {
		out = append(out, ch)
	}
	return out
}

func (p *DynamicCatalogProvider) runWatcher(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		p.logger.Warn("config watcher failed", zap.Error(err))
		return
	}
	defer watcher.Close()

	paths := p.watchPaths()
	for _, path := range paths {
		if err := watcher.Add(path); err != nil {
			p.logger.Warn("config watcher add failed", zap.String("path", path), zap.Error(err))
		}
	}

	var timer *time.Timer
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-watcher.Errors:
			if err != nil {
				p.logger.Warn("config watcher error", zap.Error(err))
			}
		case event := <-watcher.Events:
			if !shouldReloadForPath(event.Name, p.configPath) {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(defaultReloadDebounce)
				continue
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(defaultReloadDebounce)
		case <-timerChan(timer):
			timer = nil
			if err := p.reload(ctx, domain.CatalogUpdateSourceWatch); err != nil {
				p.logger.Warn("config reload failed", zap.Error(err))
			}
		}
	}
}

func (p *DynamicCatalogProvider) watchPaths() []string {
	return []string{filepath.Dir(p.configPath)}
}

func shouldReloadForPath(path string, configPath string) bool {
	if path == "" || configPath == "" {
		return false
	}
	return filepath.Clean(path) == filepath.Clean(configPath)
}

func timerChan(timer *time.Timer) <-chan time.Time {
	if timer == nil {
		return nil
	}
	return timer.C
}
