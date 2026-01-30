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

	prevCatalog := Catalog{
		Specs:   map[string]ServerSpec{"alpha": specA, "beta": specB},
		Runtime: baseRuntime,
	}
	nextCatalog := Catalog{
		Specs:   map[string]ServerSpec{"alpha": specAReplaced, "beta": specBUpdated},
		Runtime: baseRuntime,
	}

	prevState, err := NewCatalogState(prevCatalog, 1, time.Now())
	require.NoError(t, err)
	nextState, err := NewCatalogState(nextCatalog, 2, time.Now())
	require.NoError(t, err)

	diff := DiffCatalogStates(prevState, nextState)

	oldKeyA := SpecFingerprint(specA)
	newKeyA := SpecFingerprint(specAReplaced)
	keyB := SpecFingerprint(specB)

	require.Contains(t, diff.RemovedSpecKeys, oldKeyA)
	require.Contains(t, diff.AddedSpecKeys, newKeyA)
	require.Contains(t, diff.ReplacedSpecKeys, oldKeyA)
	require.Contains(t, diff.UpdatedSpecKeys, keyB)
	require.Contains(t, diff.RestartRequiredSpecKeys, keyB)
	require.Empty(t, diff.ToolsOnlySpecKeys)
	require.False(t, diff.TagsChanged)
	require.False(t, diff.RuntimeChanged)
}

func TestClassifySpecDiff(t *testing.T) {
	base := ServerSpec{
		Name:        "svc",
		Cmd:         []string{"echo", "ok"},
		Tags:        []string{"chat"},
		ExposeTools: []string{"calc"},
	}

	onlyTags := base
	onlyTags.Tags = []string{"chat", "ops"}
	require.Equal(t, SpecDiffToolsOnly, ClassifySpecDiff(base, onlyTags))

	onlyExpose := base
	onlyExpose.ExposeTools = []string{"calc", "convert"}
	require.Equal(t, SpecDiffToolsOnly, ClassifySpecDiff(base, onlyExpose))

	onlyName := base
	onlyName.Name = "svc-renamed"
	require.Equal(t, SpecDiffToolsOnly, ClassifySpecDiff(base, onlyName))

	cmdChanged := base
	cmdChanged.Cmd = []string{"echo", "v2"}
	require.Equal(t, SpecDiffRestartRequired, ClassifySpecDiff(base, cmdChanged))
}
