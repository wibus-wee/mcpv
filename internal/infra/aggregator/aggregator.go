package aggregator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/telemetry"
)

type ToolIndex struct {
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
	serverCache map[string]serverCache
	subs        map[chan domain.ToolSnapshot]struct{}
	refreshBeat *telemetry.Heartbeat
	state       atomic.Value
	reqBuilder  requestBuilder
}

type serverCache struct {
	tools   []domain.ToolDefinition
	targets map[string]domain.ToolTarget
}

type toolIndexState struct {
	snapshot domain.ToolSnapshot
	targets  map[string]domain.ToolTarget
}

func NewToolIndex(rt domain.Router, specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig, logger *zap.Logger, health *telemetry.HealthTracker, gate *RefreshGate) *ToolIndex {
	if logger == nil {
		logger = zap.NewNop()
	}
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	index := &ToolIndex{
		router:      rt,
		specs:       specs,
		specKeys:    specKeys,
		cfg:         cfg,
		logger:      logger.Named("tool_index"),
		health:      health,
		gate:        gate,
		stop:        make(chan struct{}),
		serverCache: make(map[string]serverCache),
		subs:        make(map[chan domain.ToolSnapshot]struct{}),
	}
	index.state.Store(toolIndexState{
		snapshot: domain.ToolSnapshot{},
		targets:  make(map[string]domain.ToolTarget),
	})
	return index
}

func (a *ToolIndex) Start(ctx context.Context) {
	if !a.cfg.ExposeTools {
		return
	}

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
		a.refreshBeat = a.health.Register("tool_index.refresh", interval*3)
	}
	if a.refreshBeat != nil {
		a.refreshBeat.Beat()
	}
	if err := a.refresh(ctx); err != nil {
		a.logger.Warn("initial tool refresh failed", zap.Error(err))
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
					a.logger.Warn("tool refresh failed", zap.Error(err))
				}
			case <-a.stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (a *ToolIndex) Stop() {
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

func (a *ToolIndex) Snapshot() domain.ToolSnapshot {
	state := a.state.Load().(toolIndexState)
	return copySnapshot(state.snapshot)
}

func (a *ToolIndex) Subscribe(ctx context.Context) <-chan domain.ToolSnapshot {
	ch := make(chan domain.ToolSnapshot, 1)

	a.mu.Lock()
	a.subs[ch] = struct{}{}
	a.mu.Unlock()

	state := a.state.Load().(toolIndexState)
	sendSnapshot(ch, state.snapshot)

	go func() {
		<-ctx.Done()
		a.mu.Lock()
		delete(a.subs, ch)
		a.mu.Unlock()
	}()

	return ch
}

func (a *ToolIndex) Resolve(name string) (domain.ToolTarget, bool) {
	state := a.state.Load().(toolIndexState)
	target, ok := state.targets[name]
	return target, ok
}

func (a *ToolIndex) CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
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

func (a *ToolIndex) refresh(ctx context.Context) error {
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
		tools      []domain.ToolDefinition
		targets    map[string]domain.ToolTarget
		err        error
	}

	results := make(chan refreshResult, len(serverTypes))
	timeout := refreshTimeout(a.cfg)
	workerCount := refreshWorkerCount(a.cfg, len(serverTypes))
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
					spec := a.specs[serverType]
					fetchCtx, cancel := context.WithTimeout(ctx, timeout)
					tools, targets, err := a.fetchServerTools(fetchCtx, serverType, spec)
					cancel()
					results <- refreshResult{
						serverType: serverType,
						tools:      tools,
						targets:    targets,
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
			if errors.Is(res.err, domain.ErrNoReadyInstance) {
				continue
			}
			a.logger.Warn("tool list fetch failed", zap.String("serverType", res.serverType), zap.Error(res.err))
			continue
		}

		a.mu.Lock()
		a.serverCache[res.serverType] = serverCache{
			tools:   res.tools,
			targets: res.targets,
		}
		a.mu.Unlock()
		a.rebuildSnapshot()
	}
	return nil
}

