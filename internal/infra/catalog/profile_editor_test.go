package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"mcpd/internal/domain"
)

func TestResolveProfilePath(t *testing.T) {
	root := t.TempDir()
	profilesDir := filepath.Join(root, profilesDirName)
	require.NoError(t, os.MkdirAll(profilesDir, 0o755))

	defaultPath := filepath.Join(profilesDir, "default.yaml")
	require.NoError(t, os.WriteFile(defaultPath, []byte("servers: []\n"), 0o644))

	got, err := ResolveProfilePath(root, "default")
	require.NoError(t, err)
	require.Equal(t, defaultPath, got)

	otherPath := filepath.Join(profilesDir, "other.yml")
	require.NoError(t, os.WriteFile(otherPath, []byte("servers: []\n"), 0o644))

	got, err = ResolveProfilePath(root, "other")
	require.NoError(t, err)
	require.Equal(t, otherPath, got)
}

func TestResolveProfilePath_DuplicateExtensions(t *testing.T) {
	root := t.TempDir()
	profilesDir := filepath.Join(root, profilesDirName)
	require.NoError(t, os.MkdirAll(profilesDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(profilesDir, "dup.yaml"), []byte("servers: []\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(profilesDir, "dup.yml"), []byte("servers: []\n"), 0o644))

	_, err := ResolveProfilePath(root, "dup")
	require.Error(t, err)
}

func TestBuildProfileUpdate_AppendsServers(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "default.yaml")
	content := `
routeTimeoutSeconds: 10
servers:
  - name: existing
    cmd: ["./a"]
    idleSeconds: 60
    maxConcurrent: 1
    sticky: false
    persistent: false
    minReady: 0
    protocolVersion: "2025-11-25"
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), 0o644))

	update, err := BuildProfileUpdate(profilePath, []domain.ServerSpec{
		{
			Name:                "imported",
			Cmd:                 []string{"./b"},
			IdleSeconds:         60,
			MaxConcurrent:       1,
			Sticky:              false,
			Persistent:          false,
			MinReady:            0,
			DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
			ProtocolVersion:     domain.DefaultProtocolVersion,
		},
	})
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(update.Data, &doc))
	require.Equal(t, 10, doc["routeTimeoutSeconds"])

	rawServers, ok := doc["servers"]
	require.True(t, ok)
	encoded, err := yaml.Marshal(rawServers)
	require.NoError(t, err)

	var servers []serverSpecYAML
	require.NoError(t, yaml.Unmarshal(encoded, &servers))
	require.Len(t, servers, 2)
	require.Equal(t, "existing", servers[0].Name)
	require.Equal(t, "imported", servers[1].Name)
}

func TestBuildProfileUpdate_RejectsDuplicate(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "default.yaml")
	content := `
servers:
  - name: existing
    cmd: ["./a"]
    idleSeconds: 60
    maxConcurrent: 1
    sticky: false
    persistent: false
    minReady: 0
    protocolVersion: "2025-11-25"
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), 0o644))

	_, err := BuildProfileUpdate(profilePath, []domain.ServerSpec{
		{
			Name:                "existing",
			Cmd:                 []string{"./b"},
			IdleSeconds:         60,
			MaxConcurrent:       1,
			Sticky:              false,
			Persistent:          false,
			MinReady:            0,
			DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
			ProtocolVersion:     domain.DefaultProtocolVersion,
		},
	})
	require.Error(t, err)
}
