package domain

import (
	"sync"
	"time"
)

// MetadataCache provides thread-safe storage for MCP server metadata
// collected during bootstrap. This allows the system to serve tool/resource/prompt
// information even when servers are not running (lazy startup strategy).
type MetadataCache struct {
	mu sync.RWMutex

	tools     map[string][]ToolDefinition     // specKey -> tools
	resources map[string][]ResourceDefinition // specKey -> resources
	prompts   map[string][]PromptDefinition   // specKey -> prompts

	toolETags     map[string]string // specKey -> etag
	resourceETags map[string]string
	promptETags   map[string]string

	cachedAt map[string]time.Time // specKey -> cache timestamp
}

// NewMetadataCache creates a new empty metadata cache.
func NewMetadataCache() *MetadataCache {
	return &MetadataCache{
		tools:         make(map[string][]ToolDefinition),
		resources:     make(map[string][]ResourceDefinition),
		prompts:       make(map[string][]PromptDefinition),
		toolETags:     make(map[string]string),
		resourceETags: make(map[string]string),
		promptETags:   make(map[string]string),
		cachedAt:      make(map[string]time.Time),
	}
}

// SetTools stores tool definitions for a server.
func (c *MetadataCache) SetTools(specKey string, tools []ToolDefinition, etag string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Make a copy to avoid external mutations
	copied := make([]ToolDefinition, len(tools))
	copy(copied, tools)

	c.tools[specKey] = copied
	c.toolETags[specKey] = etag
	c.cachedAt[specKey] = time.Now()
}

// GetTools retrieves cached tool definitions for a server.
func (c *MetadataCache) GetTools(specKey string) ([]ToolDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tools, ok := c.tools[specKey]
	if !ok {
		return nil, false
	}

	// Return a copy
	copied := make([]ToolDefinition, len(tools))
	copy(copied, tools)
	return copied, true
}

// GetToolETag returns the ETag for cached tools.
func (c *MetadataCache) GetToolETag(specKey string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.toolETags[specKey]
}

// SetResources stores resource definitions for a server.
func (c *MetadataCache) SetResources(specKey string, resources []ResourceDefinition, etag string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	copied := make([]ResourceDefinition, len(resources))
	copy(copied, resources)

	c.resources[specKey] = copied
	c.resourceETags[specKey] = etag
	c.cachedAt[specKey] = time.Now()
}

// GetResources retrieves cached resource definitions for a server.
func (c *MetadataCache) GetResources(specKey string) ([]ResourceDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	resources, ok := c.resources[specKey]
	if !ok {
		return nil, false
	}

	copied := make([]ResourceDefinition, len(resources))
	copy(copied, resources)
	return copied, true
}

// GetResourceETag returns the ETag for cached resources.
func (c *MetadataCache) GetResourceETag(specKey string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.resourceETags[specKey]
}

// SetPrompts stores prompt definitions for a server.
func (c *MetadataCache) SetPrompts(specKey string, prompts []PromptDefinition, etag string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	copied := make([]PromptDefinition, len(prompts))
	copy(copied, prompts)

	c.prompts[specKey] = copied
	c.promptETags[specKey] = etag
	c.cachedAt[specKey] = time.Now()
}

// GetPrompts retrieves cached prompt definitions for a server.
func (c *MetadataCache) GetPrompts(specKey string) ([]PromptDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	prompts, ok := c.prompts[specKey]
	if !ok {
		return nil, false
	}

	copied := make([]PromptDefinition, len(prompts))
	copy(copied, prompts)
	return copied, true
}

// GetPromptETag returns the ETag for cached prompts.
func (c *MetadataCache) GetPromptETag(specKey string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.promptETags[specKey]
}

// GetCachedAt returns when a server's metadata was cached.
func (c *MetadataCache) GetCachedAt(specKey string) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.cachedAt[specKey]
	return t, ok
}

// HasTools returns true if tools are cached for the given specKey.
func (c *MetadataCache) HasTools(specKey string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.tools[specKey]
	return ok
}

// HasResources returns true if resources are cached for the given specKey.
func (c *MetadataCache) HasResources(specKey string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.resources[specKey]
	return ok
}

// HasPrompts returns true if prompts are cached for the given specKey.
func (c *MetadataCache) HasPrompts(specKey string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.prompts[specKey]
	return ok
}

// Clear removes all cached metadata.
func (c *MetadataCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tools = make(map[string][]ToolDefinition)
	c.resources = make(map[string][]ResourceDefinition)
	c.prompts = make(map[string][]PromptDefinition)
	c.toolETags = make(map[string]string)
	c.resourceETags = make(map[string]string)
	c.promptETags = make(map[string]string)
	c.cachedAt = make(map[string]time.Time)
}

// ClearSpec removes cached metadata for a specific server.
func (c *MetadataCache) ClearSpec(specKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.tools, specKey)
	delete(c.resources, specKey)
	delete(c.prompts, specKey)
	delete(c.toolETags, specKey)
	delete(c.resourceETags, specKey)
	delete(c.promptETags, specKey)
	delete(c.cachedAt, specKey)
}

// GetAllTools returns all cached tools across all servers.
func (c *MetadataCache) GetAllTools() []ToolDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var all []ToolDefinition
	for _, tools := range c.tools {
		for _, tool := range tools {
			all = append(all, CloneToolDefinition(tool))
		}
	}
	return all
}

// GetAllResources returns all cached resources across all servers.
func (c *MetadataCache) GetAllResources() []ResourceDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var all []ResourceDefinition
	for _, resources := range c.resources {
		for _, resource := range resources {
			all = append(all, CloneResourceDefinition(resource))
		}
	}
	return all
}

// GetAllPrompts returns all cached prompts across all servers.
func (c *MetadataCache) GetAllPrompts() []PromptDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var all []PromptDefinition
	for _, prompts := range c.prompts {
		for _, prompt := range prompts {
			all = append(all, ClonePromptDefinition(prompt))
		}
	}
	return all
}

// SpecKeys returns all specKeys that have cached metadata.
func (c *MetadataCache) SpecKeys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	seen := make(map[string]struct{})
	for key := range c.tools {
		seen[key] = struct{}{}
	}
	for key := range c.resources {
		seen[key] = struct{}{}
	}
	for key := range c.prompts {
		seen[key] = struct{}{}
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	return keys
}

// Stats returns cache statistics.
func (c *MetadataCache) Stats() MetadataCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalTools := 0
	for _, tools := range c.tools {
		totalTools += len(tools)
	}

	totalResources := 0
	for _, resources := range c.resources {
		totalResources += len(resources)
	}

	totalPrompts := 0
	for _, prompts := range c.prompts {
		totalPrompts += len(prompts)
	}

	return MetadataCacheStats{
		ServerCount:   len(c.cachedAt),
		ToolCount:     totalTools,
		ResourceCount: totalResources,
		PromptCount:   totalPrompts,
	}
}

// MetadataCacheStats provides statistics about the cache contents.
type MetadataCacheStats struct {
	ServerCount   int
	ToolCount     int
	ResourceCount int
	PromptCount   int
}
