package aggregator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/telemetry"
)

type PromptIndex struct {
	router   domain.Router
	specs    map[string]domain.ServerSpec
	specKeys map[string]string
	cfg      domain.RuntimeConfig
	logger   *zap.Logger
	health   *telemetry.HealthTracker
	gate     *RefreshGate

	mu          sync.Mutex
	started     bool
	ticker      *time.Ticker
	stop        chan struct{}
	serverCache map[string]promptCache
	subs        map[chan domain.PromptSnapshot]struct{}
	refreshBeat *telemetry.Heartbeat
	state       atomic.Value
	reqBuilder  requestBuilder
}

type promptCache struct {
	prompts []domain.PromptDefinition
	targets map[string]domain.PromptTarget
}

type promptIndexState struct {
	snapshot domain.PromptSnapshot
	targets  map[string]domain.PromptTarget
}

func NewPromptIndex(rt domain.Router, specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig, logger *zap.Logger, health *telemetry.HealthTracker, gate *RefreshGate) *PromptIndex {
	if logger == nil {
		logger = zap.NewNop()
	}
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	index := &PromptIndex{
		router:      rt,
		specs:       specs,
		specKeys:    specKeys,
		cfg:         cfg,
		logger:      logger.Named("prompt_index"),
		health:      health,
		gate:        gate,
		stop:        make(chan struct{}),
		serverCache: make(map[string]promptCache),
		subs:        make(map[chan domain.PromptSnapshot]struct{}),
	}
	index.state.Store(promptIndexState{
		snapshot: domain.PromptSnapshot{},
		targets:  make(map[string]domain.PromptTarget),
	})
	return index
}

