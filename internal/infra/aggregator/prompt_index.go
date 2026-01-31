package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/hashutil"
	"mcpv/internal/infra/mcpcodec"
	"mcpv/internal/infra/telemetry"
)

// PromptIndex aggregates prompt metadata across specs and supports prompt calls.
type PromptIndex struct {
	router               domain.Router
	specs                map[string]domain.ServerSpec
	specKeys             map[string]string
	cfg                  domain.RuntimeConfig
	metadataCache        *domain.MetadataCache
	logger               *zap.Logger
	health               *telemetry.HealthTracker
	gate                 *RefreshGate
	listChanges          listChangeSubscriber
	specsMu              sync.RWMutex
	specKeySet           map[string]struct{}
	bootstrapWaiter      BootstrapWaiter
	bootstrapWaiterMu    sync.RWMutex
	bootstrapRefreshOnce sync.Once
	baseMu               sync.RWMutex
	baseCtx              context.Context
	baseCancel           context.CancelFunc
	serverMu             sync.RWMutex
	serverSnapshots      map[string]serverPromptSnapshot

	reqBuilder requestBuilder
	index      *GenericIndex[domain.PromptSnapshot, domain.PromptTarget, promptCache]
}

type promptCache struct {
	prompts []domain.PromptDefinition
	targets map[string]domain.PromptTarget
	etag    string
}

type serverPromptSnapshot struct {
	snapshot domain.PromptSnapshot
	targets  map[string]domain.PromptTarget
}

// NewPromptIndex builds a PromptIndex for the provided runtime configuration.
func NewPromptIndex(rt domain.Router, specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig, metadataCache *domain.MetadataCache, logger *zap.Logger, health *telemetry.HealthTracker, gate *RefreshGate, listChanges listChangeSubscriber) *PromptIndex {
	if logger == nil {
		logger = zap.NewNop()
	}
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	promptIndex := &PromptIndex{
		router:          rt,
		specs:           specs,
		specKeys:        specKeys,
		cfg:             cfg,
		metadataCache:   metadataCache,
		logger:          logger.Named("prompt_index"),
		health:          health,
		gate:            gate,
		listChanges:     listChanges,
		specKeySet:      specKeySet(specKeys),
		serverSnapshots: map[string]serverPromptSnapshot{},
	}
	promptIndex.index = NewGenericIndex(GenericIndexOptions[domain.PromptSnapshot, domain.PromptTarget, promptCache]{
		Name:              "prompt_index",
		LogLabel:          "prompt",
		FetchErrorMessage: "prompt list fetch failed",
		Specs:             specs,
		Config:            cfg,
		Logger:            promptIndex.logger,
		Health:            health,
		Gate:              gate,
		EmptySnapshot:     func() domain.PromptSnapshot { return domain.PromptSnapshot{} },
		CopySnapshot:      copyPromptSnapshot,
		SnapshotETag:      func(snapshot domain.PromptSnapshot) string { return snapshot.ETag },
		BuildSnapshot:     promptIndex.buildSnapshot,
		CacheETag:         func(cache promptCache) string { return cache.etag },
		Fetch:             promptIndex.fetchServerCache,
		OnRefreshError:    promptIndex.refreshErrorDecision,
		ShouldStart:       func(domain.RuntimeConfig) bool { return true },
	})
	return promptIndex
}

// Start begins periodic refresh and list change tracking.
func (a *PromptIndex) Start(ctx context.Context) {
	baseCtx := a.setBaseContext(ctx)
	a.index.Start(baseCtx)
	a.startListChangeListener(baseCtx)
	a.startBootstrapRefresh(baseCtx)
}

// Stop halts refresh activity and cancels bootstrap waits.
func (a *PromptIndex) Stop() {
	a.index.Stop()
	a.clearBaseContext()
}

// Refresh fetches prompt metadata on demand.
func (a *PromptIndex) Refresh(ctx context.Context) error {
	return a.index.Refresh(ctx)
}

// Snapshot returns the latest prompt snapshot.
func (a *PromptIndex) Snapshot() domain.PromptSnapshot {
	return a.index.Snapshot()
}

