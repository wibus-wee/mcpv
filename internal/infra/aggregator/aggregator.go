package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/hashutil"
	"mcpv/internal/infra/mcpcodec"
	"mcpv/internal/infra/telemetry"
)

// BootstrapWaiter waits for bootstrap completion using the provided context.
type BootstrapWaiter func(ctx context.Context) error

// ToolIndex aggregates tool metadata across specs and supports routing calls.
type ToolIndex struct {
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
	serverSnapshots      map[string]serverToolSnapshot

	reqBuilder requestBuilder
	index      *GenericIndex[domain.ToolSnapshot, domain.ToolTarget, serverCache]
}

type serverCache struct {
	tools   []domain.ToolDefinition
	targets map[string]domain.ToolTarget
	etag    string
}

type serverToolSnapshot struct {
	snapshot domain.ToolSnapshot
	targets  map[string]domain.ToolTarget
}

// NewToolIndex builds a ToolIndex for the provided runtime configuration.
func NewToolIndex(rt domain.Router, specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig, metadataCache *domain.MetadataCache, logger *zap.Logger, health *telemetry.HealthTracker, gate *RefreshGate, listChanges listChangeSubscriber) *ToolIndex {
	if logger == nil {
		logger = zap.NewNop()
	}
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	toolIndex := &ToolIndex{
		router:          rt,
		specs:           specs,
		specKeys:        specKeys,
		cfg:             cfg,
		metadataCache:   metadataCache,
		logger:          logger.Named("tool_index"),
		health:          health,
		gate:            gate,
		listChanges:     listChanges,
		specKeySet:      specKeySet(specKeys),
		serverSnapshots: map[string]serverToolSnapshot{},
	}
	toolIndex.index = NewGenericIndex(GenericIndexOptions[domain.ToolSnapshot, domain.ToolTarget, serverCache]{
		Name:              "tool_index",
		LogLabel:          "tool",
		FetchErrorMessage: "tool list fetch failed",
		Specs:             specs,
		Config:            cfg,
		Logger:            toolIndex.logger,
		Health:            health,
		Gate:              gate,
		EmptySnapshot:     func() domain.ToolSnapshot { return domain.ToolSnapshot{} },
		CopySnapshot:      copySnapshot,
		SnapshotETag:      func(snapshot domain.ToolSnapshot) string { return snapshot.ETag },
		BuildSnapshot:     toolIndex.buildSnapshot,
		CacheETag:         func(cache serverCache) string { return cache.etag },
		Fetch:             toolIndex.fetchServerCache,
		OnRefreshError:    toolIndex.refreshErrorDecision,
		ShouldStart:       func(cfg domain.RuntimeConfig) bool { return cfg.ExposeTools },
	})
	return toolIndex
}

// Start begins periodic refresh and list change tracking.
func (a *ToolIndex) Start(ctx context.Context) {
	baseCtx := a.setBaseContext(ctx)
	a.index.Start(baseCtx)
	a.startListChangeListener(baseCtx)
	a.startBootstrapRefresh(baseCtx)
}

// Stop halts refresh activity and cancels bootstrap waits.
func (a *ToolIndex) Stop() {
	a.index.Stop()
	a.clearBaseContext()
}

// Refresh fetches tool metadata on demand.
func (a *ToolIndex) Refresh(ctx context.Context) error {
	return a.index.Refresh(ctx)
}

// Snapshot returns the latest tool snapshot.
func (a *ToolIndex) Snapshot() domain.ToolSnapshot {
	return a.index.Snapshot()
}

// SnapshotForServer returns the latest tool snapshot for a server.
func (a *ToolIndex) SnapshotForServer(serverName string) (domain.ToolSnapshot, bool) {
	if serverName == "" {
		return domain.ToolSnapshot{}, false
	}
	a.serverMu.RLock()
	entry, ok := a.serverSnapshots[serverName]
	a.serverMu.RUnlock()
	if !ok {
		return domain.ToolSnapshot{}, false
	}
	return domain.CloneToolSnapshot(entry.snapshot), true
}

