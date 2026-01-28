package domain

import (
	"container/list"
	"sync"
	"time"
)

// SessionCache tracks per-session state for schema deduplication.
// It uses hash mismatch detection to determine when full schemas need to be resent.
// Implements LRU eviction using a doubly-linked list for O(1) eviction performance.
type SessionCache struct {
	mu      sync.RWMutex
	entries map[string]*list.Element // sessionKey -> list element
	lru     *list.List               // LRU list, front = most recent, back = oldest
	ttl     time.Duration
	maxSize int
}

// lruEntry wraps SessionCacheEntry with LRU metadata.
type lruEntry struct {
	sessionKey string
	entry      *SessionCacheEntry
}

// NewSessionCache creates a new session cache.
func NewSessionCache(ttl time.Duration, maxSize int) *SessionCache {
	return &SessionCache{
		entries: make(map[string]*list.Element),
		lru:     list.New(),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Get retrieves session state for a session key.
// Accessing an entry moves it to the front of the LRU list.
func (c *SessionCache) Get(sessionKey string) (*SessionCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[sessionKey]
	if !ok {
		return nil, false
	}

	entry := elem.Value.(*lruEntry).entry

	// Check TTL expiration
	if time.Since(entry.LastUpdated) > c.ttl {
		return nil, false
	}

	// Move to front (most recently used)
	c.lru.MoveToFront(elem)

	return entry, true
}

// Update updates the session state after sending tools.
func (c *SessionCache) Update(sessionKey string, sentSchemas map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[sessionKey]
	var entry *SessionCacheEntry

	if !ok {
		// New entry
		entry = &SessionCacheEntry{
			SessionKey:  sessionKey,
			SentSchemas: make(map[string]string),
		}
		lruItem := &lruEntry{
			sessionKey: sessionKey,
			entry:      entry,
		}
		elem = c.lru.PushFront(lruItem)
		c.entries[sessionKey] = elem
	} else {
		// Existing entry - move to front (most recently used)
		c.lru.MoveToFront(elem)
		entry = elem.Value.(*lruEntry).entry
	}

	// Merge sent schemas
	for name, hash := range sentSchemas {
		entry.SentSchemas[name] = hash
	}
	entry.LastUpdated = time.Now()
	entry.RequestCount++

	// Enforce max size by evicting oldest entries (O(1) operation)
	for len(c.entries) > c.maxSize {
		c.evictOldest()
	}
}

// Invalidate clears session state for a session key (called on context compression signal).
func (c *SessionCache) Invalidate(sessionKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[sessionKey]
	if !ok {
		return
	}

	c.lru.Remove(elem)
	delete(c.entries, sessionKey)
}

// NeedsFull determines if full schema needs to be sent based on hash mismatch.
// Returns true if:
// - Session has no entry
// - Tool was never sent before
// - Current hash differs from last sent hash (schema changed).
// Accessing an entry moves it to the front of the LRU list.
func (c *SessionCache) NeedsFull(sessionKey, toolName, currentHash string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[sessionKey]
	if !ok {
		return true
	}

	entry := elem.Value.(*lruEntry).entry

	// Check TTL expiration
	if time.Since(entry.LastUpdated) > c.ttl {
		return true
	}

	lastHash, ok := entry.SentSchemas[toolName]
	if !ok {
		return true
	}

	// Move to front (most recently used)
	c.lru.MoveToFront(elem)

	// Hash mismatch detection - if hash changed, resend full schema
	return lastHash != currentHash
}

// Cleanup removes expired entries.
func (c *SessionCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for sessionKey, elem := range c.entries {
		entry := elem.Value.(*lruEntry).entry
		if now.Sub(entry.LastUpdated) > c.ttl {
			c.lru.Remove(elem)
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
// This is now O(1) thanks to the LRU list.
func (c *SessionCache) evictOldest() {
	elem := c.lru.Back()
	if elem == nil {
		return
	}

	lruItem := elem.Value.(*lruEntry)
	c.lru.Remove(elem)
	delete(c.entries, lruItem.sessionKey)
}