// SnapshotForServer returns the latest prompt snapshot for a server.
func (a *PromptIndex) SnapshotForServer(serverName string) (domain.PromptSnapshot, bool) {
	if serverName == "" {
		return domain.PromptSnapshot{}, false
	}
	a.serverMu.RLock()
	entry, ok := a.serverSnapshots[serverName]
	a.serverMu.RUnlock()
	if !ok {
		return domain.PromptSnapshot{}, false
	}
	return domain.ClonePromptSnapshot(entry.snapshot), true
}

// Subscribe streams prompt snapshot updates.
func (a *PromptIndex) Subscribe(ctx context.Context) <-chan domain.PromptSnapshot {
	return a.index.Subscribe(ctx)
}

// Resolve locates a prompt target by name.
func (a *PromptIndex) Resolve(name string) (domain.PromptTarget, bool) {
	return a.index.Resolve(name)
}

// ResolveForServer locates a prompt target for a server by raw prompt name.
func (a *PromptIndex) ResolveForServer(serverName, promptName string) (domain.PromptTarget, bool) {
	if serverName == "" || promptName == "" {
		return domain.PromptTarget{}, false
	}
	a.serverMu.RLock()
	entry, ok := a.serverSnapshots[serverName]
	a.serverMu.RUnlock()
	if !ok {
		return domain.PromptTarget{}, false
	}
	target, ok := entry.targets[promptName]
	return target, ok
}

// SetBootstrapWaiter registers a bootstrap completion hook.
func (a *PromptIndex) SetBootstrapWaiter(waiter BootstrapWaiter) {
	a.bootstrapWaiterMu.Lock()
	a.bootstrapWaiter = waiter
	a.bootstrapWaiterMu.Unlock()
	if baseCtx := a.baseContext(); baseCtx != nil {
		a.startBootstrapRefresh(baseCtx)
	}
}

// GetPrompt routes a prompt request to the owning server.
func (a *PromptIndex) GetPrompt(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	// Wait for bootstrap completion if needed
	a.bootstrapWaiterMu.RLock()
	waiter := a.bootstrapWaiter
	a.bootstrapWaiterMu.RUnlock()

	if waiter != nil {
		// Create context with 60s timeout for bootstrap wait
		waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		if err := waiter(waitCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("bootstrap timeout: %w", err)
			}
			return nil, fmt.Errorf("bootstrap wait failed: %w", err)
		}
	}

	target, ok := a.Resolve(name)
	if !ok {
		return nil, domain.ErrPromptNotFound
	}

	var arguments map[string]string
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, err
		}
	}

	params := &mcp.GetPromptParams{
		Name:      target.PromptName,
		Arguments: arguments,
	}
	payload, err := a.reqBuilder.Build("prompts/get", params)
	if err != nil {
		return nil, err
	}

	resp, err := a.router.Route(ctx, target.ServerType, target.SpecKey, "", payload)
	if err != nil {
		return nil, err
	}

	result, err := decodePromptResult(resp)
	if err != nil {
		return nil, err
	}
	return marshalPromptResult(result)
}

// GetPromptForServer routes a prompt request to the owning server using a raw prompt name.
func (a *PromptIndex) GetPromptForServer(ctx context.Context, serverName, promptName string, args json.RawMessage) (json.RawMessage, error) {
	target, ok := a.ResolveForServer(serverName, promptName)
	if !ok {
		return nil, domain.ErrPromptNotFound
	}

	var arguments map[string]string
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, err
		}
	}

	params := &mcp.GetPromptParams{
		Name:      target.PromptName,
		Arguments: arguments,
	}
	payload, err := a.reqBuilder.Build("prompts/get", params)
	if err != nil {
		return nil, err
	}

	resp, err := a.router.Route(ctx, target.ServerType, target.SpecKey, "", payload)
	if err != nil {
		return nil, err
	}

	result, err := decodePromptResult(resp)
	if err != nil {
		return nil, err
	}
	return marshalPromptResult(result)
}

