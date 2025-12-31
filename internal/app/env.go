package app

import (
	"os"
	"strings"
)

func envBool(key string) bool {
	val := strings.TrimSpace(os.Getenv(key))
	return val == "1" || strings.EqualFold(val, "true")
}