// CachedSnapshot builds a snapshot from metadata cache without touching live instances.
func (a *ToolIndex) CachedSnapshot() domain.ToolSnapshot {
	a.specsMu.RLock()
	specs := a.specs
	cfg := a.cfg
	a.specsMu.RUnlock()
	if a.metadataCache == nil || !cfg.ExposeTools {
		return domain.ToolSnapshot{}
	}

	cache := make(map[string]serverCache)
	serverTypes := sortedServerTypes(specs)
	for _, serverType := range serverTypes {
		spec := specs[serverType]
		cached, ok := a.cachedServerCache(serverType, spec)
		if !ok || len(cached.tools) == 0 {
			continue
		}
		cache[serverType] = cached
	}

	if len(cache) == 0 {
		return domain.ToolSnapshot{}
	}

	snapshot, _ := a.buildSnapshot(cache)
	return snapshot
}

// Subscribe streams tool snapshot updates.
func (a *ToolIndex) Subscribe(ctx context.Context) <-chan domain.ToolSnapshot {
	return a.index.Subscribe(ctx)
}

// Resolve locates a tool target by name.
func (a *ToolIndex) Resolve(name string) (domain.ToolTarget, bool) {
	return a.index.Resolve(name)
}

// ResolveForServer locates a tool target for a server by raw tool name.
func (a *ToolIndex) ResolveForServer(serverName, toolName string) (domain.ToolTarget, bool) {
	if serverName == "" || toolName == "" {
		return domain.ToolTarget{}, false
	}
	a.serverMu.RLock()
	entry, ok := a.serverSnapshots[serverName]
	a.serverMu.RUnlock()
	if !ok {
		return domain.ToolTarget{}, false
	}
	target, ok := entry.targets[toolName]
	return target, ok
}

// SetBootstrapWaiter registers a bootstrap completion hook.
func (a *ToolIndex) SetBootstrapWaiter(waiter BootstrapWaiter) {
	a.bootstrapWaiterMu.Lock()
	a.bootstrapWaiter = waiter
	a.bootstrapWaiterMu.Unlock()
	if baseCtx := a.baseContext(); baseCtx != nil {
		a.startBootstrapRefresh(baseCtx)
	}
}

// CallTool routes a tool call to the owning server.
func (a *ToolIndex) CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
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
		return nil, domain.ErrToolNotFound
	}

	params := &mcp.CallToolParams{
		Name:      target.ToolName,
		Arguments: json.RawMessage(args),
	}
	payload, err := a.reqBuilder.Build("tools/call", params)
	if err != nil {
		return nil, err
	}

	resp, err := a.router.Route(ctx, target.ServerType, target.SpecKey, routingKey, payload)
	if err != nil {
		return nil, err
	}

	result, err := decodeToolResult(resp)
	if err != nil {
		return nil, err
	}
	return marshalToolResult(result)
}

// CallToolForServer routes a tool call to the owning server using a raw tool name.
func (a *ToolIndex) CallToolForServer(ctx context.Context, serverName, toolName string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	target, ok := a.ResolveForServer(serverName, toolName)
	if !ok {
		return nil, domain.ErrToolNotFound
	}

	params := &mcp.CallToolParams{
		Name:      target.ToolName,
		Arguments: json.RawMessage(args),
	}
	payload, err := a.reqBuilder.Build("tools/call", params)
	if err != nil {
		return nil, err
	}

	resp, err := a.router.Route(ctx, target.ServerType, target.SpecKey, routingKey, payload)
	if err != nil {
		return nil, err
	}

	result, err := decodeToolResult(resp)
	if err != nil {
		return nil, err
	}
	return marshalToolResult(result)
}

// UpdateSpecs replaces the registry backing the tool index.
func (a *ToolIndex) UpdateSpecs(specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig) {
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
func (a *ToolIndex) ApplyRuntimeConfig(cfg domain.RuntimeConfig) {
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
	if prevCfg.ToolRefreshInterval() != cfg.ToolRefreshInterval() || prevCfg.ExposeTools != cfg.ExposeTools {
		a.index.Stop()
		a.index.Start(baseCtx)
	}
	if !prevCfg.ExposeTools && cfg.ExposeTools {
		a.startListChangeListener(baseCtx)
	}
}

func (a *ToolIndex) refresh(ctx context.Context) error {
	return a.index.Refresh(ctx)
}

func (a *ToolIndex) startListChangeListener(ctx context.Context) {
	if a.listChanges == nil {
		return
	}
	a.specsMu.RLock()
	exposeTools := a.cfg.ExposeTools
	a.specsMu.RUnlock()
	if !exposeTools {
		return
	}
	ch := a.listChanges.Subscribe(ctx, domain.ListChangeTools)
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
				exposeTools := a.cfg.ExposeTools
				specs := a.specs
				specKeySet := a.specKeySet
				a.specsMu.RUnlock()
				if !exposeTools {
					continue
				}
				if !listChangeApplies(specs, specKeySet, event) {
					continue
				}
				if err := a.index.Refresh(ctx); err != nil {
					a.logger.Warn("tool refresh after list change failed", zap.Error(err))
				}
			}
		}
	}()
}

