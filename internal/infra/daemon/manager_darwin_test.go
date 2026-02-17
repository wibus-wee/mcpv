//go:build darwin

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	calls []string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) (string, int, error) {
	r.calls = append(r.calls, strings.Join(append([]string{name}, args...), " "))
	return "", 1, os.ErrNotExist
}

func TestManagerInstall_WritesPlist(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	binPath := filepath.Join(tempDir, "mcpv")
	require.NoError(t, os.WriteFile(binPath, []byte(""), 0o755))

	manager, err := NewManager(Options{
		BinaryPath: binPath,
		ConfigPath: filepath.Join(tempDir, "runtime.yaml"),
		RPCAddress: "unix:///tmp/mcpv.sock",
		LogPath:    filepath.Join(tempDir, "mcpv.log"),
		Runner:     (&fakeRunner{}).Run,
	})
	require.NoError(t, err)

	status, err := manager.Install(context.Background())
	require.NoError(t, err)
	require.True(t, status.Installed)

	plistPath := filepath.Join(tempDir, "Library", "LaunchAgents", launchdLabel+".plist")
	plistBytes, err := os.ReadFile(plistPath)
	require.NoError(t, err)
	plist := string(plistBytes)
	require.Contains(t, plist, binPath)
	require.Contains(t, plist, "--config")
	require.Contains(t, plist, "MCPV_RPC_ADDRESS")
	require.Contains(t, plist, "StandardOutPath")
}

func TestManagerStatus_NotInstalled(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	manager, err := NewManager(Options{Runner: (&fakeRunner{}).Run})
	require.NoError(t, err)

	status, err := manager.Status(context.Background())
	require.NoError(t, err)
	require.False(t, status.Installed)
	require.False(t, status.Running)
}
