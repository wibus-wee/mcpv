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

func TestCreateProfile(t *testing.T) {
	root := t.TempDir()
	profilesDir := filepath.Join(root, profilesDirName)
	require.NoError(t, os.MkdirAll(profilesDir, fsutil.DefaultDirMode))

	path, err := CreateProfile(root, "custom")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(profilesDir, "custom.yaml"), path)
}

func TestDeleteProfile(t *testing.T) {
	root := t.TempDir()
	profilesDir := filepath.Join(root, profilesDirName)
	require.NoError(t, os.MkdirAll(profilesDir, fsutil.DefaultDirMode))

	path := filepath.Join(profilesDir, "custom.yaml")
	require.NoError(t, os.WriteFile(path, []byte("servers: []\n"), fsutil.DefaultFileMode))

	require.NoError(t, DeleteProfile(root, "custom"))
	_, err := os.Stat(path)
	require.Error(t, err)
}

func TestSetCallerMapping(t *testing.T) {
	root := t.TempDir()
	profilesDir := filepath.Join(root, profilesDirName)
	require.NoError(t, os.MkdirAll(profilesDir, fsutil.DefaultDirMode))
	require.NoError(t, os.WriteFile(filepath.Join(profilesDir, "default.yaml"), []byte("servers: []\n"), fsutil.DefaultFileMode))

	update, err := SetCallerMapping(root, "cursor", "default", map[string]domain.Profile{
		domain.DefaultProfileName: {
			Name:    domain.DefaultProfileName,
			Catalog: domain.Catalog{},
		},
	})
	require.NoError(t, err)

	var doc rawCallers
	require.NoError(t, yaml.Unmarshal(update.Data, &doc))
	require.Equal(t, map[string]string{"cursor": "default"}, doc.Callers)
}

func TestRemoveCallerMapping(t *testing.T) {
	root := t.TempDir()
	profilesDir := filepath.Join(root, profilesDirName)
	require.NoError(t, os.MkdirAll(profilesDir, fsutil.DefaultDirMode))
	require.NoError(t, os.WriteFile(filepath.Join(profilesDir, "default.yaml"), []byte("servers: []\n"), fsutil.DefaultFileMode))

	callersPath := filepath.Join(root, callersFileName)
	require.NoError(t, os.WriteFile(callersPath, []byte("callers:\n  cursor: default\n"), fsutil.DefaultFileMode))

	update, err := RemoveCallerMapping(root, "cursor", map[string]domain.Profile{
		domain.DefaultProfileName: {
			Name:    domain.DefaultProfileName,
			Catalog: domain.Catalog{},
		},
	})
	require.NoError(t, err)

	var doc rawCallers
	require.NoError(t, yaml.Unmarshal(update.Data, &doc))
	require.Equal(t, map[string]string{}, doc.Callers)
}