// UpdateSpecs replaces the registry backing the prompt index.
func (a *PromptIndex) UpdateSpecs(specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig) {
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	specsCopy := copyServerSpecs(specs)
	specKeysCopy := copySpecKeys(specKeys)
	specKeySetCopy := specKeySet(specKeysCopy)

	a.specsMu.Lock()
	a.specs = specsCopy
	a.specKeys = specKeysCopy
	a.specKeySet = specKeySetCopy
	a.cfg = cfg
	a.specsMu.Unlock()
	a.index.UpdateSpecs(specsCopy, cfg)
}

// ApplyRuntimeConfig updates runtime configuration and refresh scheduling.
func (a *PromptIndex) ApplyRuntimeConfig(cfg domain.RuntimeConfig) {
	a.specsMu.Lock()
	prevCfg := a.cfg
	specsCopy := copyServerSpecs(a.specs)
	a.cfg = cfg
	a.specsMu.Unlock()
	a.index.UpdateSpecs(specsCopy, cfg)

	baseCtx := a.baseContext()
	if baseCtx == nil {
		return
	}
	if prevCfg.ToolRefreshInterval() != cfg.ToolRefreshInterval() {
		a.index.Stop()
		a.index.Start(baseCtx)
	}
}

func (a *PromptIndex) startListChangeListener(ctx context.Context) {
	if a.listChanges == nil {
		return
	}
	ch := a.listChanges.Subscribe(ctx, domain.ListChangePrompts)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				a.specsMu.RLock()
				specs := a.specs
				specKeySet := a.specKeySet
				a.specsMu.RUnlock()
				if !listChangeApplies(specs, specKeySet, event) {
					continue
				}
				if err := a.index.Refresh(ctx); err != nil {
					a.logger.Warn("prompt refresh after list change failed", zap.Error(err))
				}
			}
		}
	}()
}

func (a *PromptIndex) startBootstrapRefresh(ctx context.Context) {
	a.bootstrapWaiterMu.RLock()
	waiter := a.bootstrapWaiter
	a.bootstrapWaiterMu.RUnlock()
	if waiter == nil {
		return
	}
	if ctx == nil {
		return
	}
	a.bootstrapRefreshOnce.Do(func() {
		go func() {
			if err := waiter(ctx); err != nil {
				a.logger.Warn("prompt bootstrap wait failed", zap.Error(err))
				return
			}

			a.specsMu.RLock()
			cfg := a.cfg
			a.specsMu.RUnlock()
			refreshCtx, cancel := withRefreshTimeout(ctx, cfg)
			defer cancel()
			if err := a.index.Refresh(refreshCtx); err != nil {
				a.logger.Warn("prompt refresh after bootstrap failed", zap.Error(err))
			}
		}()
	})
}

func (a *PromptIndex) setBaseContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	a.baseMu.Lock()
	if a.baseCtx == nil {
		baseCtx, cancel := context.WithCancel(ctx)
		a.baseCtx = baseCtx
		a.baseCancel = cancel
	}
	baseCtx := a.baseCtx
	a.baseMu.Unlock()
	return baseCtx
}

func (a *PromptIndex) baseContext() context.Context {
	a.baseMu.RLock()
	defer a.baseMu.RUnlock()
	return a.baseCtx
}

func (a *PromptIndex) clearBaseContext() {
	a.baseMu.Lock()
	if a.baseCancel != nil {
		a.baseCancel()
	}
	a.baseCtx = nil
	a.baseCancel = nil
	a.baseMu.Unlock()
}

