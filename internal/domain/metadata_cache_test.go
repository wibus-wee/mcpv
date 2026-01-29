package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetadataCache_Tools(t *testing.T) {
	cache := NewMetadataCache()

	// Test empty cache
	_, ok := cache.GetTools("spec-1")
	require.False(t, ok)

	// Test set and get
	tools := []ToolDefinition{
		{Name: "tool1", Description: "Test tool 1"},
		{Name: "tool2", Description: "Test tool 2"},
	}
	cache.SetTools("spec-1", tools, "etag-1")

	retrieved, ok := cache.GetTools("spec-1")
	require.True(t, ok)
	require.Len(t, retrieved, 2)
	require.Equal(t, "tool1", retrieved[0].Name)
	require.Equal(t, "tool2", retrieved[1].Name)

	// Test that returned slice is a copy (modification doesn't affect cache)
	retrieved[0].Name = "modified"
	retrieved2, ok := cache.GetTools("spec-1")
	require.True(t, ok)
	require.Equal(t, "tool1", retrieved2[0].Name)

	// Test different spec key
	_, ok = cache.GetTools("spec-2")
	require.False(t, ok)

	// Test update existing entry
	newTools := []ToolDefinition{
		{Name: "tool3", Description: "Test tool 3"},
	}
	cache.SetTools("spec-1", newTools, "etag-2")

	retrieved3, ok := cache.GetTools("spec-1")
	require.True(t, ok)
	require.Len(t, retrieved3, 1)
	require.Equal(t, "tool3", retrieved3[0].Name)
}

func TestMetadataCache_Resources(t *testing.T) {
	cache := NewMetadataCache()

	// Test empty cache
	_, ok := cache.GetResources("spec-1")
	require.False(t, ok)

	// Test set and get
	resources := []ResourceDefinition{
		{URI: "file://test1", Name: "Test Resource 1"},
		{URI: "file://test2", Name: "Test Resource 2"},
	}
	cache.SetResources("spec-1", resources, "etag-1")

	retrieved, ok := cache.GetResources("spec-1")
	require.True(t, ok)
	require.Len(t, retrieved, 2)
	require.Equal(t, "file://test1", retrieved[0].URI)
	require.Equal(t, "file://test2", retrieved[1].URI)

	// Test that returned slice is a copy
	retrieved[0].URI = "modified"
	retrieved2, ok := cache.GetResources("spec-1")
	require.True(t, ok)
	require.Equal(t, "file://test1", retrieved2[0].URI)
}

func TestMetadataCache_Prompts(t *testing.T) {
	cache := NewMetadataCache()

	// Test empty cache
	_, ok := cache.GetPrompts("spec-1")
	require.False(t, ok)

	// Test set and get
	prompts := []PromptDefinition{
		{Name: "prompt1", Description: "Test prompt 1"},
		{Name: "prompt2", Description: "Test prompt 2"},
	}
	cache.SetPrompts("spec-1", prompts, "etag-1")

	retrieved, ok := cache.GetPrompts("spec-1")
	require.True(t, ok)
	require.Len(t, retrieved, 2)
	require.Equal(t, "prompt1", retrieved[0].Name)
	require.Equal(t, "prompt2", retrieved[1].Name)

	// Test that returned slice is a copy
	retrieved[0].Name = "modified"
	retrieved2, ok := cache.GetPrompts("spec-1")
	require.True(t, ok)
	require.Equal(t, "prompt1", retrieved2[0].Name)
}

func TestMetadataCache_MultipleSpecKeys(t *testing.T) {
	cache := NewMetadataCache()

	// Set different tools for different spec keys
	tools1 := []ToolDefinition{
		{Name: "tool1", Description: "Spec 1 tool"},
	}
	tools2 := []ToolDefinition{
		{Name: "tool2", Description: "Spec 2 tool"},
	}

	cache.SetTools("spec-1", tools1, "etag-1")
	cache.SetTools("spec-2", tools2, "etag-2")

	// Verify both are stored independently
	retrieved1, ok := cache.GetTools("spec-1")
	require.True(t, ok)
	require.Len(t, retrieved1, 1)
	require.Equal(t, "tool1", retrieved1[0].Name)

	retrieved2, ok := cache.GetTools("spec-2")
	require.True(t, ok)
	require.Len(t, retrieved2, 1)
	require.Equal(t, "tool2", retrieved2[0].Name)
}

func TestMetadataCache_EmptySlices(t *testing.T) {
	cache := NewMetadataCache()

	// Test setting empty slices
	cache.SetTools("spec-1", []ToolDefinition{}, "etag-1")
	cache.SetResources("spec-2", []ResourceDefinition{}, "etag-2")
	cache.SetPrompts("spec-3", []PromptDefinition{}, "etag-3")

	// Verify we can retrieve empty slices
	tools, ok := cache.GetTools("spec-1")
	require.True(t, ok)
	require.Len(t, tools, 0)

	resources, ok := cache.GetResources("spec-2")
	require.True(t, ok)
	require.Len(t, resources, 0)

	prompts, ok := cache.GetPrompts("spec-3")
	require.True(t, ok)
	require.Len(t, prompts, 0)
}

func TestMetadataCache_TTLExpiration(t *testing.T) {
	cache := NewMetadataCacheWithTTL(10 * time.Millisecond)

	tools := []ToolDefinition{{Name: "tool1"}}
	resources := []ResourceDefinition{{URI: "file://test"}}
	prompts := []PromptDefinition{{Name: "prompt1"}}

	cache.SetTools("spec-1", tools, "etag-tools")
	cache.SetResources("spec-1", resources, "etag-resources")
	cache.SetPrompts("spec-1", prompts, "etag-prompts")

	require.True(t, cache.HasTools("spec-1"))

	time.Sleep(15 * time.Millisecond)

	_, ok := cache.GetTools("spec-1")
	require.False(t, ok)
	require.False(t, cache.HasResources("spec-1"))
	require.False(t, cache.HasPrompts("spec-1"))
	require.Empty(t, cache.GetToolETag("spec-1"))
	require.Empty(t, cache.GetResourceETag("spec-1"))
	require.Empty(t, cache.GetPromptETag("spec-1"))
	_, ok = cache.GetCachedAt("spec-1")
	require.False(t, ok)
	require.Equal(t, 0, cache.Stats().ServerCount)
}
