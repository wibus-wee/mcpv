package rpc

import (
	"fmt"
	"os"
	"strings"

	"mcpv/internal/domain"
)

func resolveSocketMode(value string) (os.FileMode, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := domain.ParseSocketMode(value)
	if err != nil {
		return 0, fmt.Errorf("invalid rpc socketMode %q: %w", value, err)
	}
	return os.FileMode(parsed), nil
}
