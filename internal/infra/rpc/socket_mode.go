package rpc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func resolveSocketMode(value string) (os.FileMode, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(value, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid rpc socketMode %q: expected 0660 or 0o660", value)
	}
	if parsed > 0o777 {
		return 0, fmt.Errorf("rpc socketMode must be <= 0777")
	}
	return os.FileMode(parsed), nil
}
