package gateway

import (
	"encoding/json"
	"net/url"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	controlv1 "mcpd/pkg/api/control/v1"
)

type resourceRegistry struct {
	server     *mcp.Server
	handler    func(uri string) mcp.ResourceHandler
	logger     *zap.Logger
	mu         sync.Mutex
	etag       string
	registered map[string]struct{}
}

func newResourceRegistry(server *mcp.Server, handler func(uri string) mcp.ResourceHandler, logger *zap.Logger) *resourceRegistry {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &resourceRegistry{
		server:     server,
		handler:    handler,
		logger:     logger.Named("resource_registry"),
		registered: make(map[string]struct{}),
	}
}

func (r *resourceRegistry) ApplySnapshot(snapshot *controlv1.ResourcesSnapshot) {
	if snapshot == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if snapshot.Etag != "" && snapshot.Etag == r.etag {
		return
	}

	next := make(map[string]struct{})
	for _, def := range snapshot.Resources {
		if def == nil || len(def.ResourceJson) == 0 {
			continue
		}
		var resource mcp.Resource
		if err := json.Unmarshal(def.ResourceJson, &resource); err != nil {
			r.logger.Warn("decode resource failed", zap.String("uri", def.Uri), zap.Error(err))
			continue
		}
		if resource.URI == "" {
			resource.URI = def.Uri
		}
		if resource.URI == "" {
			continue
		}
		if def.Uri != "" && resource.URI != def.Uri {
			r.logger.Warn("resource uri mismatch", zap.String("uri", resource.URI), zap.String("expected", def.Uri))
			resource.URI = def.Uri
		}
		if !validResourceURI(resource.URI) {
			r.logger.Warn("skip resource with invalid uri", zap.String("uri", resource.URI))
			continue
		}

		r.server.AddResource(&resource, r.handler(resource.URI))
		next[resource.URI] = struct{}{}
	}

	var remove []string
	for uri := range r.registered {
		if _, ok := next[uri]; !ok {
			remove = append(remove, uri)
		}
	}
	if len(remove) > 0 {
		r.server.RemoveResources(remove...)
	}

	r.registered = next
	r.etag = snapshot.Etag
}

func validResourceURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme != ""
}
