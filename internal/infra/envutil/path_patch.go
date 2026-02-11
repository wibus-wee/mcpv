package envutil

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	skipPathPatchEnv = "MCPV_SKIP_PATH_PATCH"
	termEnv          = "TERM"
	shellEnv         = "SHELL"
	pathEnv          = "PATH"
)

type pathCacheEntry struct {
	path string
	err  error
}

var loginPathCache sync.Map

// PatchPATHIfNeeded updates PATH for GUI-launched processes on macOS.
func PatchPATHIfNeeded(env []string) []string {
	if runtime.GOOS != "darwin" {
		return env
	}
	if strings.TrimSpace(envVarValue(env, skipPathPatchEnv)) != "" {
		return env
	}
	if strings.TrimSpace(envVarValue(env, termEnv)) != "" {
		return env
	}
	shellPath := strings.TrimSpace(envVarValue(env, shellEnv))
	if shellPath == "" {
		shellPath = "/bin/zsh"
	}
	loginPath, err := loginShellPATH(shellPath)
	if err != nil || strings.TrimSpace(loginPath) == "" {
		return env
	}
	currentPath := envVarValue(env, pathEnv)
	mergedPath := mergePATH(loginPath, currentPath)
	if mergedPath == "" || mergedPath == currentPath {
		return env
	}
	return setEnvValue(env, pathEnv, mergedPath)
}

func envVarValue(env []string, key string) string {
	if key == "" {
		return ""
	}
	prefix := key + "="
	var value string
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			value = strings.TrimPrefix(entry, prefix)
		}
	}
	return value
}

func setEnvValue(env []string, key, value string) []string {
	if key == "" {
		return env
	}
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	out = append(out, prefix+value)
	return out
}

func loginShellPATH(shellPath string) (string, error) {
	if cached, ok := loginPathCache.Load(shellPath); ok {
		entry := cached.(pathCacheEntry)
		return entry.path, entry.err
	}
	path, err := resolveLoginShellPATH(shellPath)
	loginPathCache.Store(shellPath, pathCacheEntry{path: path, err: err})
	return path, err
}

func resolveLoginShellPATH(shellPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, shellPath, "-lc", "echo $PATH")
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func mergePATH(primary, fallback string) string {
	separator := string(os.PathListSeparator)
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)

	appendPath := func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		for _, entry := range strings.Split(path, separator) {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if _, exists := seen[entry]; exists {
				continue
			}
			seen[entry] = struct{}{}
			out = append(out, entry)
		}
	}

	appendPath(primary)
	appendPath(fallback)

	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, separator)
}
