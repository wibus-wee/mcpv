package uiconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStoreUpdateAndGetGlobal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ui-settings.db")
	store, err := OpenStore(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	updates := map[string]json.RawMessage{
		"serversTable": json.RawMessage(`{"sort":"name"}`),
	}
	snapshot, err := store.Update(ScopeGlobal, "", updates, nil)
	require.NoError(t, err)
	require.Equal(t, ScopeGlobal, snapshot.Scope)
	require.Contains(t, snapshot.Sections, "serversTable")
	require.NotEmpty(t, snapshot.UpdatedAt)

	readBack, err := store.Get(ScopeGlobal, "")
	require.NoError(t, err)
	require.Equal(t, snapshot.Sections, readBack.Sections)
}

func TestStoreWorkspaceOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ui-settings.db")
	store, err := OpenStore(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	_, err = store.Update(ScopeGlobal, "", map[string]json.RawMessage{
		"panel": json.RawMessage(`{"open":false}`),
	}, nil)
	require.NoError(t, err)

	_, err = store.Update(ScopeWorkspace, "workspace-1", map[string]json.RawMessage{
		"panel": json.RawMessage(`{"open":true}`),
	}, nil)
	require.NoError(t, err)

	effective, err := store.GetEffective("workspace-1")
	require.NoError(t, err)
	require.Equal(t, ScopeEffective, effective.Scope)
	require.Contains(t, effective.Sections, "panel")
	require.JSONEq(t, `{"open":true}`, string(effective.Sections["panel"]))
}

func TestStoreResetWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ui-settings.db")
	store, err := OpenStore(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	_, err = store.Update(ScopeWorkspace, "workspace-2", map[string]json.RawMessage{
		"filters": json.RawMessage(`{"value":"active"}`),
	}, nil)
	require.NoError(t, err)

	resetSnapshot, err := store.Reset(ScopeWorkspace, "workspace-2")
	require.NoError(t, err)
	require.Empty(t, resetSnapshot.Sections)
}
