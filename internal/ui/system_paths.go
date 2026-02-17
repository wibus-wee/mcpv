package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func ResolveMcpvmcpPath() string {
	return resolveBinaryPath("mcpvmcp")
}

func ResolveMcpvPath() string {
	return resolveBinaryPath("mcpv")
}

func ResolveMcpvctlPath() string {
	return resolveBinaryPath("mcpvctl")
}

func resolveBinaryPath(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	if execPath, err := os.Executable(); err == nil {
		dir := filepath.Dir(execPath)
		candidate := filepath.Join(dir, name)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
	}

	return name
}
