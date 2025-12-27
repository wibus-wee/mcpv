package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"mcpd/internal/domain"
)

func TestPaginateResources_EmptySnapshot(t *testing.T) {
	page, err := paginateResources(domain.ResourceSnapshot{}, "")
	require.NoError(t, err)
	require.Len(t, page.Snapshot.Resources, 0)
	require.Empty(t, page.NextCursor)
}

func TestPaginateResources_InvalidCursor(t *testing.T) {
	snapshot := domain.ResourceSnapshot{
		Resources: []domain.ResourceDefinition{
			{URI: "file:///a"},
		},
	}
	_, err := paginateResources(snapshot, "missing")
	require.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestPaginateResources_LastCursor(t *testing.T) {
	snapshot := domain.ResourceSnapshot{
		Resources: []domain.ResourceDefinition{
			{URI: "file:///a"},
			{URI: "file:///b"},
		},
	}
	page, err := paginateResources(snapshot, "file:///b")
	require.NoError(t, err)
	require.Len(t, page.Snapshot.Resources, 0)
	require.Empty(t, page.NextCursor)
}

func TestPaginateResources_NextCursor(t *testing.T) {
	resources := make([]domain.ResourceDefinition, 0, listPageSize+1)
	for i := 0; i < listPageSize+1; i++ {
		resources = append(resources, domain.ResourceDefinition{
			URI: fmt.Sprintf("file:///r%03d", i),
		})
	}
	snapshot := domain.ResourceSnapshot{Resources: resources}

	page, err := paginateResources(snapshot, "")
	require.NoError(t, err)
	require.Len(t, page.Snapshot.Resources, listPageSize)
	require.Equal(t, resources[listPageSize-1].URI, page.NextCursor)
}

func TestPaginatePrompts_EmptySnapshot(t *testing.T) {
	page, err := paginatePrompts(domain.PromptSnapshot{}, "")
	require.NoError(t, err)
	require.Len(t, page.Snapshot.Prompts, 0)
	require.Empty(t, page.NextCursor)
}

func TestPaginatePrompts_InvalidCursor(t *testing.T) {
	snapshot := domain.PromptSnapshot{
		Prompts: []domain.PromptDefinition{
			{Name: "alpha"},
		},
	}
	_, err := paginatePrompts(snapshot, "missing")
	require.ErrorIs(t, err, domain.ErrInvalidCursor)
}

func TestPaginatePrompts_LastCursor(t *testing.T) {
	snapshot := domain.PromptSnapshot{
		Prompts: []domain.PromptDefinition{
			{Name: "alpha"},
			{Name: "beta"},
		},
	}
	page, err := paginatePrompts(snapshot, "beta")
	require.NoError(t, err)
	require.Len(t, page.Snapshot.Prompts, 0)
	require.Empty(t, page.NextCursor)
}

func TestPaginatePrompts_NextCursor(t *testing.T) {
	prompts := make([]domain.PromptDefinition, 0, listPageSize+1)
	for i := 0; i < listPageSize+1; i++ {
		prompts = append(prompts, domain.PromptDefinition{
			Name: fmt.Sprintf("prompt-%03d", i),
		})
	}
	snapshot := domain.PromptSnapshot{Prompts: prompts}

	page, err := paginatePrompts(snapshot, "")
	require.NoError(t, err)
	require.Len(t, page.Snapshot.Prompts, listPageSize)
	require.Equal(t, prompts[listPageSize-1].Name, page.NextCursor)
}
