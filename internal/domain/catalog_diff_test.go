package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDiffCatalogStates_SpecChanges(t *testing.T) {
	baseRuntime := RuntimeConfig{
		RouteTimeoutSeconds: 1,
	}

	specA := ServerSpec{Name: "a", Cmd: []string{"echo", "a"}}
	specB := ServerSpec{Name: "b", Cmd: []string{"echo", "b"}}
	specBUpdated := ServerSpec{Name: "b", Cmd: []string{"echo", "b"}, MinReady: 2}
	specAReplaced := ServerSpec{Name: "a", Cmd: []string{"echo", "a", "v2"}}

	prevStore := ProfileStore{
		Profiles: map[string]Profile{
			DefaultProfileName: {
				Name: DefaultProfileName,
				Catalog: Catalog{
					Specs:   map[string]ServerSpec{"alpha": specA, "beta": specB},
					Runtime: baseRuntime,
				},
			},
		},
		Callers: map[string]string{"caller": DefaultProfileName},
	}
	nextStore := ProfileStore{
		Profiles: map[string]Profile{
			DefaultProfileName: {
				Name: DefaultProfileName,
				Catalog: Catalog{
					Specs:   map[string]ServerSpec{"alpha": specAReplaced, "beta": specBUpdated},
					Runtime: baseRuntime,
				},
			},
		},
		Callers: map[string]string{"caller": "default", "new": "default"},
	}

	prevState, err := NewCatalogState(prevStore, 1, time.Now())
	require.NoError(t, err)
	nextState, err := NewCatalogState(nextStore, 2, time.Now())
	require.NoError(t, err)

	diff := DiffCatalogStates(prevState, nextState)

	oldKeyA, err := SpecFingerprint(specA)
	require.NoError(t, err)
	newKeyA, err := SpecFingerprint(specAReplaced)
	require.NoError(t, err)
	keyB, err := SpecFingerprint(specB)
	require.NoError(t, err)

	require.Contains(t, diff.RemovedSpecKeys, oldKeyA)
	require.Contains(t, diff.AddedSpecKeys, newKeyA)
	require.Contains(t, diff.ReplacedSpecKeys, oldKeyA)
	require.Contains(t, diff.UpdatedSpecKeys, keyB)
	require.True(t, diff.CallersChanged)
}
