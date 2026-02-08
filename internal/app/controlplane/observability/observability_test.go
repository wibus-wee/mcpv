package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mcpv/internal/domain"
)

func TestFilterRuntimeStatusSnapshot_FiltersAndSorts(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	snapshot := domain.RuntimeStatusSnapshot{
		GeneratedAt: now,
		Statuses: []domain.ServerRuntimeStatus{
			{SpecKey: "b", ServerName: "server-b", Stats: domain.PoolStats{Total: 2}},
			{SpecKey: "a", ServerName: "server-a", Stats: domain.PoolStats{Total: 1}},
			{SpecKey: "c", ServerName: "server-c", Stats: domain.PoolStats{Total: 3}},
		},
	}
	visible := map[string]struct{}{"a": {}, "b": {}}

	filtered := filterRuntimeStatusSnapshot(snapshot, visible)

	expected := []domain.ServerRuntimeStatus{
		{SpecKey: "a", ServerName: "server-a", Stats: domain.PoolStats{Total: 1}},
		{SpecKey: "b", ServerName: "server-b", Stats: domain.PoolStats{Total: 2}},
	}
	data, err := json.Marshal(expected)
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	expectedETag := hex.EncodeToString(sum[:])

	require.Equal(t, now, filtered.GeneratedAt)
	require.Equal(t, expected, filtered.Statuses)
	require.Equal(t, expectedETag, filtered.ETag)
}

func TestFilterRuntimeStatusSnapshot_EmptyVisibility(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	snapshot := domain.RuntimeStatusSnapshot{
		GeneratedAt: now,
		Statuses: []domain.ServerRuntimeStatus{
			{SpecKey: "a", ServerName: "server-a"},
		},
	}

	filtered := filterRuntimeStatusSnapshot(snapshot, map[string]struct{}{})

	require.Equal(t, now, filtered.GeneratedAt)
	require.Empty(t, filtered.Statuses)
	require.Empty(t, filtered.ETag)
}

func TestFilterServerInitStatusSnapshot_FiltersAndSorts(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	snapshot := domain.ServerInitStatusSnapshot{
		GeneratedAt: now,
		Statuses: []domain.ServerInitStatus{
			{SpecKey: "b", ServerName: "server-b", State: domain.ServerInitStarting},
			{SpecKey: "a", ServerName: "server-a", State: domain.ServerInitReady},
			{SpecKey: "c", ServerName: "server-c", State: domain.ServerInitFailed},
		},
	}
	visible := map[string]struct{}{"a": {}, "c": {}}

	filtered := filterServerInitStatusSnapshot(snapshot, visible)

	expected := []domain.ServerInitStatus{
		{SpecKey: "a", ServerName: "server-a", State: domain.ServerInitReady},
		{SpecKey: "c", ServerName: "server-c", State: domain.ServerInitFailed},
	}
	require.Equal(t, now, filtered.GeneratedAt)
	require.Equal(t, expected, filtered.Statuses)
}
