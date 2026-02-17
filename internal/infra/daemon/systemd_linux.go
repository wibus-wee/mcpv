//go:build linux

package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const systemdServiceName = "mcpv.service"

func platformServiceName() string {
	return systemdServiceName
}

func platformInstall(ctx context.Context, m *Manager, binaryPath string, configPath string) error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}
	unit := renderSystemdUnit(systemdServiceName, binaryPath, configPath, m.rpcAddress, m.logPath)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return err
	}
	if err := runCommand(ctx, m.runner, "systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	return runCommand(ctx, m.runner, "systemctl", "--user", "enable", systemdServiceName)
}

func platformUninstall(ctx context.Context, m *Manager) error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(unitPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_ = runCommand(ctx, m.runner, "systemctl", "--user", "disable", "--now", systemdServiceName)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return runCommand(ctx, m.runner, "systemctl", "--user", "daemon-reload")
}

func platformStart(ctx context.Context, m *Manager, binaryPath string, configPath string) error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(unitPath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotInstalled
		}
		return err
	}
	return runCommand(ctx, m.runner, "systemctl", "--user", "start", systemdServiceName)
}

func platformStop(ctx context.Context, m *Manager) error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(unitPath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotInstalled
		}
		return err
	}
	return runCommand(ctx, m.runner, "systemctl", "--user", "stop", systemdServiceName)
}

func platformStatus(ctx context.Context, m *Manager) (bool, bool, error) {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return false, false, err
	}
	if _, err := os.Stat(unitPath); err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	output, exitCode, err := m.runner(ctx, "systemctl", "--user", "is-active", systemdServiceName)
	trimmed := strings.TrimSpace(output)
	if err == nil && trimmed == "active" {
		return true, true, nil
	}
	if exitCode == 3 || trimmed == "inactive" || trimmed == "failed" {
		return true, false, nil
	}
	if exitCode == 4 || trimmed == "unknown" {
		return false, false, nil
	}
	if err != nil {
		return true, false, formatCommandError("systemctl", []string{"--user", "is-active", systemdServiceName}, output, err, exitCode)
	}
	return true, false, nil
}

func systemdUnitPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "systemd", "user", systemdServiceName), nil
}

func renderSystemdUnit(serviceName, binaryPath, configPath, rpcAddress, logPath string) string {
	var builder strings.Builder
	builder.WriteString("[Unit]\n")
	builder.WriteString("Description=mcpv core\n")
	builder.WriteString("After=network.target\n\n")

	builder.WriteString("[Service]\n")
	builder.WriteString("Type=simple\n")
	builder.WriteString("ExecStart=")
	builder.WriteString(escapeSystemdArg(binaryPath))
	builder.WriteString(" serve --config ")
	builder.WriteString(escapeSystemdArg(configPath))
	builder.WriteString("\n")
	if strings.TrimSpace(rpcAddress) != "" {
		builder.WriteString("Environment=")
		builder.WriteString(escapeSystemdEnv("MCPV_RPC_ADDRESS", rpcAddress))
		builder.WriteString("\n")
	}
	builder.WriteString("Environment=")
	builder.WriteString(escapeSystemdEnv("MCPV_CONFIG_PATH", configPath))
	builder.WriteString("\n")
	if strings.TrimSpace(logPath) != "" {
		builder.WriteString("Environment=")
		builder.WriteString(escapeSystemdEnv("MCPV_LOG_PATH", logPath))
		builder.WriteString("\n")
		builder.WriteString("StandardOutput=append:")
		builder.WriteString(logPath)
		builder.WriteString("\n")
		builder.WriteString("StandardError=append:")
		builder.WriteString(logPath)
		builder.WriteString("\n")
	}
	builder.WriteString("Restart=on-failure\n")
	builder.WriteString("RestartSec=2\n\n")

	builder.WriteString("[Install]\n")
	builder.WriteString("WantedBy=default.target\n")
	return builder.String()
}

func escapeSystemdArg(value string) string {
	if value == "" {
		return "\"\""
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '"' || r == '\\'
	}) >= 0 {
		return strconv.Quote(value)
	}
	return value
}

func escapeSystemdEnv(key, value string) string {
	combined := key + "=" + value
	return strconv.Quote(combined)
}

func runCommand(ctx context.Context, runner CommandRunner, name string, args ...string) error {
	output, exitCode, err := runner(ctx, name, args...)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return formatCommandError(name, args, output, err, exitCode)
	}
	return nil
}
