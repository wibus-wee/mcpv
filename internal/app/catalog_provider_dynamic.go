package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/catalog"
)

const defaultReloadDebounce = 200 * time.Millisecond

type DynamicCatalogProvider struct {
	logger      *zap.Logger
	loader      *catalog.ProfileStoreLoader
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

func NewDynamicCatalogProvider(ctx context.Context, cfg ServeConfig, logger *zap.Logger) (*DynamicCatalogProvider, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	loader := catalog.NewProfileStoreLoader(logger)
	store, err := loader.Load(ctx, cfg.ConfigPath, catalog.ProfileStoreOptions{
		AllowCreate: true,
	})
	if err != nil {
		return nil, err
	}
	state, err := domain.NewCatalogState(store, 1, time.Now())
	if err != nil {
		return nil, err
	}

	provider := &DynamicCatalogProvider{
		logger:      logger.Named("catalog_provider"),
		loader:      loader,
		configPath:  cfg.ConfigPath,
		allowCreate: true,
		subs:        make(map[chan domain.CatalogUpdate]struct{}),
		watchCtx:    ctx,
	}
	provider.state.Store(state)
	provider.revision.Store(state.Revision)
	return provider, nil
}

func (p *DynamicCatalogProvider) Snapshot(ctx context.Context) (domain.CatalogState, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return domain.CatalogState{}, err
		}
	}
	state := p.state.Load().(domain.CatalogState)
	return state, nil
}

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
	store, err := p.loader.Load(ctx, p.configPath, catalog.ProfileStoreOptions{
		AllowCreate: p.allowCreate,
	})
	if err != nil {
		return err
	}

	nextRevision := p.revision.Load() + 1
	next, err := domain.NewCatalogState(store, nextRevision, time.Now())
	if err != nil {
		return err
	}

	if prev.Revision > 0 && prev.Summary.DefaultRuntime != next.Summary.DefaultRuntime {
		return errors.New("runtime config changed; restart required to apply")
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
			if !shouldReloadForPath(event.Name) {
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
	paths := []string{p.configPath}
	paths = append(paths, filepath.Join(p.configPath, "profiles"))
	return paths
}

func shouldReloadForPath(path string) bool {
	if path == "" {
		return false
	}
	base := filepath.Base(path)
	if base == "runtime.yaml" || base == "runtime.yml" || base == "callers.yaml" {
		return true
	}
	if filepath.Base(filepath.Dir(path)) == "profiles" {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".yaml" || ext == ".yml"
	}
	return false
}

func timerChan(timer *time.Timer) <-chan time.Time {
	if timer == nil {
		return nil
	}
	return timer.C
}
