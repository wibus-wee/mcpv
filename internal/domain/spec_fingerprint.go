package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type specFingerprintInput struct {
	Cmd             []string   `json:"cmd"`
	Env             []envEntry `json:"env"`
	Cwd             string     `json:"cwd"`
	IdleSeconds     int        `json:"idleSeconds"`
	MaxConcurrent   int        `json:"maxConcurrent"`
	Sticky          bool       `json:"sticky"`
	Persistent      bool       `json:"persistent"`
	MinReady        int        `json:"minReady"`
	DrainTimeout    int        `json:"drainTimeoutSeconds"`
	ProtocolVersion string     `json:"protocolVersion"`
	ExposeTools     []string   `json:"exposeTools"`
}

type envEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func SpecFingerprint(spec ServerSpec) (string, error) {
	env := fingerprintEnv(spec.Env)
	exposeTools := append([]string(nil), spec.ExposeTools...)
	sort.Strings(exposeTools)

	if len(env) == 0 {
		env = []envEntry{}
	}
	if len(exposeTools) == 0 {
		exposeTools = []string{}
	}

	input := specFingerprintInput{
		Cmd:             append([]string(nil), spec.Cmd...),
		Env:             env,
		Cwd:             spec.Cwd,
		IdleSeconds:     spec.IdleSeconds,
		MaxConcurrent:   spec.MaxConcurrent,
		Sticky:          spec.Sticky,
		Persistent:      spec.Persistent,
		MinReady:        spec.MinReady,
		DrainTimeout:    spec.DrainTimeoutSeconds,
		ProtocolVersion: spec.ProtocolVersion,
		ExposeTools:     exposeTools,
	}

	raw, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal spec fingerprint: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func fingerprintEnv(env map[string]string) []envEntry {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]envEntry, 0, len(keys))
	for _, key := range keys {
		out = append(out, envEntry{Key: key, Value: env[key]})
	}
	return out
}
