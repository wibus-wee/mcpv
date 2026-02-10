package uiconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

const defaultSettingsFileName = "ui-settings.db"

// ResolveDefaultPath returns the default path for UI settings storage.
func ResolveDefaultPath() string {
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
	return filepath.Join(base, "mcpv", defaultSettingsFileName)
}

// WorkspaceIDForPath derives a stable workspace identifier from a config path.
func WorkspaceIDForPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	clean := trimmed
	if abs, err := filepath.Abs(trimmed); err == nil {
		clean = abs
	}
	sum := sha256.Sum256([]byte(clean))
	return hex.EncodeToString(sum[:])
}