func (a *ToolIndex) startBootstrapRefresh(ctx context.Context) {
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
				a.logger.Warn("tool bootstrap wait failed", zap.Error(err))
				return
			}

			a.specsMu.RLock()
			cfg := a.cfg
			a.specsMu.RUnlock()
			refreshCtx, cancel := withRefreshTimeout(ctx, cfg)
			defer cancel()
			if err := a.index.Refresh(refreshCtx); err != nil {
				a.logger.Warn("tool refresh after bootstrap failed", zap.Error(err))
			}
		}()
	})
}

func (a *ToolIndex) setBaseContext(ctx context.Context) context.Context {
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

func (a *ToolIndex) baseContext() context.Context {
	a.baseMu.RLock()
	defer a.baseMu.RUnlock()
	return a.baseCtx
}

func (a *ToolIndex) clearBaseContext() {
	a.baseMu.Lock()
	if a.baseCancel != nil {
		a.baseCancel()
	}
	a.baseCtx = nil
	a.baseCancel = nil
	a.baseMu.Unlock()
}

func (a *ToolIndex) buildSnapshot(cache map[string]serverCache) (domain.ToolSnapshot, map[string]domain.ToolTarget) {
	merged := make([]domain.ToolDefinition, 0)
	targets := make(map[string]domain.ToolTarget)
	serverSnapshots := make(map[string]serverToolSnapshot, len(cache))
	a.specsMu.RLock()
	specs := a.specs
	strategy := a.cfg.ToolNamespaceStrategy
	a.specsMu.RUnlock()

	serverTypes := sortedServerTypes(cache)
	for _, serverType := range serverTypes {
		server := cache[serverType]
		spec := specs[serverType]
		tools := append([]domain.ToolDefinition(nil), server.tools...)
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })

		snapshot := serverToolSnapshot{
			snapshot: domain.ToolSnapshot{
				ETag:  a.hashTools(tools),
				Tools: tools,
			},
			targets: copyToolTargets(server.targets),
		}
		if spec.Name == "" {
			a.logger.Warn("tool snapshot skipped: missing server name", zap.String("serverType", serverType))
		} else {
			serverSnapshots[spec.Name] = snapshot
		}

		for _, tool := range tools {
			toolDef := tool
			target := server.targets[tool.Name]
			displayName := namespaceTool(serverType, tool.Name, strategy)
			toolDef.Name = displayName

			if existing, exists := targets[displayName]; exists {
				if strategy != domain.ToolNamespaceStrategyFlat {
					a.logger.Warn("tool name conflict", zap.String("serverType", serverType), zap.String("tool", tool.Name))
					continue
				}
				resolvedName, err := a.resolveFlatConflict(displayName, serverType, targets)
				if err != nil {
					a.logger.Warn("tool conflict resolution failed", zap.String("serverType", serverType), zap.String("tool", tool.Name), zap.Error(err))
					continue
				}
				renamed, err := renameToolDefinition(toolDef, resolvedName)
				if err != nil {
					a.logger.Warn("tool rename failed", zap.String("serverType", serverType), zap.String("tool", tool.Name), zap.Error(err))
					continue
				}
				toolDef = renamed
				target = domain.ToolTarget{
					ServerType: target.ServerType,
					SpecKey:    target.SpecKey,
					ToolName:   target.ToolName,
				}
				targets[displayName] = existing
			}

			toolDef.SpecKey = target.SpecKey
			toolDef.ServerName = spec.Name

			targets[toolDef.Name] = target
			merged = append(merged, toolDef)
		}
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })

	a.storeServerSnapshots(serverSnapshots)

	return domain.ToolSnapshot{
		ETag:  a.hashTools(merged),
		Tools: merged,
	}, targets
}