func (a *PromptIndex) buildSnapshot(cache map[string]promptCache) (domain.PromptSnapshot, map[string]domain.PromptTarget) {
	merged := make([]domain.PromptDefinition, 0)
	targets := make(map[string]domain.PromptTarget)
	serverSnapshots := make(map[string]serverPromptSnapshot, len(cache))
	a.specsMu.RLock()
	specs := a.specs
	strategy := a.cfg.ToolNamespaceStrategy
	a.specsMu.RUnlock()

	serverTypes := sortedServerTypes(cache)
	for _, serverType := range serverTypes {
		server := cache[serverType]
		spec := specs[serverType]
		prompts := append([]domain.PromptDefinition(nil), server.prompts...)
		sort.Slice(prompts, func(i, j int) bool { return prompts[i].Name < prompts[j].Name })

		snapshot := serverPromptSnapshot{
			snapshot: domain.PromptSnapshot{
				ETag:    a.hashPrompts(prompts),
				Prompts: prompts,
			},
			targets: copyPromptTargets(server.targets),
		}
		if spec.Name == "" {
			a.logger.Warn("prompt snapshot skipped: missing server name", zap.String("serverType", serverType))
		} else {
			serverSnapshots[spec.Name] = snapshot
		}

		for _, prompt := range prompts {
			promptDef := prompt
			target := server.targets[prompt.Name]
			displayName := namespacePrompt(serverType, prompt.Name, strategy)
			promptDef.Name = displayName

			if existing, exists := targets[displayName]; exists {
				if strategy != domain.ToolNamespaceStrategyFlat {
					a.logger.Warn("prompt name conflict", zap.String("serverType", serverType), zap.String("prompt", prompt.Name))
					continue
				}
				resolvedName, err := a.resolveFlatConflict(displayName, serverType, targets)
				if err != nil {
					a.logger.Warn("prompt conflict resolution failed", zap.String("serverType", serverType), zap.String("prompt", prompt.Name), zap.Error(err))
					continue
				}
				promptDef = renamePromptDefinition(promptDef, resolvedName)
				target = domain.PromptTarget{
					ServerType: target.ServerType,
					SpecKey:    target.SpecKey,
					PromptName: target.PromptName,
				}
				targets[displayName] = existing
			}

			targets[promptDef.Name] = target
			merged = append(merged, promptDef)
		}
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })

	a.storeServerSnapshots(serverSnapshots)

	return domain.PromptSnapshot{
		ETag:    a.hashPrompts(merged),
		Prompts: merged,
	}, targets
}

func (a *PromptIndex) storeServerSnapshots(snapshots map[string]serverPromptSnapshot) {
	a.serverMu.Lock()
	a.serverSnapshots = snapshots
	a.serverMu.Unlock()
}

