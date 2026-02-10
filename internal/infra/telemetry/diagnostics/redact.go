package diagnostics

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

var sensitiveKeys = []string{
	"token",
	"secret",
	"authorization",
	"api_key",
	"apikey",
	"cookie",
}

// RedactValue masks the value if the key is sensitive.
func RedactValue(key, value string) string {
	lower := strings.ToLower(key)
	for _, needle := range sensitiveKeys {
		if strings.Contains(lower, needle) {
			return "***"
		}
	}
	return value
}

// RedactMap redacts values for sensitive keys.
func RedactMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = RedactValue(key, value)
	}
	return out
}

// EncodeStringMap serializes the map into a JSON string.
func EncodeStringMap(input map[string]string) string {
	if len(input) == 0 {
		return ""
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	return string(raw)
}

// ContainsSensitiveKey reports whether the key should be redacted.
func ContainsSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, needle := range sensitiveKeys {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

// HashBytes computes a sha256 hash for the input and returns hex encoding.
func HashBytes(input []byte) string {
	if len(input) == 0 {
		return ""
	}
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}

// TruncateString truncates the value to limit bytes and appends a suffix when needed.
func TruncateString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
