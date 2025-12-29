package ui

import (
	"context"
	"sync"
	"time"

	"mcpd/internal/domain"
)

// SharedState holds cached data and subscription tracking
type SharedState struct {
	mu sync.RWMutex

	// Cached snapshots
	toolSnapshot     domain.ToolSnapshot
	resourceSnapshot domain.ResourceSnapshot
	promptSnapshot   domain.PromptSnapshot

	// Subscription tracking
	activeWatches map[string]*WatchSubscription
}

// WatchSubscription represents an active Watch subscription
type WatchSubscription struct {
	ID        string
	Type      string // "tools", "resources", "prompts", "logs"
	StartedAt time.Time
	Cancel    context.CancelFunc
}

// NewSharedState creates a new SharedState instance
func NewSharedState() *SharedState {
	return &SharedState{
		activeWatches: make(map[string]*WatchSubscription),
	}
}

// GetToolSnapshot returns a copy of the cached tool snapshot
func (s *SharedState) GetToolSnapshot() domain.ToolSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyToolSnapshot(s.toolSnapshot)
}

// SetToolSnapshot updates the cached tool snapshot
func (s *SharedState) SetToolSnapshot(snapshot domain.ToolSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolSnapshot = snapshot
}

// GetResourceSnapshot returns a copy of the cached resource snapshot
func (s *SharedState) GetResourceSnapshot() domain.ResourceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyResourceSnapshot(s.resourceSnapshot)
}

// SetResourceSnapshot updates the cached resource snapshot
func (s *SharedState) SetResourceSnapshot(snapshot domain.ResourceSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resourceSnapshot = snapshot
}

// GetPromptSnapshot returns a copy of the cached prompt snapshot
func (s *SharedState) GetPromptSnapshot() domain.PromptSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyPromptSnapshot(s.promptSnapshot)
}

// SetPromptSnapshot updates the cached prompt snapshot
func (s *SharedState) SetPromptSnapshot(snapshot domain.PromptSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.promptSnapshot = snapshot
}

// RegisterWatch adds a watch subscription to the registry
func (s *SharedState) RegisterWatch(sub *WatchSubscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeWatches[sub.ID] = sub
}

// UnregisterWatch removes a watch subscription from the registry
func (s *SharedState) UnregisterWatch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeWatches, id)
}

// GetActiveWatches returns a copy of all active watch subscriptions
func (s *SharedState) GetActiveWatches() []WatchSubscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	watches := make([]WatchSubscription, 0, len(s.activeWatches))
	for _, sub := range s.activeWatches {
		watches = append(watches, *sub)
	}
	return watches
}

// CancelAllWatches cancels all active watch subscriptions
func (s *SharedState) CancelAllWatches() {
	s.mu.Lock()
	watches := s.activeWatches
	s.activeWatches = make(map[string]*WatchSubscription)
	s.mu.Unlock()

	for _, sub := range watches {
		if sub.Cancel != nil {
			sub.Cancel()
		}
	}
}

// Helper functions to deep copy snapshots

func copyToolSnapshot(snapshot domain.ToolSnapshot) domain.ToolSnapshot {
	tools := make([]domain.ToolDefinition, 0, len(snapshot.Tools))
	for _, tool := range snapshot.Tools {
		raw := make([]byte, len(tool.ToolJSON))
		copy(raw, tool.ToolJSON)
		tools = append(tools, domain.ToolDefinition{
			Name:       tool.Name,
			ToolJSON:   raw,
			SpecKey:    tool.SpecKey,
			ServerName: tool.ServerName,
		})
	}
	return domain.ToolSnapshot{
		ETag:  snapshot.ETag,
		Tools: tools,
	}
}

func copyResourceSnapshot(snapshot domain.ResourceSnapshot) domain.ResourceSnapshot {
	resources := make([]domain.ResourceDefinition, len(snapshot.Resources))
	copy(resources, snapshot.Resources)
	return domain.ResourceSnapshot{
		ETag:      snapshot.ETag,
		Resources: resources,
	}
}

func copyPromptSnapshot(snapshot domain.PromptSnapshot) domain.PromptSnapshot {
	prompts := make([]domain.PromptDefinition, len(snapshot.Prompts))
	copy(prompts, snapshot.Prompts)
	return domain.PromptSnapshot{
		ETag:    snapshot.ETag,
		Prompts: prompts,
	}
}
