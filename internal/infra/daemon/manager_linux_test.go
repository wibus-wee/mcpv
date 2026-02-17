//go:build linux

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
	return "inactive", 3, nil
}

func TestManagerInstall_WritesUnitAndRunsCommands(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	runner := &fakeRunner{}
	manager, err := NewManager(Options{
		BinaryPath: "/usr/local/bin/mcpv",
		ConfigPath: "/tmp/runtime.yaml",
		RPCAddress: "unix:///tmp/mcpv.sock",
		LogPath:    "/tmp/mcpv.log",
		Runner:     runner.Run,
	})
	require.NoError(t, err)

	status, err := manager.Install(context.Background())
	require.NoError(t, err)
	require.True(t, status.Installed)

	unitPath := filepath.Join(tempDir, "systemd", "user", systemdServiceName)
	unitBytes, err := os.ReadFile(unitPath)
	require.NoError(t, err)
	unit := string(unitBytes)
	require.Contains(t, unit, "ExecStart=/usr/local/bin/mcpv serve --config /tmp/runtime.yaml")
	require.Contains(t, unit, "MCPV_RPC_ADDRESS")
	require.Contains(t, unit, "/tmp/mcpv.log")

	require.GreaterOrEqual(t, len(runner.calls), 2)
	require.Contains(t, runner.calls[0], "systemctl --user daemon-reload")
	require.Contains(t, runner.calls[1], "systemctl --user enable "+systemdServiceName)
}

func TestManagerStatus_NotInstalled(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	manager, err := NewManager(Options{Runner: (&fakeRunner{}).Run})
	require.NoError(t, err)

	status, err := manager.Status(context.Background())
	require.NoError(t, err)
	require.False(t, status.Installed)
	require.False(t, status.Running)
}
