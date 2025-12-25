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
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

type ToolIndex struct {
	router domain.Router
	specs  map[string]domain.ServerSpec
	cfg    domain.RuntimeConfig
	logger *zap.Logger

	mu          sync.RWMutex
	started     bool
	ticker      *time.Ticker
	stop        chan struct{}
	snapshot    domain.ToolSnapshot
	targets     map[string]domain.ToolTarget
	serverCache map[string]serverCache
	subs        map[chan domain.ToolSnapshot]struct{}
}

type serverCache struct {
	tools   []domain.ToolDefinition
	targets map[string]domain.ToolTarget
}

func NewToolIndex(rt domain.Router, specs map[string]domain.ServerSpec, cfg domain.RuntimeConfig, logger *zap.Logger) *ToolIndex {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ToolIndex{
		router:      rt,
		specs:       specs,
		cfg:         cfg,
		logger:      logger.Named("tool_index"),
		stop:        make(chan struct{}),
		targets:     make(map[string]domain.ToolTarget),
		serverCache: make(map[string]serverCache),
		subs:        make(map[chan domain.ToolSnapshot]struct{}),
	}
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
	a.mu.Unlock()

	if err := a.refresh(ctx); err != nil {
		a.logger.Warn("initial tool refresh failed", zap.Error(err))
	}

	interval := time.Duration(a.cfg.ToolRefreshSeconds) * time.Second
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
	select {
	case <-a.stop:
	default:
		close(a.stop)
	}
	a.mu.Unlock()
}

func (a *ToolIndex) Snapshot() domain.ToolSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return copySnapshot(a.snapshot)
}

func (a *ToolIndex) Subscribe(ctx context.Context) <-chan domain.ToolSnapshot {
	ch := make(chan domain.ToolSnapshot, 1)

	a.mu.Lock()
	a.subs[ch] = struct{}{}
	snapshot := copySnapshot(a.snapshot)
	a.mu.Unlock()

	sendSnapshot(ch, snapshot)

	go func() {
		<-ctx.Done()
		a.mu.Lock()
		delete(a.subs, ch)
		close(ch)
		a.mu.Unlock()
	}()

	return ch
}

func (a *ToolIndex) Resolve(name string) (domain.ToolTarget, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	target, ok := a.targets[name]
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
	payload, err := buildJSONRPCRequest("tools/call", params)
	if err != nil {
		return nil, err
	}

	resp, err := a.router.Route(ctx, target.ServerType, routingKey, payload)
	if err != nil {
		return marshalToolResult(errorResult(err))
	}

	result, err := decodeToolResult(resp)
	if err != nil {
		return nil, err
	}
	return marshalToolResult(result)
}

func (a *ToolIndex) refresh(ctx context.Context) error {
	serverTypes := sortedServerTypes(a.specs)
	var refreshed bool

	for _, serverType := range serverTypes {
		spec := a.specs[serverType]
		tools, targets, err := a.fetchServerTools(ctx, serverType, spec)
		if err != nil {
			a.logger.Warn("tool list fetch failed", zap.String("serverType", serverType), zap.Error(err))
			continue
		}

		a.mu.Lock()
		a.serverCache[serverType] = serverCache{
			tools:   tools,
			targets: targets,
		}
		a.mu.Unlock()
		refreshed = true
	}

	if refreshed {
		a.rebuildSnapshot()
	}
	return nil
}

func (a *ToolIndex) rebuildSnapshot() {
	a.mu.Lock()
	defer a.mu.Unlock()

	merged := make([]domain.ToolDefinition, 0)
	targets := make(map[string]domain.ToolTarget)

	serverTypes := sortedServerTypes(a.serverCache)
	for _, serverType := range serverTypes {
		cache := a.serverCache[serverType]
		tools := append([]domain.ToolDefinition(nil), cache.tools...)
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })

		for _, tool := range tools {
			if _, exists := targets[tool.Name]; exists {
				a.logger.Warn("tool name conflict", zap.String("serverType", serverType), zap.String("tool", tool.Name))
				continue
			}
			targets[tool.Name] = cache.targets[tool.Name]
			merged = append(merged, tool)
		}
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })

	etag := hashTools(merged)
	if etag == a.snapshot.ETag {
		return
	}

	a.snapshot = domain.ToolSnapshot{
		ETag:  etag,
		Tools: merged,
	}
	a.targets = targets
	a.broadcastLocked(a.snapshot)
}

func (a *ToolIndex) broadcastLocked(snapshot domain.ToolSnapshot) {
	for ch := range a.subs {
		sendSnapshot(ch, snapshot)
	}
}

func (a *ToolIndex) fetchServerTools(ctx context.Context, serverType string, spec domain.ServerSpec) ([]domain.ToolDefinition, map[string]domain.ToolTarget, error) {
	tools, err := a.fetchTools(ctx, serverType)
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
			Name:     name,
			ToolJSON: raw,
		})
		targets[name] = domain.ToolTarget{
			ServerType: serverType,
			ToolName:   tool.Name,
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, targets, nil
}

func (a *ToolIndex) fetchTools(ctx context.Context, serverType string) ([]*mcp.Tool, error) {
	var tools []*mcp.Tool
	cursor := ""

	for {
		params := &mcp.ListToolsParams{Cursor: cursor}
		payload, err := buildJSONRPCRequest("tools/list", params)
		if err != nil {
			return nil, err
		}

		resp, err := a.router.Route(ctx, serverType, "", payload)
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

	raw, err := json.Marshal(schema)
	if err != nil {
		return false
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	if typ, ok := obj["type"]; ok {
		if val, ok := typ.(string); ok {
			return strings.EqualFold(val, "object")
		}
	}
	return false
}

func buildJSONRPCRequest(method string, params any) (json.RawMessage, error) {
	id, err := jsonrpc.MakeID(fmt.Sprintf("mcpd-%s-%d", method, time.Now().UnixNano()))
	if err != nil {
		return nil, fmt.Errorf("build request id: %w", err)
	}
	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	req := &jsonrpc.Request{ID: id, Method: method, Params: rawParams}
	wire, err := jsonrpc.EncodeMessage(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	return json.RawMessage(wire), nil
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
