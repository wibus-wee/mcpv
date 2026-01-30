package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"mcpv/internal/domain"
	"mcpv/internal/infra/fsutil"
)

func TestResolveProfilePath(t *testing.T) {
	root := t.TempDir()
	profilesDir := filepath.Join(root, profilesDirName)
	require.NoError(t, os.MkdirAll(profilesDir, fsutil.DefaultDirMode))

	defaultPath := filepath.Join(profilesDir, "default.yaml")
	require.NoError(t, os.WriteFile(defaultPath, []byte("servers: []\n"), fsutil.DefaultFileMode))

	got, err := ResolveProfilePath(root, "default")
	require.NoError(t, err)
	require.Equal(t, defaultPath, got)

	otherPath := filepath.Join(profilesDir, "other.yml")
	require.NoError(t, os.WriteFile(otherPath, []byte("servers: []\n"), fsutil.DefaultFileMode))

	got, err = ResolveProfilePath(root, "other")
	require.NoError(t, err)
	require.Equal(t, otherPath, got)
}

func TestResolveProfilePath_DuplicateExtensions(t *testing.T) {
	root := t.TempDir()
	profilesDir := filepath.Join(root, profilesDirName)
	require.NoError(t, os.MkdirAll(profilesDir, fsutil.DefaultDirMode))

	require.NoError(t, os.WriteFile(filepath.Join(profilesDir, "dup.yaml"), []byte("servers: []\n"), fsutil.DefaultFileMode))
	require.NoError(t, os.WriteFile(filepath.Join(profilesDir, "dup.yml"), []byte("servers: []\n"), fsutil.DefaultFileMode))

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
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), fsutil.DefaultFileMode))

	update, err := BuildProfileUpdate(profilePath, []domain.ServerSpec{
		{
			Name:                "imported",
			Cmd:                 []string{"./b"},
			IdleSeconds:         60,
			MaxConcurrent:       1,
			Strategy:            domain.StrategyStateless,
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
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), fsutil.DefaultFileMode))

	_, err := BuildProfileUpdate(profilePath, []domain.ServerSpec{
		{
			Name:                "existing",
			Cmd:                 []string{"./b"},
			IdleSeconds:         60,
			MaxConcurrent:       1,
			Strategy:            domain.StrategyStateless,
			MinReady:            0,
			DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
			ProtocolVersion:     domain.DefaultProtocolVersion,
		},
	})
	require.Error(t, err)
}

func TestCreateServer_Appends(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "default.yaml")
	content := `
servers:
  - name: existing
    cmd: ["./a"]
    idleSeconds: 60
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), fsutil.DefaultFileMode))

	update, err := CreateServer(profilePath, domain.ServerSpec{
		Name:                "created",
		Cmd:                 []string{"./b"},
		IdleSeconds:         60,
		MaxConcurrent:       1,
		Strategy:            domain.StrategyStateless,
		MinReady:            0,
		DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
		ProtocolVersion:     domain.DefaultProtocolVersion,
	})
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(update.Data, &doc))

	rawServers, ok := doc["servers"]
	require.True(t, ok)
	encoded, err := yaml.Marshal(rawServers)
	require.NoError(t, err)

	var servers []serverSpecYAML
	require.NoError(t, yaml.Unmarshal(encoded, &servers))
	require.Len(t, servers, 2)
	require.Equal(t, "existing", servers[0].Name)
	require.Equal(t, "created", servers[1].Name)
}

func TestCreateServer_RejectsDuplicate(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "default.yaml")
	content := `
servers:
  - name: existing
    cmd: ["./a"]
    idleSeconds: 60
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), fsutil.DefaultFileMode))

	_, err := CreateServer(profilePath, domain.ServerSpec{
		Name:                "existing",
		Cmd:                 []string{"./b"},
		IdleSeconds:         60,
		MaxConcurrent:       1,
		Strategy:            domain.StrategyStateless,
		MinReady:            0,
		DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
		ProtocolVersion:     domain.DefaultProtocolVersion,
	})
	require.ErrorIs(t, err, ErrServerExists)
}

func TestUpdateServer_Replaces(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "default.yaml")
	content := `
servers:
  - name: existing
    cmd: ["./a"]
    idleSeconds: 60
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), fsutil.DefaultFileMode))

	update, err := UpdateServer(profilePath, domain.ServerSpec{
		Name:                "existing",
		Cmd:                 []string{"./updated"},
		IdleSeconds:         120,
		MaxConcurrent:       2,
		Strategy:            domain.StrategyStateless,
		MinReady:            0,
		DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
		ProtocolVersion:     domain.DefaultProtocolVersion,
	})
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(update.Data, &doc))

	rawServers, ok := doc["servers"]
	require.True(t, ok)
	encoded, err := yaml.Marshal(rawServers)
	require.NoError(t, err)

	var servers []serverSpecYAML
	require.NoError(t, yaml.Unmarshal(encoded, &servers))
	require.Len(t, servers, 1)
	require.Equal(t, "existing", servers[0].Name)
	require.Equal(t, []string{"./updated"}, servers[0].Cmd)
	require.Equal(t, 120, servers[0].IdleSeconds)
	require.Equal(t, 2, servers[0].MaxConcurrent)
}

func TestUpdateServer_Missing(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "default.yaml")
	content := `
servers: []
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), fsutil.DefaultFileMode))

	_, err := UpdateServer(profilePath, domain.ServerSpec{
		Name:                "missing",
		Cmd:                 []string{"./b"},
		IdleSeconds:         60,
		MaxConcurrent:       1,
		Strategy:            domain.StrategyStateless,
		MinReady:            0,
		DrainTimeoutSeconds: domain.DefaultDrainTimeoutSeconds,
		ProtocolVersion:     domain.DefaultProtocolVersion,
	})
	require.ErrorIs(t, err, ErrServerNotFound)
}

func TestSetServerDisabled(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "default.yaml")
	content := `
servers:
  - name: existing
    cmd: ["./a"]
    idleSeconds: 60
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), fsutil.DefaultFileMode))

	update, err := SetServerDisabled(profilePath, "existing", true)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(update.Data, &doc))

	rawServers, ok := doc["servers"]
	require.True(t, ok)
	encoded, err := yaml.Marshal(rawServers)
	require.NoError(t, err)

	var servers []serverSpecYAML
	require.NoError(t, yaml.Unmarshal(encoded, &servers))
	require.Len(t, servers, 1)
	require.True(t, servers[0].Disabled)
}

func TestDeleteServer(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "default.yaml")
	content := `
servers:
  - name: first
    cmd: ["./a"]
    idleSeconds: 60
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
  - name: second
    cmd: ["./b"]
    idleSeconds: 60
    maxConcurrent: 1
    strategy: stateless
    minReady: 0
    protocolVersion: "2025-11-25"
`
	require.NoError(t, os.WriteFile(profilePath, []byte(content), fsutil.DefaultFileMode))

	update, err := DeleteServer(profilePath, "first")
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(update.Data, &doc))

	rawServers, ok := doc["servers"]
	require.True(t, ok)
	encoded, err := yaml.Marshal(rawServers)
	require.NoError(t, err)

	var servers []serverSpecYAML
	require.NoError(t, yaml.Unmarshal(encoded, &servers))
	require.Len(t, servers, 1)
	require.Equal(t, "second", servers[0].Name)
}
