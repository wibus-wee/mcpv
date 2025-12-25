package gateway

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	controlv1 "mcpd/pkg/api/control/v1"
)

type toolRegistry struct {
	server     *mcp.Server
	handler    func(name string) mcp.ToolHandler
	logger     *zap.Logger
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

	r.mu.Lock()
	defer r.mu.Unlock()

	if snapshot.Etag != "" && snapshot.Etag == r.etag {
		return
	}

	next := make(map[string]struct{})
	for _, def := range snapshot.Tools {
		if def == nil || len(def.ToolJson) == 0 {
			continue
		}
		var tool mcp.Tool
		if err := json.Unmarshal(def.ToolJson, &tool); err != nil {
			r.logger.Warn("decode tool failed", zap.String("tool", def.Name), zap.Error(err))
			continue
		}
		if tool.Name == "" {
			tool.Name = def.Name
		}
		if tool.Name == "" {
			continue
		}
		if tool.Name != def.Name && def.Name != "" {
			r.logger.Warn("tool name mismatch", zap.String("tool", tool.Name), zap.String("expected", def.Name))
			tool.Name = def.Name
		}
		if !isObjectSchema(tool.InputSchema) {
			r.logger.Warn("skip tool with invalid input schema", zap.String("tool", tool.Name))
			continue
		}
		if tool.OutputSchema != nil && !isObjectSchema(tool.OutputSchema) {
			r.logger.Warn("skip tool with invalid output schema", zap.String("tool", tool.Name))
			continue
		}

		r.server.AddTool(&tool, r.handler(tool.Name))
		next[tool.Name] = struct{}{}
	}

	var remove []string
	for name := range r.registered {
		if _, ok := next[name]; !ok {
			remove = append(remove, name)
		}
	}
	if len(remove) > 0 {
		r.server.RemoveTools(remove...)
	}

	r.registered = next
	r.etag = snapshot.Etag
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
