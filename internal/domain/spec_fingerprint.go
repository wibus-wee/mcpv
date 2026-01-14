package domain

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"sort"
)

func SpecFingerprint(spec ServerSpec) (string, error) {
	hasher := sha256.New()
	transport := NormalizeTransport(spec.Transport)
	writeString(hasher, string(transport))
	writeStringSlice(hasher, spec.Cmd)
	writeEnvMap(hasher, spec.Env)
	writeString(hasher, spec.Cwd)
	writeString(hasher, spec.ProtocolVersion)
	if transport == TransportStreamableHTTP {
		writeStreamableHTTPConfig(hasher, spec.HTTP)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeStreamableHTTPConfig(h hash.Hash, cfg *StreamableHTTPConfig) {
	if cfg == nil {
		writeInt(h, 0)
		return
	}
	writeInt(h, 1)
	writeString(h, cfg.Endpoint)
	writeInt(h, cfg.MaxRetries)
	writeEnvMap(h, cfg.Headers)
}

func writeStringSlice(h hash.Hash, values []string) {
	if values == nil {
		writeInt(h, 0)
		return
	}
	writeInt(h, len(values))
	for _, value := range values {
		writeString(h, value)
	}
}

func writeEnvMap(h hash.Hash, env map[string]string) {
	if len(env) == 0 {
		writeInt(h, 0)
		return
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writeInt(h, len(keys))
	for _, key := range keys {
		writeString(h, key)
		writeString(h, env[key])
	}
}

func writeString(h hash.Hash, value string) {
	writeInt(h, len(value))
	_, _ = h.Write([]byte(value))
}

func writeInt(h hash.Hash, value int) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(value))
	_, _ = h.Write(buf)
}
