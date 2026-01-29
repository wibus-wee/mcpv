package domain

import (
	"sync"
	"time"
)

const defaultMetadataCacheTTL = 24 * time.Hour

// MetadataCache provides thread-safe storage for MCP server metadata
// collected during bootstrap. This allows the system to serve tool/resource/prompt
// information even when servers are not running (lazy startup strategy).
type MetadataCache struct {
	mu sync.RWMutex
	ttl time.Duration

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
	return NewMetadataCacheWithTTL(defaultMetadataCacheTTL)
}

// NewMetadataCacheWithTTL creates a metadata cache with a TTL.
// A non-positive TTL disables expiration.
func NewMetadataCacheWithTTL(ttl time.Duration) *MetadataCache {
	return &MetadataCache{
		tools:         make(map[string][]ToolDefinition),
		resources:     make(map[string][]ResourceDefinition),
		prompts:       make(map[string][]PromptDefinition),
		toolETags:     make(map[string]string),
		resourceETags: make(map[string]string),
		promptETags:   make(map[string]string),
		cachedAt:      make(map[string]time.Time),
		ttl:           ttl,
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
	return c.getToolsAt(specKey, time.Now())
}

func (c *MetadataCache) getToolsAt(specKey string, now time.Time) ([]ToolDefinition, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isExpiredLocked(specKey, now) {
		c.clearSpecLocked(specKey)
		return nil, false
	}

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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isExpiredLocked(specKey, time.Now()) {
		c.clearSpecLocked(specKey)
		return ""
	}
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isExpiredLocked(specKey, time.Now()) {
		c.clearSpecLocked(specKey)
		return nil, false
	}

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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isExpiredLocked(specKey, time.Now()) {
		c.clearSpecLocked(specKey)
		return ""
	}
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isExpiredLocked(specKey, time.Now()) {
		c.clearSpecLocked(specKey)
		return nil, false
	}

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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isExpiredLocked(specKey, time.Now()) {
		c.clearSpecLocked(specKey)
		return ""
	}
	return c.promptETags[specKey]
}

// GetCachedAt returns when a server's metadata was cached.
func (c *MetadataCache) GetCachedAt(specKey string) (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isExpiredLocked(specKey, time.Now()) {
		c.clearSpecLocked(specKey)
		return time.Time{}, false
	}
	t, ok := c.cachedAt[specKey]
	return t, ok
}

// HasTools returns true if tools are cached for the given specKey.
func (c *MetadataCache) HasTools(specKey string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isExpiredLocked(specKey, time.Now()) {
		c.clearSpecLocked(specKey)
		return false
	}
	_, ok := c.tools[specKey]
	return ok
}

// HasResources returns true if resources are cached for the given specKey.
func (c *MetadataCache) HasResources(specKey string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isExpiredLocked(specKey, time.Now()) {
		c.clearSpecLocked(specKey)
		return false
	}
	_, ok := c.resources[specKey]
	return ok
}

// HasPrompts returns true if prompts are cached for the given specKey.
func (c *MetadataCache) HasPrompts(specKey string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isExpiredLocked(specKey, time.Now()) {
		c.clearSpecLocked(specKey)
		return false
	}
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
	c.clearSpecLocked(specKey)
}

// GetAllTools returns all cached tools across all servers.
func (c *MetadataCache) GetAllTools() []ToolDefinition {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.purgeExpiredLocked(time.Now())

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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.purgeExpiredLocked(time.Now())

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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.purgeExpiredLocked(time.Now())

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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.purgeExpiredLocked(time.Now())

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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.purgeExpiredLocked(time.Now())

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

func (c *MetadataCache) isExpiredLocked(specKey string, now time.Time) bool {
	if c.ttl <= 0 {
		return false
	}
	cachedAt, ok := c.cachedAt[specKey]
	if !ok {
		return true
	}
	return now.Sub(cachedAt) > c.ttl
}

func (c *MetadataCache) purgeExpiredLocked(now time.Time) {
	if c.ttl <= 0 {
		return
	}
	for specKey, cachedAt := range c.cachedAt {
		if now.Sub(cachedAt) > c.ttl {
			c.clearSpecLocked(specKey)
		}
	}
}

func (c *MetadataCache) clearSpecLocked(specKey string) {
	delete(c.tools, specKey)
	delete(c.resources, specKey)
	delete(c.prompts, specKey)
	delete(c.toolETags, specKey)
	delete(c.resourceETags, specKey)
	delete(c.promptETags, specKey)
	delete(c.cachedAt, specKey)
}

// MetadataCacheStats provides statistics about the cache contents.
type MetadataCacheStats struct {
	ServerCount   int
	ToolCount     int
	ResourceCount int
	PromptCount   int
}
