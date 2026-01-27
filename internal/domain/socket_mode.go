package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSocketMode parses an octal file mode string like 0660 or 0o660.
func ParseSocketMode(value string) (uint32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("rpc.socketMode must be an octal file mode like 0660 or 0o660")
	}
	parsed, err := strconv.ParseUint(value, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("rpc.socketMode must be an octal file mode like 0660 or 0o660")
	}
	if parsed > 0o777 {
		return 0, fmt.Errorf("rpc.socketMode must be <= 0777")
	}
	return uint32(parsed), nil
}
