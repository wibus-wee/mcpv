package domain

import (
	"sync"
	"time"
)

// SessionCache tracks per-session state for schema deduplication.
// It uses hash mismatch detection to determine when full schemas need to be resent.
type SessionCache struct {
	mu      sync.RWMutex
	entries map[string]*SessionCacheEntry
	ttl     time.Duration
	maxSize int
}

// NewSessionCache creates a new session cache.
func NewSessionCache(ttl time.Duration, maxSize int) *SessionCache {
	return &SessionCache{
		entries: make(map[string]*SessionCacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Get retrieves session state for a session key.
func (c *SessionCache) Get(sessionKey string) (*SessionCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[sessionKey]
	if !ok {
		return nil, false
	}

	// Check TTL expiration
	if time.Since(entry.LastUpdated) > c.ttl {
		return nil, false
	}

	return entry, true
}

// Update updates the session state after sending tools.
func (c *SessionCache) Update(sessionKey string, sentSchemas map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[sessionKey]
	if !ok {
		entry = &SessionCacheEntry{
			SessionKey:  sessionKey,
			SentSchemas: make(map[string]string),
		}
		c.entries[sessionKey] = entry
	}

	// Merge sent schemas
	for name, hash := range sentSchemas {
		entry.SentSchemas[name] = hash
	}
	entry.LastUpdated = time.Now()
	entry.RequestCount++

	// Enforce max size by evicting oldest entries
	if len(c.entries) > c.maxSize {
		c.evictOldest()
	}
}

// Invalidate clears session state for a session key (called on context compression signal).
func (c *SessionCache) Invalidate(sessionKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, sessionKey)
}

// NeedsFull determines if full schema needs to be sent based on hash mismatch.
// Returns true if:
// - Session has no entry
// - Tool was never sent before
// - Current hash differs from last sent hash (schema changed).
func (c *SessionCache) NeedsFull(sessionKey, toolName, currentHash string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[sessionKey]
	if !ok {
		return true
	}

	// Check TTL expiration
	if time.Since(entry.LastUpdated) > c.ttl {
		return true
	}

	lastHash, ok := entry.SentSchemas[toolName]
	if !ok {
		return true
	}

	// Hash mismatch detection - if hash changed, resend full schema
	return lastHash != currentHash
}

// Cleanup removes expired entries.
func (c *SessionCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for sessionKey, entry := range c.entries {
		if now.Sub(entry.LastUpdated) > c.ttl {
			delete(c.entries, sessionKey)
		}
	}
}

// Size returns the current number of entries in the cache.
func (c *SessionCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// evictOldest removes the oldest entry (must be called with lock held).
func (c *SessionCache) evictOldest() {
	var oldestID string
	var oldestTime time.Time

	for sessionKey, entry := range c.entries {
		if oldestID == "" || entry.LastUpdated.Before(oldestTime) {
			oldestID = sessionKey
			oldestTime = entry.LastUpdated
		}
	}

	if oldestID != "" {
		delete(c.entries, oldestID)
	}
}