func (a *PromptIndex) Start(ctx context.Context) {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return
	}
	a.started = true
	if a.stop == nil {
		a.stop = make(chan struct{})
	}
	a.mu.Unlock()

	interval := time.Duration(a.cfg.ToolRefreshSeconds) * time.Second
	if interval > 0 && a.health != nil && a.refreshBeat == nil {
		a.refreshBeat = a.health.Register("prompt_index.refresh", interval*3)
	}
	if a.refreshBeat != nil {
		a.refreshBeat.Beat()
	}
	if err := a.refresh(ctx); err != nil {
		a.logger.Warn("initial prompt refresh failed", zap.Error(err))
	}
	if interval <= 0 {
		return
	}

	a.mu.Lock()
	if a.ticker != nil {
		a.mu.Unlock()
		return
	}
	a.ticker = time.NewTicker(interval)
	a.mu.Unlock()

	go func() {
		for {
			select {
			case <-a.ticker.C:
				if a.refreshBeat != nil {
					a.refreshBeat.Beat()
				}
				if err := a.refresh(ctx); err != nil {
					a.logger.Warn("prompt refresh failed", zap.Error(err))
				}
			case <-a.stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (a *PromptIndex) Stop() {
	a.mu.Lock()
	if a.ticker != nil {
		a.ticker.Stop()
		a.ticker = nil
	}
	if a.refreshBeat != nil {
		a.refreshBeat.Stop()
		a.refreshBeat = nil
	}
	if a.stop != nil {
		close(a.stop)
		a.stop = nil
	}
	a.started = false
	a.mu.Unlock()
}

func (a *PromptIndex) Snapshot() domain.PromptSnapshot {
	state := a.state.Load().(promptIndexState)
	return copyPromptSnapshot(state.snapshot)
}

func (a *PromptIndex) Subscribe(ctx context.Context) <-chan domain.PromptSnapshot {
	ch := make(chan domain.PromptSnapshot, 1)

	a.mu.Lock()
	a.subs[ch] = struct{}{}
	a.mu.Unlock()

	state := a.state.Load().(promptIndexState)
	sendPromptSnapshot(ch, state.snapshot)

	go func() {
		<-ctx.Done()
		a.mu.Lock()
		delete(a.subs, ch)
		a.mu.Unlock()
	}()

	return ch
}

func (a *PromptIndex) Resolve(name string) (domain.PromptTarget, bool) {
	state := a.state.Load().(promptIndexState)
	target, ok := state.targets[name]
	return target, ok
}

func (a *PromptIndex) GetPrompt(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
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

func (a *PromptIndex) refresh(ctx context.Context) error {
	if err := a.gate.Acquire(ctx); err != nil {
		return err
	}
	defer a.gate.Release()

	serverTypes := sortedServerTypes(a.specs)
	if len(serverTypes) == 0 {
		return nil
	}

	type refreshResult struct {
		serverType string
		prompts    []domain.PromptDefinition
		targets    map[string]domain.PromptTarget
		err        error
	}

	results := make(chan refreshResult, len(serverTypes))
	timeout := refreshTimeout(a.cfg)

	var wg sync.WaitGroup
	for _, serverType := range serverTypes {
		spec := a.specs[serverType]
		wg.Add(1)
		go func(serverType string, spec domain.ServerSpec) {
			defer wg.Done()
			fetchCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			prompts, targets, err := a.fetchServerPrompts(fetchCtx, serverType, spec)
			results <- refreshResult{
				serverType: serverType,
				prompts:    prompts,
				targets:    targets,
				err:        err,
			}
		}(serverType, spec)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.err != nil {
			if errors.Is(res.err, domain.ErrMethodNotAllowed) {
				a.mu.Lock()
				delete(a.serverCache, res.serverType)
				a.mu.Unlock()
				a.rebuildSnapshot()
				continue
			}
			a.logger.Warn("prompt list fetch failed", zap.String("serverType", res.serverType), zap.Error(res.err))
			continue
		}

		a.mu.Lock()
		a.serverCache[res.serverType] = promptCache{
			prompts: res.prompts,
			targets: res.targets,
		}
		a.mu.Unlock()
		a.rebuildSnapshot()
	}
	return nil
}

func (a *PromptIndex) rebuildSnapshot() {
	cache := a.copyServerCache()
	merged := make([]domain.PromptDefinition, 0)
	targets := make(map[string]domain.PromptTarget)

	serverTypes := sortedServerTypes(cache)
	for _, serverType := range serverTypes {
		server := cache[serverType]
		prompts := append([]domain.PromptDefinition(nil), server.prompts...)
		sort.Slice(prompts, func(i, j int) bool { return prompts[i].Name < prompts[j].Name })

		for _, prompt := range prompts {
			promptDef := prompt
			target := server.targets[prompt.Name]

			if existing, exists := targets[prompt.Name]; exists {
				if a.cfg.ToolNamespaceStrategy != "flat" {
					a.logger.Warn("prompt name conflict", zap.String("serverType", serverType), zap.String("prompt", prompt.Name))
					continue
				}
				resolvedName, err := a.resolveFlatConflict(prompt.Name, serverType, targets)
				if err != nil {
					a.logger.Warn("prompt conflict resolution failed", zap.String("serverType", serverType), zap.String("prompt", prompt.Name), zap.Error(err))
					continue
				}
				renamed, err := renamePromptDefinition(prompt, resolvedName)
				if err != nil {
					a.logger.Warn("prompt rename failed", zap.String("serverType", serverType), zap.String("prompt", prompt.Name), zap.Error(err))
					continue
				}
				promptDef = renamed
				target = domain.PromptTarget{
					ServerType: target.ServerType,
					SpecKey:    target.SpecKey,
					PromptName: target.PromptName,
				}
				targets[prompt.Name] = existing
			}

			targets[promptDef.Name] = target
			merged = append(merged, promptDef)
		}
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })

	etag := hashPrompts(merged)
	state := a.state.Load().(promptIndexState)
	if etag == state.snapshot.ETag {
		return
	}

	snapshot := domain.PromptSnapshot{
		ETag:    etag,
		Prompts: merged,
	}
	a.state.Store(promptIndexState{
		snapshot: snapshot,
		targets:  targets,
	})
	a.broadcast(snapshot)
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

func renamePromptDefinition(def domain.PromptDefinition, newName string) (domain.PromptDefinition, error) {
	var obj map[string]any
	if err := json.Unmarshal(def.PromptJSON, &obj); err != nil {
		return def, err
	}
	obj["name"] = newName
	raw, err := json.Marshal(obj)
	if err != nil {
		return def, err
	}
	return domain.PromptDefinition{
		Name:       newName,
		PromptJSON: raw,
	}, nil
}

func (a *PromptIndex) broadcast(snapshot domain.PromptSnapshot) {
	subs := a.copySubscribers()
	for _, ch := range subs {
		sendPromptSnapshot(ch, snapshot)
	}
}

func (a *PromptIndex) copyServerCache() map[string]promptCache {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make(map[string]promptCache, len(a.serverCache))
	for key, cache := range a.serverCache {
		out[key] = cache
	}
	return out
}

func (a *PromptIndex) copySubscribers() []chan domain.PromptSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]chan domain.PromptSnapshot, 0, len(a.subs))
	for ch := range a.subs {
		out = append(out, ch)
	}
	return out
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
		name := a.namespacePrompt(serverType, prompt.Name)
		promptCopy := *prompt
		promptCopy.Name = name

		raw, err := json.Marshal(&promptCopy)
		if err != nil {
			a.logger.Warn("marshal prompt failed", zap.String("serverType", serverType), zap.String("prompt", prompt.Name), zap.Error(err))
			continue
		}

		result = append(result, domain.PromptDefinition{
			Name:       name,
			PromptJSON: raw,
		})
		targets[name] = domain.PromptTarget{
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

		resp, err := a.router.Route(ctx, serverType, specKey, "", payload)
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

func (a *PromptIndex) namespacePrompt(serverType, promptName string) string {
	if a.cfg.ToolNamespaceStrategy == "flat" {
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

func hashPrompts(prompts []domain.PromptDefinition) string {
	hasher := sha256.New()
	for _, prompt := range prompts {
		_, _ = hasher.Write([]byte(prompt.Name))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(prompt.PromptJSON)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func copyPromptSnapshot(snapshot domain.PromptSnapshot) domain.PromptSnapshot {
	out := domain.PromptSnapshot{
		ETag:    snapshot.ETag,
		Prompts: make([]domain.PromptDefinition, 0, len(snapshot.Prompts)),
	}
	for _, prompt := range snapshot.Prompts {
		raw := make([]byte, len(prompt.PromptJSON))
		copy(raw, prompt.PromptJSON)
		out.Prompts = append(out.Prompts, domain.PromptDefinition{
			Name:       prompt.Name,
			PromptJSON: raw,
		})
	}
	return out
}

func sendPromptSnapshot(ch chan domain.PromptSnapshot, snapshot domain.PromptSnapshot) {
	select {
	case ch <- copyPromptSnapshot(snapshot):
	default:
	}
}
