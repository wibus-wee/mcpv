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
	store, err := loader.Load(context.Background(), dir, ProfileStoreOptions{AllowCreate: true})
	require.NoError(t, err)
	profile, ok := store.Profiles[domain.DefaultProfileName]
	require.True(t, ok)
	require.Empty(t, profile.Catalog.Specs)

	_, err = os.Stat(filepath.Join(dir, "callers.yaml"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "profiles", "default.yaml"))
	require.NoError(t, err)
}

func TestProfileStoreLoader_RuntimeOverrideFromStore(t *testing.T) {
	dir := t.TempDir()
	profilesDir := filepath.Join(dir, "profiles")
	require.NoError(t, os.MkdirAll(profilesDir, 0o755))
	writeProfile(t, filepath.Join(profilesDir, "default.yaml"), "default-server")
	writeProfile(t, filepath.Join(profilesDir, "chat.yaml"), "chat-server")

	runtime := `routeTimeoutSeconds: 15
pingIntervalSeconds: 20
toolRefreshSeconds: 45
callerCheckSeconds: 7
exposeTools: false
toolNamespaceStrategy: flat
observability:
  listenAddress: "0.0.0.0:1111"
rpc:
  listenAddress: "unix:///tmp/test.sock"
  maxRecvMsgSize: 256
  maxSendMsgSize: 512
  keepaliveTimeSeconds: 9
  keepaliveTimeoutSeconds: 3
  socketMode: "0660"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "runtime.yaml"), []byte(runtime), 0o644))

	loader := NewProfileStoreLoader(zap.NewNop())
	store, err := loader.Load(context.Background(), dir, ProfileStoreOptions{})
	require.NoError(t, err)
	require.Len(t, store.Profiles, 2)

	for name, profile := range store.Profiles {
		require.Equal(t, 15, profile.Catalog.Runtime.RouteTimeoutSeconds, "profile %s", name)
		require.Equal(t, 20, profile.Catalog.Runtime.PingIntervalSeconds, "profile %s", name)
		require.Equal(t, 45, profile.Catalog.Runtime.ToolRefreshSeconds, "profile %s", name)
		require.Equal(t, 7, profile.Catalog.Runtime.CallerCheckSeconds, "profile %s", name)
		require.False(t, profile.Catalog.Runtime.ExposeTools, "profile %s", name)
		require.Equal(t, "flat", profile.Catalog.Runtime.ToolNamespaceStrategy, "profile %s", name)
		require.Equal(t, "unix:///tmp/test.sock", profile.Catalog.Runtime.RPC.ListenAddress, "profile %s", name)
		require.Equal(t, "0.0.0.0:1111", profile.Catalog.Runtime.Observability.ListenAddress, "profile %s", name)
	}
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
