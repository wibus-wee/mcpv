//go:build darwin

package daemon

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

const launchdLabel = "com.mcpv.core"

func platformServiceName() string {
	return launchdLabel
}

func platformInstall(_ context.Context, m *Manager, binaryPath string, configPath string) error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}
	payload := renderLaunchdPlist(launchdLabel, binaryPath, configPath, m.rpcAddress, m.logPath)
	return os.WriteFile(plistPath, []byte(payload), 0o644)
}

func platformUninstall(ctx context.Context, m *Manager) error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_ = runLaunchctl(ctx, m.runner, "bootout", launchdDomain(), launchdLabel)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func platformStart(ctx context.Context, m *Manager, _ string, _ string) error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotInstalled
		}
		return err
	}
	loaded, err := launchdLoaded(ctx, m.runner)
	if err != nil {
		return err
	}
	if !loaded {
		if err := runLaunchctl(ctx, m.runner, "bootstrap", launchdDomain(), plistPath); err != nil {
			return err
		}
	}
	return runLaunchctl(ctx, m.runner, "kickstart", "-k", fmt.Sprintf("%s/%s", launchdDomain(), launchdLabel))
}

func platformStop(ctx context.Context, m *Manager) error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotInstalled
		}
		return err
	}
	if err := runLaunchctl(ctx, m.runner, "stop", fmt.Sprintf("%s/%s", launchdDomain(), launchdLabel)); err != nil {
		if errors.Is(err, ErrNotRunning) {
			return nil
		}
		return err
	}
	return nil
}

func platformStatus(ctx context.Context, m *Manager) (bool, bool, error) {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return false, false, err
	}
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	output, exitCode, err := m.runner(ctx, "launchctl", "print", fmt.Sprintf("%s/%s", launchdDomain(), launchdLabel))
	if err != nil {
		return true, false, nil
	}
	if exitCode != 0 {
		return true, false, nil
	}
	if strings.Contains(output, "state = running") || strings.Contains(output, "pid =") {
		return true, true, nil
	}
	return true, false, nil
}

func launchdPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func launchdDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func launchdLoaded(ctx context.Context, runner CommandRunner) (bool, error) {
	output, exitCode, err := runner(ctx, "launchctl", "print", fmt.Sprintf("%s/%s", launchdDomain(), launchdLabel))
	if err != nil {
		return false, nil
	}
	if exitCode != 0 {
		return false, nil
	}
	if strings.Contains(output, "service =") || strings.Contains(output, "state =") {
		return true, nil
	}
	return false, nil
}

func renderLaunchdPlist(label, binaryPath, configPath, rpcAddress, logPath string) string {
	builder := strings.Builder{}
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	builder.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	builder.WriteString(`<plist version="1.0">` + "\n")
	builder.WriteString("<dict>\n")
	builder.WriteString("  <key>Label</key>\n")
	builder.WriteString("  <string>" + xmlEscape(label) + "</string>\n")
	builder.WriteString("  <key>ProgramArguments</key>\n")
	builder.WriteString("  <array>\n")
	builder.WriteString("    <string>" + xmlEscape(binaryPath) + "</string>\n")
	builder.WriteString("    <string>serve</string>\n")
	builder.WriteString("    <string>--config</string>\n")
	builder.WriteString("    <string>" + xmlEscape(configPath) + "</string>\n")
	builder.WriteString("  </array>\n")
	builder.WriteString("  <key>RunAtLoad</key>\n")
	builder.WriteString("  <true/>\n")
	builder.WriteString("  <key>KeepAlive</key>\n")
	builder.WriteString("  <false/>\n")
	builder.WriteString("  <key>EnvironmentVariables</key>\n")
	builder.WriteString("  <dict>\n")
	builder.WriteString("    <key>MCPV_CONFIG_PATH</key>\n")
	builder.WriteString("    <string>" + xmlEscape(configPath) + "</string>\n")
	if strings.TrimSpace(rpcAddress) != "" {
		builder.WriteString("    <key>MCPV_RPC_ADDRESS</key>\n")
		builder.WriteString("    <string>" + xmlEscape(rpcAddress) + "</string>\n")
	}
	if strings.TrimSpace(logPath) != "" {
		builder.WriteString("    <key>MCPV_LOG_PATH</key>\n")
		builder.WriteString("    <string>" + xmlEscape(logPath) + "</string>\n")
	}
	builder.WriteString("  </dict>\n")
	if strings.TrimSpace(logPath) != "" {
		builder.WriteString("  <key>StandardOutPath</key>\n")
		builder.WriteString("  <string>" + xmlEscape(logPath) + "</string>\n")
		builder.WriteString("  <key>StandardErrorPath</key>\n")
		builder.WriteString("  <string>" + xmlEscape(logPath) + "</string>\n")
	}
	builder.WriteString("</dict>\n")
	builder.WriteString("</plist>\n")
	return builder.String()
}

func runLaunchctl(ctx context.Context, runner CommandRunner, args ...string) error {
	output, exitCode, err := runner(ctx, "launchctl", args...)
	if err != nil {
		if strings.Contains(output, "No such process") {
			return ErrNotRunning
		}
		return formatCommandError("launchctl", args, output, err, exitCode)
	}
	return nil
}

func xmlEscape(value string) string {
	return html.EscapeString(value)
}

var ErrNotRunning = errors.New("service not running")
