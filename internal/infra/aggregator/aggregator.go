package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mcpcodec"
	"mcpd/internal/infra/telemetry"
)

type ToolIndex struct {
	router      domain.Router
	specs       map[string]domain.ServerSpec
	specKeys    map[string]string
	cfg         domain.RuntimeConfig
	logger      *zap.Logger
	health      *telemetry.HealthTracker
	gate        *RefreshGate
	listChanges listChangeSubscriber
	specKeySet  map[string]struct{}

	reqBuilder requestBuilder
	index      *GenericIndex[domain.ToolSnapshot, domain.ToolTarget, serverCache]
}

type serverCache struct {
	tools   []domain.ToolDefinition
	targets map[string]domain.ToolTarget
	etag    string
}

func NewToolIndex(rt domain.Router, specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig, logger *zap.Logger, health *telemetry.HealthTracker, gate *RefreshGate, listChanges listChangeSubscriber) *ToolIndex {
	if logger == nil {
		logger = zap.NewNop()
	}
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	toolIndex := &ToolIndex{
		router:      rt,
		specs:       specs,
		specKeys:    specKeys,
		cfg:         cfg,
		logger:      logger.Named("tool_index"),
		health:      health,
		gate:        gate,
		listChanges: listChanges,
		specKeySet:  specKeySet(specKeys),
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

func (a *ToolIndex) Start(ctx context.Context) {
	a.index.Start(ctx)
	a.startListChangeListener(ctx)
}

func (a *ToolIndex) Stop() {
	a.index.Stop()
}

func (a *ToolIndex) Refresh(ctx context.Context) error {
	return a.index.Refresh(ctx)
}

func (a *ToolIndex) Snapshot() domain.ToolSnapshot {
	return a.index.Snapshot()
}

func (a *ToolIndex) Subscribe(ctx context.Context) <-chan domain.ToolSnapshot {
	return a.index.Subscribe(ctx)
}

func (a *ToolIndex) Resolve(name string) (domain.ToolTarget, bool) {
	return a.index.Resolve(name)
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

func (a *ToolIndex) UpdateSpecs(specs map[string]domain.ServerSpec, specKeys map[string]string, cfg domain.RuntimeConfig) {
	if specKeys == nil {
		specKeys = map[string]string{}
	}
	a.specs = specs
	a.specKeys = specKeys
	a.specKeySet = specKeySet(specKeys)
	a.cfg = cfg
	a.index.UpdateSpecs(specs, cfg)
}

func (a *ToolIndex) refresh(ctx context.Context) error {
	return a.index.Refresh(ctx)
}

func (a *ToolIndex) startListChangeListener(ctx context.Context) {
	if a.listChanges == nil || !a.cfg.ExposeTools {
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
				if !listChangeApplies(a.specs, a.specKeySet, event) {
					continue
				}
				if err := a.index.Refresh(ctx); err != nil {
					a.logger.Warn("tool refresh after list change failed", zap.Error(err))
				}
			}
		}
	}()
}

func (a *ToolIndex) buildSnapshot(cache map[string]serverCache) (domain.ToolSnapshot, map[string]domain.ToolTarget) {
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
				targets[tool.Name] = existing
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

	return domain.ToolSnapshot{
		ETag:  hashTools(merged),
		Tools: merged,
	}, targets
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
		return serverCache{}, err
	}
	return serverCache{tools: tools, targets: targets, etag: hashTools(tools)}, nil
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

		def := mcpcodec.ToolFromMCP(&toolCopy)
		def.Name = name
		def.SpecKey = specKey
		def.ServerName = spec.Name
		result = append(result, def)
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
	return mcpcodec.HashToolDefinitions(tools)
}

func copySnapshot(snapshot domain.ToolSnapshot) domain.ToolSnapshot {
	return domain.CloneToolSnapshot(snapshot)
}