func (a *ToolIndex) rebuildSnapshot() {
	cache := a.copyServerCache()
	merged := make([]domain.ToolDefinition, 0)
	targets := make(map[string]domain.ToolTarget)

	serverTypes := sortedServerTypes(cache)
	for _, serverType := range serverTypes {
		server := cache[serverType]
		tools := append([]domain.ToolDefinition(nil), server.tools...)
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })

		for _, tool := range tools {
			toolDef := tool
			target := server.targets[tool.Name]

			if existing, exists := targets[tool.Name]; exists {
				if a.cfg.ToolNamespaceStrategy != "flat" {
					a.logger.Warn("tool name conflict", zap.String("serverType", serverType), zap.String("tool", tool.Name))
					continue
				}
				resolvedName, err := a.resolveFlatConflict(tool.Name, serverType, targets)
				if err != nil {
					a.logger.Warn("tool conflict resolution failed", zap.String("serverType", serverType), zap.String("tool", tool.Name), zap.Error(err))
					continue
				}
				renamed, err := renameToolDefinition(tool, resolvedName)
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
				targets[tool.Name] = existing // keep existing binding
			}

			toolDef.SpecKey = target.SpecKey
			if spec, ok := a.specs[serverType]; ok {
				toolDef.ServerName = spec.Name
			}

			targets[toolDef.Name] = target
			merged = append(merged, toolDef)
		}
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })

	etag := hashTools(merged)
	state := a.state.Load().(toolIndexState)
	if etag == state.snapshot.ETag {
		return
	}

	snapshot := domain.ToolSnapshot{
		ETag:  etag,
		Tools: merged,
	}
	a.state.Store(toolIndexState{
		snapshot: snapshot,
		targets:  targets,
	})
	a.broadcast(snapshot)
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
	var obj map[string]any
	if err := json.Unmarshal(def.ToolJSON, &obj); err != nil {
		return def, err
	}
	obj["name"] = newName
	raw, err := json.Marshal(obj)
	if err != nil {
		return def, err
	}
	return domain.ToolDefinition{
		Name:       newName,
		ToolJSON:   raw,
		SpecKey:    def.SpecKey,
		ServerName: def.ServerName,
	}, nil
}

func (a *ToolIndex) broadcast(snapshot domain.ToolSnapshot) {
	subs := a.copySubscribers()
	for _, ch := range subs {
		sendSnapshot(ch, snapshot)
	}
}

func (a *ToolIndex) copyServerCache() map[string]serverCache {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make(map[string]serverCache, len(a.serverCache))
	for key, cache := range a.serverCache {
		out[key] = cache
	}
	return out
}

func (a *ToolIndex) copySubscribers() []chan domain.ToolSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]chan domain.ToolSnapshot, 0, len(a.subs))
	for ch := range a.subs {
		out = append(out, ch)
	}
	return out
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
		if !isObjectSchema(tool.InputSchema) {
			a.logger.Warn("skip tool with invalid input schema", zap.String("serverType", serverType), zap.String("tool", tool.Name))
			continue
		}
		if tool.OutputSchema != nil && !isObjectSchema(tool.OutputSchema) {
			a.logger.Warn("skip tool with invalid output schema", zap.String("serverType", serverType), zap.String("tool", tool.Name))
			continue
		}

		name := a.namespaceTool(serverType, tool.Name)
		toolCopy := *tool
		toolCopy.Name = name

		raw, err := json.Marshal(&toolCopy)
		if err != nil {
			a.logger.Warn("marshal tool failed", zap.String("serverType", serverType), zap.String("tool", tool.Name), zap.Error(err))
			continue
		}

		result = append(result, domain.ToolDefinition{
			Name:       name,
			ToolJSON:   raw,
			SpecKey:    specKey,
			ServerName: spec.Name,
		})
		targets[name] = domain.ToolTarget{
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

func (a *ToolIndex) namespaceTool(serverType, toolName string) string {
	if a.cfg.ToolNamespaceStrategy == "flat" {
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
	timeout := time.Duration(cfg.RouteTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(domain.DefaultRouteTimeoutSeconds) * time.Second
	}
	return timeout
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

func isObjectSchema(schema any) bool {
	if schema == nil {
		return false
	}

	switch value := schema.(type) {
	case map[string]any:
		return hasObjectType(value["type"])
	case json.RawMessage:
		return hasObjectTypeJSON(value)
	case []byte:
		return hasObjectTypeJSON(value)
	case string:
		return hasObjectTypeJSON([]byte(value))
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return false
		}
		return hasObjectTypeJSON(raw)
	}
}

type schemaTypeField struct {
	Type any `json:"type"`
}

func hasObjectTypeJSON(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var schema schemaTypeField
	if err := json.Unmarshal(raw, &schema); err != nil {
		return false
	}
	return hasObjectType(schema.Type)
}

func hasObjectType(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.EqualFold(typed, "object")
	case []any:
		for _, item := range typed {
			if str, ok := item.(string); ok && strings.EqualFold(str, "object") {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if strings.EqualFold(item, "object") {
				return true
			}
		}
	}
	return false
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

func hashTools(tools []domain.ToolDefinition) string {
	hasher := sha256.New()
	for _, tool := range tools {
		_, _ = hasher.Write([]byte(tool.Name))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(tool.ToolJSON)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func copySnapshot(snapshot domain.ToolSnapshot) domain.ToolSnapshot {
	out := domain.ToolSnapshot{
		ETag:  snapshot.ETag,
		Tools: make([]domain.ToolDefinition, 0, len(snapshot.Tools)),
	}
	for _, tool := range snapshot.Tools {
		raw := make([]byte, len(tool.ToolJSON))
		copy(raw, tool.ToolJSON)
		out.Tools = append(out.Tools, domain.ToolDefinition{
			Name:     tool.Name,
			ToolJSON: raw,
		})
	}
	return out
}

func sendSnapshot(ch chan domain.ToolSnapshot, snapshot domain.ToolSnapshot) {
	select {
	case ch <- copySnapshot(snapshot):
	default:
	}
}