func (a *ToolIndex) storeServerSnapshots(snapshots map[string]serverToolSnapshot) {
	a.serverMu.Lock()
	a.serverSnapshots = snapshots
	a.serverMu.Unlock()
}

func copyToolTargets(in map[string]domain.ToolTarget) map[string]domain.ToolTarget {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]domain.ToolTarget, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (a *ToolIndex) resolveFlatConflict(name, serverType string, existing map[string]domain.ToolTarget) (string, error) {
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

func renameToolDefinition(def domain.ToolDefinition, newName string) (domain.ToolDefinition, error) {
	def.Name = newName
	return def, nil
}

func (a *ToolIndex) refreshErrorDecision(_ string, err error) refreshErrorDecision {
	if errors.Is(err, domain.ErrNoReadyInstance) {
		return refreshErrorSkip
	}
	return refreshErrorLog
}

func (a *ToolIndex) fetchServerCache(ctx context.Context, serverType string, spec domain.ServerSpec) (serverCache, error) {
	tools, targets, err := a.fetchServerTools(ctx, serverType, spec)
	if err != nil {
		if errors.Is(err, domain.ErrNoReadyInstance) {
			if cached, ok := a.cachedServerCache(serverType, spec); ok {
				return cached, nil
			}
		}
		return serverCache{}, err
	}
	return serverCache{tools: tools, targets: targets, etag: a.hashTools(tools)}, nil
}

func (a *ToolIndex) cachedServerCache(serverType string, spec domain.ServerSpec) (serverCache, bool) {
	if a.metadataCache == nil {
		return serverCache{}, false
	}
	specKey := a.specKeys[serverType]
	if specKey == "" {
		return serverCache{}, false
	}
	tools, ok := a.metadataCache.GetTools(specKey)
	if !ok {
		return serverCache{}, false
	}

	allowed := allowedTools(spec)
	result := make([]domain.ToolDefinition, 0, len(tools))
	targets := make(map[string]domain.ToolTarget)

	for _, tool := range tools {
		if tool.Name == "" {
			continue
		}
		if !allowed(tool.Name) {
			continue
		}
		if !mcpcodec.IsObjectSchema(tool.InputSchema) {
			a.logger.Warn("skip cached tool with invalid input schema", zap.String("serverType", serverType), zap.String("tool", tool.Name))
			continue
		}
		if tool.OutputSchema != nil && !mcpcodec.IsObjectSchema(tool.OutputSchema) {
			a.logger.Warn("skip cached tool with invalid output schema", zap.String("serverType", serverType), zap.String("tool", tool.Name))
			continue
		}

		toolDef := tool
		toolDef.Name = tool.Name
		toolDef.SpecKey = specKey
		toolDef.ServerName = spec.Name

		result = append(result, toolDef)
		targets[tool.Name] = domain.ToolTarget{
			ServerType: serverType,
			SpecKey:    specKey,
			ToolName:   tool.Name,
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return serverCache{tools: result, targets: targets, etag: a.hashTools(result)}, true
}

func (a *ToolIndex) fetchServerTools(ctx context.Context, serverType string, spec domain.ServerSpec) ([]domain.ToolDefinition, map[string]domain.ToolTarget, error) {
	specKey := a.specKeys[serverType]
	if specKey == "" {
		return nil, nil, fmt.Errorf("missing spec key for server type %q", serverType)
	}
	tools, err := a.fetchTools(ctx, serverType, specKey)
	if err != nil {
		return nil, nil, err
	}

	allowed := allowedTools(spec)
	result := make([]domain.ToolDefinition, 0, len(tools))
	targets := make(map[string]domain.ToolTarget)

	for _, tool := range tools {
		if tool == nil {
			continue
		}
		if !allowed(tool.Name) {
			continue
		}
		if tool.Name == "" {
			continue
		}
		if !mcpcodec.IsObjectSchema(tool.InputSchema) {
			a.logger.Warn("skip tool with invalid input schema", zap.String("serverType", serverType), zap.String("tool", tool.Name))
			continue
		}
		if tool.OutputSchema != nil && !mcpcodec.IsObjectSchema(tool.OutputSchema) {
			a.logger.Warn("skip tool with invalid output schema", zap.String("serverType", serverType), zap.String("tool", tool.Name))
			continue
		}

		def := mcpcodec.ToolFromMCP(tool)
		def.Name = tool.Name
		def.SpecKey = specKey
		def.ServerName = spec.Name
		result = append(result, def)
		targets[tool.Name] = domain.ToolTarget{
			ServerType: serverType,
			SpecKey:    specKey,
			ToolName:   tool.Name,
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, targets, nil
}

func (a *ToolIndex) fetchTools(ctx context.Context, serverType, specKey string) ([]*mcp.Tool, error) {
	var tools []*mcp.Tool
	cursor := ""

	for {
		params := &mcp.ListToolsParams{Cursor: cursor}
		payload, err := a.reqBuilder.Build("tools/list", params)
		if err != nil {
			return nil, err
		}

		resp, err := a.router.RouteWithOptions(ctx, serverType, specKey, "", payload, domain.RouteOptions{AllowStart: false})
		if err != nil {
			return nil, err
		}

		result, err := decodeListToolsResult(resp)
		if err != nil {
			return nil, err
		}
		tools = append(tools, result.Tools...)
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return tools, nil
}

func namespaceTool(serverType, toolName string, strategy domain.ToolNamespaceStrategy) string {
	if strategy == domain.ToolNamespaceStrategyFlat {
		return toolName
	}
	return fmt.Sprintf("%s.%s", serverType, toolName)
}

func sortedServerTypes[T any](specs map[string]T) []string {
	keys := make([]string, 0, len(specs))
	for key := range specs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func refreshTimeout(cfg domain.RuntimeConfig) time.Duration {
	return cfg.RouteTimeout()
}

func allowedTools(spec domain.ServerSpec) func(string) bool {
	if len(spec.ExposeTools) == 0 {
		return func(_ string) bool { return true }
	}

	allowed := make(map[string]struct{}, len(spec.ExposeTools))
	for _, name := range spec.ExposeTools {
		allowed[name] = struct{}{}
	}
	return func(name string) bool {
		_, ok := allowed[name]
		return ok
	}
}

func decodeListToolsResult(raw json.RawMessage) (*mcp.ListToolsResult, error) {
	resp, err := decodeJSONRPCResponse(raw)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %w", resp.Error)
	}

	if len(resp.Result) == 0 {
		return nil, errors.New("tools/list response missing result")
	}

	var result mcp.ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode tools/list result: %w", err)
	}
	return &result, nil
}

func decodeToolResult(raw json.RawMessage) (*mcp.CallToolResult, error) {
	if protoErr, err := decodeProtocolError(raw); err != nil {
		return nil, err
	} else if protoErr != nil {
		return nil, protoErr
	}
	resp, err := decodeJSONRPCResponse(raw)
	if err != nil {
		return errorResult(err), nil
	}

	if resp.Error != nil {
		return errorResult(resp.Error), nil
	}

	if len(resp.Result) == 0 {
		return errorResult(errors.New("tools/call response missing result")), nil
	}

	var result mcp.CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return errorResult(fmt.Errorf("decode tools/call result: %w", err)), nil
	}
	return &result, nil
}

type wireError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type wireResponse struct {
	Error  *wireError      `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

func decodeProtocolError(raw json.RawMessage) (*domain.ProtocolError, error) {
	var wire wireResponse
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, nil
	}
	if wire.Error == nil {
		return nil, nil
	}
	if wire.Error.Code != domain.ErrCodeURLElicitationRequired {
		return nil, nil
	}
	return &domain.ProtocolError{
		Code:    wire.Error.Code,
		Message: wire.Error.Message,
		Data:    wire.Error.Data,
	}, nil
}

func decodeJSONRPCResponse(raw json.RawMessage) (*jsonrpc.Response, error) {
	msg, err := jsonrpc.DecodeMessage(raw)
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	resp, ok := msg.(*jsonrpc.Response)
	if !ok {
		return nil, errors.New("response is not a response message")
	}
	return resp, nil
}

func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("error: %s", err.Error())},
		},
	}
}

func marshalToolResult(result *mcp.CallToolResult) (json.RawMessage, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func (a *ToolIndex) hashTools(tools []domain.ToolDefinition) string {
	return hashutil.ToolETag(a.logger, tools)
}

func copySnapshot(snapshot domain.ToolSnapshot) domain.ToolSnapshot {
	return domain.CloneToolSnapshot(snapshot)
}
