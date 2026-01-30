package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"mcpv/internal/infra/fsutil"
)

const defaultConfigFileName = "runtime.yaml"

// ResolveDefaultConfigPath returns the default config path for the UI runtime.
func ResolveDefaultConfigPath() string {
	return defaultConfigPath()
}

// EnsureConfigFile makes sure the config file exists on disk.
func EnsureConfigFile(path string) error {
	return ensureConfigFile(path)
}

func defaultConfigPath() string {
	base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			base = filepath.Join(home, ".config")
		}
	}
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "mcpv", defaultConfigFileName)
}

func ensureConfigFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("config path is required")
	}
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return errors.New("config path must be a file")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), fsutil.DefaultDirMode); err != nil {
		return err
	}
	payload := []byte("servers: []\n")
	return os.WriteFile(path, payload, fsutil.DefaultFileMode)
}