func copyPromptTargets(in map[string]domain.PromptTarget) map[string]domain.PromptTarget {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]domain.PromptTarget, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (a *PromptIndex) resolveFlatConflict(name, serverType string, existing map[string]domain.PromptTarget) (string, error) {
	base := fmt.Sprintf("%s_%s", name, serverType)
	if _, ok := existing[base]; !ok {
		return base, nil
	}
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s_%s_%d", name, serverType, i)
		if _, ok := existing[candidate]; !ok {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not resolve conflict for %s", name)
}

func renamePromptDefinition(def domain.PromptDefinition, newName string) domain.PromptDefinition {
	def.Name = newName
	return def
}

func (a *PromptIndex) refreshErrorDecision(_ string, err error) refreshErrorDecision {
	if errors.Is(err, domain.ErrNoReadyInstance) {
		return refreshErrorSkip
	}
	if errors.Is(err, domain.ErrMethodNotAllowed) {
		return refreshErrorDropCache
	}
	return refreshErrorLog
}

func (a *PromptIndex) fetchServerCache(ctx context.Context, serverType string, spec domain.ServerSpec) (promptCache, error) {
	prompts, targets, err := a.fetchServerPrompts(ctx, serverType, spec)
	if err != nil {
		if errors.Is(err, domain.ErrNoReadyInstance) {
			if cached, ok := a.cachedServerCache(serverType, spec); ok {
				return cached, nil
			}
		}
		return promptCache{}, err
	}
	return promptCache{prompts: prompts, targets: targets, etag: a.hashPrompts(prompts)}, nil
}

func (a *PromptIndex) cachedServerCache(serverType string, spec domain.ServerSpec) (promptCache, bool) {
	if a.metadataCache == nil {
		return promptCache{}, false
	}
	specKey := a.specKeys[serverType]
	if specKey == "" {
		return promptCache{}, false
	}
	prompts, ok := a.metadataCache.GetPrompts(specKey)
	if !ok {
		return promptCache{}, false
	}

	result := make([]domain.PromptDefinition, 0, len(prompts))
	targets := make(map[string]domain.PromptTarget)

	for _, prompt := range prompts {
		if prompt.Name == "" {
			continue
		}
		promptDef := prompt
		promptDef.Name = prompt.Name
		promptDef.SpecKey = specKey
		promptDef.ServerName = spec.Name
		result = append(result, promptDef)
		targets[prompt.Name] = domain.PromptTarget{
			ServerType: serverType,
			SpecKey:    specKey,
			PromptName: prompt.Name,
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return promptCache{prompts: result, targets: targets, etag: a.hashPrompts(result)}, true
}

func (a *PromptIndex) fetchServerPrompts(ctx context.Context, serverType string, spec domain.ServerSpec) ([]domain.PromptDefinition, map[string]domain.PromptTarget, error) {
	specKey := a.specKeys[serverType]
	if specKey == "" {
		return nil, nil, fmt.Errorf("missing spec key for server type %q", serverType)
	}
	prompts, err := a.fetchPrompts(ctx, serverType, specKey)
	if err != nil {
		return nil, nil, err
	}

	result := make([]domain.PromptDefinition, 0, len(prompts))
	targets := make(map[string]domain.PromptTarget)

	for _, prompt := range prompts {
		if prompt == nil {
			continue
		}
		if prompt.Name == "" {
			continue
		}
		def := mcpcodec.PromptFromMCP(prompt)
		def.Name = prompt.Name
		def.SpecKey = specKey
		def.ServerName = spec.Name
		result = append(result, def)
		targets[prompt.Name] = domain.PromptTarget{
			ServerType: serverType,
			SpecKey:    specKey,
			PromptName: prompt.Name,
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, targets, nil
}

func (a *PromptIndex) fetchPrompts(ctx context.Context, serverType, specKey string) ([]*mcp.Prompt, error) {
	var prompts []*mcp.Prompt
	cursor := ""

	for {
		params := &mcp.ListPromptsParams{Cursor: cursor}
		payload, err := a.reqBuilder.Build("prompts/list", params)
		if err != nil {
			return nil, err
		}

		resp, err := a.router.RouteWithOptions(ctx, serverType, specKey, "", payload, domain.RouteOptions{AllowStart: false})
		if err != nil {
			return nil, err
		}

		result, err := decodeListPromptsResult(resp)
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, result.Prompts...)
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return prompts, nil
}

func namespacePrompt(serverType, promptName string, strategy domain.ToolNamespaceStrategy) string {
	if strategy == domain.ToolNamespaceStrategyFlat {
		return promptName
	}
	return fmt.Sprintf("%s.%s", serverType, promptName)
}

func decodeListPromptsResult(raw json.RawMessage) (*mcp.ListPromptsResult, error) {
	resp, err := decodeJSONRPCResponse(raw)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("prompts/list error: %w", resp.Error)
	}

	if len(resp.Result) == 0 {
		return nil, errors.New("prompts/list response missing result")
	}

	var result mcp.ListPromptsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode prompts/list result: %w", err)
	}
	return &result, nil
}

func decodePromptResult(raw json.RawMessage) (*mcp.GetPromptResult, error) {
	resp, err := decodeJSONRPCResponse(raw)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("prompts/get error: %w", resp.Error)
	}

	if len(resp.Result) == 0 {
		return nil, errors.New("prompts/get response missing result")
	}

	var result mcp.GetPromptResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode prompts/get result: %w", err)
	}
	return &result, nil
}

func marshalPromptResult(result *mcp.GetPromptResult) (json.RawMessage, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func (a *PromptIndex) hashPrompts(prompts []domain.PromptDefinition) string {
	return hashutil.PromptETag(a.logger, prompts)
}

func copyPromptSnapshot(snapshot domain.PromptSnapshot) domain.PromptSnapshot {
	return domain.ClonePromptSnapshot(snapshot)
}
