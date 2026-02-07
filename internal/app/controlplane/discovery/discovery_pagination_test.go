package discovery

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"mcpv/internal/domain"
)

func TestPaginateResources_InvalidCursor(t *testing.T) {
	snapshot := domain.ResourceSnapshot{
		ETag: "etag",
		Resources: []domain.ResourceDefinition{
			{URI: "res-1"},
			{URI: "res-2"},
		},
	}

	_, err := paginateResources(snapshot, "missing")
	require.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestPaginateResources_NextCursor(t *testing.T) {
	resources := make([]domain.ResourceDefinition, snapshotPageSize+1)
	for i := range resources {
		resources[i] = domain.ResourceDefinition{URI: fmt.Sprintf("res-%03d", i)}
	}
	snapshot := domain.ResourceSnapshot{ETag: "etag", Resources: resources}

	page, err := paginateResources(snapshot, "")
	require.NoError(t, err)
	require.Len(t, page.Snapshot.Resources, snapshotPageSize)
	require.Equal(t, resources[snapshotPageSize-1].URI, page.NextCursor)

	next, err := paginateResources(snapshot, page.NextCursor)
	require.NoError(t, err)
	require.Len(t, next.Snapshot.Resources, 1)
	require.Empty(t, next.NextCursor)
	require.Equal(t, resources[snapshotPageSize].URI, next.Snapshot.Resources[0].URI)
}

func TestPaginatePrompts_InvalidCursor(t *testing.T) {
	snapshot := domain.PromptSnapshot{
		ETag: "etag",
		Prompts: []domain.PromptDefinition{
			{Name: "prompt-1"},
			{Name: "prompt-2"},
		},
	}

	_, err := paginatePrompts(snapshot, "missing")
	require.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestPaginatePrompts_NextCursor(t *testing.T) {
	prompts := make([]domain.PromptDefinition, snapshotPageSize+1)
	for i := range prompts {
		prompts[i] = domain.PromptDefinition{Name: fmt.Sprintf("prompt-%03d", i)}
	}
	snapshot := domain.PromptSnapshot{ETag: "etag", Prompts: prompts}

	page, err := paginatePrompts(snapshot, "")
	require.NoError(t, err)
	require.Len(t, page.Snapshot.Prompts, snapshotPageSize)
	require.Equal(t, prompts[snapshotPageSize-1].Name, page.NextCursor)

	next, err := paginatePrompts(snapshot, page.NextCursor)
	require.NoError(t, err)
	require.Len(t, next.Snapshot.Prompts, 1)
	require.Empty(t, next.NextCursor)
	require.Equal(t, prompts[snapshotPageSize].Name, next.Snapshot.Prompts[0].Name)
}
