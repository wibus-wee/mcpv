package gateway

import (
	"encoding/json"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	controlv1 "mcpd/pkg/api/control/v1"
)

type promptRegistry struct {
	server     *mcp.Server
	handler    func(name string) mcp.PromptHandler
	logger     *zap.Logger
	mu         sync.Mutex
	etag       string
	registered map[string]struct{}
}

func newPromptRegistry(server *mcp.Server, handler func(name string) mcp.PromptHandler, logger *zap.Logger) *promptRegistry {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &promptRegistry{
		server:     server,
		handler:    handler,
		logger:     logger.Named("prompt_registry"),
		registered: make(map[string]struct{}),
	}
}

func (r *promptRegistry) ApplySnapshot(snapshot *controlv1.PromptsSnapshot) {
	if snapshot == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if snapshot.Etag != "" && snapshot.Etag == r.etag {
		return
	}

	next := make(map[string]struct{})
	for _, def := range snapshot.Prompts {
		if def == nil || len(def.PromptJson) == 0 {
			continue
		}
		var prompt mcp.Prompt
		if err := json.Unmarshal(def.PromptJson, &prompt); err != nil {
			r.logger.Warn("decode prompt failed", zap.String("prompt", def.Name), zap.Error(err))
			continue
		}
		if prompt.Name == "" {
			prompt.Name = def.Name
		}
		if prompt.Name == "" {
			continue
		}
		if def.Name != "" && prompt.Name != def.Name {
			r.logger.Warn("prompt name mismatch", zap.String("prompt", prompt.Name), zap.String("expected", def.Name))
			prompt.Name = def.Name
		}

		r.server.AddPrompt(&prompt, r.handler(prompt.Name))
		next[prompt.Name] = struct{}{}
	}

	var remove []string
	for name := range r.registered {
		if _, ok := next[name]; !ok {
			remove = append(remove, name)
		}
	}
	if len(remove) > 0 {
		r.server.RemovePrompts(remove...)
	}

	r.registered = next
	r.etag = snapshot.Etag
}
