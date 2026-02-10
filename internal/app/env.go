package app

import (
	"os"
	"strings"

	"mcpv/internal/buildinfo"
)

func envBool(key string) bool {
	val := strings.TrimSpace(os.Getenv(key))
	return val == "1" || strings.EqualFold(val, "true")
}

func envBoolOptional(key string) (bool, bool) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return false, false
	}
	if val == "1" || strings.EqualFold(val, "true") {
		return true, true
	}
	if val == "0" || strings.EqualFold(val, "false") {
		return false, true
	}
	return false, true
}

func diagnosticsCaptureSensitive() bool {
	if value, ok := envBoolOptional("MCPV_DIAGNOSTICS_CAPTURE_SENSITIVE"); ok {
		return value
	}
	return buildinfo.Version == "dev"
}
