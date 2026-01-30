package gateway

import (
	"encoding/json"
	"net/url"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	controlv1 "mcpv/pkg/api/control/v1"
)

type resourceRegistry struct {
	server     *mcp.Server
	handler    func(uri string) mcp.ResourceHandler
	logger     *zap.Logger
	applyMu    sync.Mutex
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

	r.applyMu.Lock()
	defer r.applyMu.Unlock()

	r.mu.Lock()
	if snapshot.GetEtag() != "" && snapshot.GetEtag() == r.etag {
		r.mu.Unlock()
		return
	}
	prev := make(map[string]struct{}, len(r.registered))
	for uri := range r.registered {
		prev[uri] = struct{}{}
	}
	r.mu.Unlock()

	next := make(map[string]struct{})
	toAdd := make([]mcp.Resource, 0, len(snapshot.GetResources()))
	for _, def := range snapshot.GetResources() {
		if def == nil || len(def.GetResourceJson()) == 0 {
			continue
		}
		var resource mcp.Resource
		if err := json.Unmarshal(def.GetResourceJson(), &resource); err != nil {
			r.logger.Warn("decode resource failed", zap.String("uri", def.GetUri()), zap.Error(err))
			continue
		}
		if resource.URI == "" {
			resource.URI = def.GetUri()
		}
		if resource.URI == "" {
			continue
		}
		if def.GetUri() != "" && resource.URI != def.GetUri() {
			r.logger.Warn("resource uri mismatch", zap.String("uri", resource.URI), zap.String("expected", def.GetUri()))
			resource.URI = def.GetUri()
		}
		if !validResourceURI(resource.URI) {
			r.logger.Warn("skip resource with invalid uri", zap.String("uri", resource.URI))
			continue
		}

		toAdd = append(toAdd, resource)
		next[resource.URI] = struct{}{}
	}

	var remove []string
	for uri := range prev {
		if _, ok := next[uri]; !ok {
			remove = append(remove, uri)
		}
	}
	for i := range toAdd {
		resource := toAdd[i]
		r.server.AddResource(&resource, r.handler(resource.URI))
	}
	if len(remove) > 0 {
		r.server.RemoveResources(remove...)
	}

	r.mu.Lock()
	r.registered = next
	r.etag = snapshot.GetEtag()
	r.mu.Unlock()
}

func validResourceURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme != ""
}
