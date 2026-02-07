package discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

func TestBuildToolCatalogSnapshot_MergesSources(t *testing.T) {
	cachedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	liveTools := []domain.ToolDefinition{
		{
			Name:       "srv.echo",
			SpecKey:    "spec-live",
			ServerName: "server-live",
		},
	}
	cachedTools := []domain.ToolDefinition{
		{
			Name:       "srv.echo",
			SpecKey:    "spec-live",
			ServerName: "server-live",
		},
		{
			Name:       "srv.cached",
			SpecKey:    "spec-cache",
			ServerName: "server-cache",
		},
	}

	snapshot := buildToolCatalogSnapshot(zap.NewNop(), liveTools, cachedTools, map[string]time.Time{
		"spec-cache": cachedAt,
	})

	require.Len(t, snapshot.Tools, 2)

	entries := make(map[string]domain.ToolCatalogEntry, len(snapshot.Tools))
	for _, entry := range snapshot.Tools {
		entries[entry.Definition.Name] = entry
	}

	liveEntry := entries["srv.echo"]
	require.Equal(t, domain.ToolSourceLive, liveEntry.Source)
	require.True(t, liveEntry.CachedAt.IsZero())

	cachedEntry := entries["srv.cached"]
	require.Equal(t, domain.ToolSourceCache, cachedEntry.Source)
	require.Equal(t, cachedAt, cachedEntry.CachedAt)
}
