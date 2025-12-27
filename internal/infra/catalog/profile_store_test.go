package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpd/internal/domain"
)

func TestProfileStoreLoader_LoadFromFile(t *testing.T) {
	file := writeTempConfig(t, `
servers:
  - name: git-helper
    cmd: ["./git-helper"]
    idleSeconds: 60
    maxConcurrent: 2
    sticky: false
    persistent: false
    minReady: 0
    protocolVersion: "2025-11-25"
`)

	loader := NewProfileStoreLoader(zap.NewNop())
	store, err := loader.Load(context.Background(), file, ProfileStoreOptions{})
	require.NoError(t, err)

	require.Len(t, store.Profiles, 1)
	profile, ok := store.Profiles[domain.DefaultProfileName]
	require.True(t, ok)
	require.Len(t, profile.Catalog.Specs, 1)
	require.Empty(t, store.Callers)
}

func TestProfileStoreLoader_LoadFromDir(t *testing.T) {
	dir := t.TempDir()
	profilesDir := filepath.Join(dir, "profiles")
	require.NoError(t, os.MkdirAll(profilesDir, 0o755))

	writeProfile(t, filepath.Join(profilesDir, "default.yaml"), "default-server")
	writeProfile(t, filepath.Join(profilesDir, "vscode.yml"), "vscode-server")
	callers := []byte("callers:\n  vscode: vscode\n  default-client: default\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "callers.yaml"), callers, 0o644))

	loader := NewProfileStoreLoader(zap.NewNop())
	store, err := loader.Load(context.Background(), dir, ProfileStoreOptions{})
	require.NoError(t, err)

	require.Len(t, store.Profiles, 2)
	require.Contains(t, store.Profiles, domain.DefaultProfileName)
	require.Contains(t, store.Profiles, "vscode")
	require.Equal(t, "vscode", store.Callers["vscode"])
	require.Equal(t, "default", store.Callers["default-client"])
}

func TestProfileStoreLoader_MissingDefaultProfile(t *testing.T) {
	dir := t.TempDir()
	profilesDir := filepath.Join(dir, "profiles")
	require.NoError(t, os.MkdirAll(profilesDir, 0o755))
	writeProfile(t, filepath.Join(profilesDir, "vscode.yaml"), "vscode-server")

	loader := NewProfileStoreLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), dir, ProfileStoreOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "default profile")
}

func TestProfileStoreLoader_InvalidCallerMapping(t *testing.T) {
	dir := t.TempDir()
	profilesDir := filepath.Join(dir, "profiles")
	require.NoError(t, os.MkdirAll(profilesDir, 0o755))
	writeProfile(t, filepath.Join(profilesDir, "default.yaml"), "default-server")

	callers := []byte("callers:\n  vscode: missing\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "callers.yaml"), callers, 0o644))

	loader := NewProfileStoreLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), dir, ProfileStoreOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown profile")
}

func TestProfileStoreLoader_AllowCreate(t *testing.T) {
	dir := t.TempDir()

	loader := NewProfileStoreLoader(zap.NewNop())
	_, err := loader.Load(context.Background(), dir, ProfileStoreOptions{AllowCreate: true})
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(dir, "callers.yaml"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "profiles", "default.yaml"))
	require.NoError(t, err)
}

func writeProfile(t *testing.T, path string, name string) {
	t.Helper()

	content := `servers:
  - name: ` + name + `
    cmd: ["./` + name + `"]
    idleSeconds: 0
    maxConcurrent: 1
    sticky: false
    persistent: false
    minReady: 0
    protocolVersion: "2025-11-25"
`

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
