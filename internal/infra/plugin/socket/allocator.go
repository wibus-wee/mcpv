package socket

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const socketFileName = "plugin.sock"

func Prepare(rootDir, name string) (string, string, error) {
	prefix := sanitizeName(name)
	if prefix == "" {
		prefix = "p"
	}
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	addrDir, err := os.MkdirTemp(rootDir, prefix+"-")
	if err != nil {
		return "", "", fmt.Errorf("create plugin socket dir: %w", err)
	}
	path := filepath.Join(addrDir, socketFileName)
	if err := os.RemoveAll(path); err != nil {
		return "", "", fmt.Errorf("cleanup plugin socket: %w", err)
	}
	return addrDir, path, nil
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, string(os.PathSeparator), "-")
	return name
}
