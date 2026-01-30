package gateway

import (
	"encoding/json"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"mcpv/internal/infra/mcpcodec"
	controlv1 "mcpv/pkg/api/control/v1"
)

type toolRegistry struct {
	server     *mcp.Server
	handler    func(name string) mcp.ToolHandler
	logger     *zap.Logger
	applyMu    sync.Mutex
	mu         sync.Mutex
	etag       string
	registered map[string]struct{}
}

func newToolRegistry(server *mcp.Server, handler func(name string) mcp.ToolHandler, logger *zap.Logger) *toolRegistry {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &toolRegistry{
		server:     server,
		handler:    handler,
		logger:     logger.Named("tool_registry"),
		registered: make(map[string]struct{}),
	}
}

func (r *toolRegistry) ApplySnapshot(snapshot *controlv1.ToolsSnapshot) {
	if snapshot == nil {
		return
	}

	r.applyMu.Lock()
	defer r.applyMu.Unlock()

	r.mu.Lock()
	if snapshot.GetEtag() != "" && snapshot.GetEtag() == r.etag {
		r.mu.Unlock()
		return
	}
	prev := make(map[string]struct{}, len(r.registered))
	for name := range r.registered {
		prev[name] = struct{}{}
	}
	r.mu.Unlock()

	next := make(map[string]struct{})
	toAdd := make([]mcp.Tool, 0, len(snapshot.GetTools()))
	for _, def := range snapshot.GetTools() {
		if def == nil || len(def.GetToolJson()) == 0 {
			continue
		}
		var tool mcp.Tool
		if err := json.Unmarshal(def.GetToolJson(), &tool); err != nil {
			r.logger.Warn("decode tool failed", zap.String("tool", def.GetName()), zap.Error(err))
			continue
		}
		if tool.Name == "" {
			tool.Name = def.GetName()
		}
		if tool.Name == "" {
			continue
		}
		if tool.Name != def.GetName() && def.GetName() != "" {
			r.logger.Warn("tool name mismatch", zap.String("tool", tool.Name), zap.String("expected", def.GetName()))
			tool.Name = def.GetName()
		}
		if !mcpcodec.IsObjectSchema(tool.InputSchema) {
			r.logger.Warn("skip tool with invalid input schema", zap.String("tool", tool.Name))
			continue
		}
		if tool.OutputSchema != nil && !mcpcodec.IsObjectSchema(tool.OutputSchema) {
			r.logger.Warn("skip tool with invalid output schema", zap.String("tool", tool.Name))
			continue
		}

		toAdd = append(toAdd, tool)
		next[tool.Name] = struct{}{}
	}

	var remove []string
	for name := range prev {
		if _, ok := next[name]; !ok {
			remove = append(remove, name)
		}
	}
	for i := range toAdd {
		tool := toAdd[i]
		r.server.AddTool(&tool, r.handler(tool.Name))
	}
	if len(remove) > 0 {
		r.server.RemoveTools(remove...)
	}

	r.mu.Lock()
	r.registered = next
	r.etag = snapshot.GetEtag()
	r.mu.Unlock()
}
