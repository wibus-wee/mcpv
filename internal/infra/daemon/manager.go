package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mcpv/internal/domain"
)

var ErrNotInstalled = errors.New("service not installed")
var ErrUnsupported = errors.New("service manager unsupported")

type Status struct {
	Installed   bool
	Running     bool
	ServiceName string
	ConfigPath  string
	RPCAddress  string
	LogPath     string
}

type Options struct {
	BinaryPath string
	ConfigPath string
	RPCAddress string
	LogPath    string
	Runner     CommandRunner
}

type CommandRunner func(ctx context.Context, name string, args ...string) (string, int, error)

type Manager struct {
	binaryPath string
	configPath string
	rpcAddress string
	logPath    string
	runner     CommandRunner
}

func NewManager(opts Options) (*Manager, error) {
	configPath, err := normalizePath(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	logPath, err := normalizePath(opts.LogPath)
	if err != nil {
		return nil, err
	}
	runner := opts.Runner
	if runner == nil {
		runner = execCommand
	}
	return &Manager{
		binaryPath: strings.TrimSpace(opts.BinaryPath),
		configPath: configPath,
		rpcAddress: strings.TrimSpace(opts.RPCAddress),
		logPath:    logPath,
		runner:     runner,
	}, nil
}

func (m *Manager) Install(ctx context.Context) (Status, error) {
	configPath, err := ensureConfigPath(m.configPath)
	if err != nil {
		return Status{}, err
	}
	binaryPath, err := resolveBinaryPath(m.binaryPath)
	if err != nil {
		return Status{}, err
	}
	if err := ensureLogDir(m.logPath); err != nil {
		return Status{}, err
	}
	if err := platformInstall(ctx, m, binaryPath, configPath); err != nil {
		return Status{}, err
	}
	installed, running, err := platformStatus(ctx, m)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Installed:   installed,
		Running:     running,
		ServiceName: platformServiceName(),
		ConfigPath:  configPath,
		RPCAddress:  m.rpcAddress,
		LogPath:     m.logPath,
	}, nil
}

func (m *Manager) Uninstall(ctx context.Context) (Status, error) {
	if err := platformUninstall(ctx, m); err != nil {
		return Status{}, err
	}
	installed, running, err := platformStatus(ctx, m)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Installed:   installed,
		Running:     running,
		ServiceName: platformServiceName(),
		ConfigPath:  m.configPath,
		RPCAddress:  m.rpcAddress,
		LogPath:     m.logPath,
	}, nil
}

func (m *Manager) Start(ctx context.Context) (Status, error) {
	configPath, err := ensureConfigPath(m.configPath)
	if err != nil {
		return Status{}, err
	}
	binaryPath, err := resolveBinaryPath(m.binaryPath)
	if err != nil {
		return Status{}, err
	}
	if err := ensureLogDir(m.logPath); err != nil {
		return Status{}, err
	}
	if err := platformStart(ctx, m, binaryPath, configPath); err != nil {
		return Status{}, err
	}
	installed, running, err := platformStatus(ctx, m)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Installed:   installed,
		Running:     running,
		ServiceName: platformServiceName(),
		ConfigPath:  configPath,
		RPCAddress:  m.rpcAddress,
		LogPath:     m.logPath,
	}, nil
}

func (m *Manager) Stop(ctx context.Context) (Status, error) {
	if err := platformStop(ctx, m); err != nil {
		return Status{}, err
	}
	installed, running, err := platformStatus(ctx, m)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Installed:   installed,
		Running:     running,
		ServiceName: platformServiceName(),
		ConfigPath:  m.configPath,
		RPCAddress:  m.rpcAddress,
		LogPath:     m.logPath,
	}, nil
}

func (m *Manager) Status(ctx context.Context) (Status, error) {
	installed, running, err := platformStatus(ctx, m)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Installed:   installed,
		Running:     running,
		ServiceName: platformServiceName(),
		ConfigPath:  m.configPath,
		RPCAddress:  m.rpcAddress,
		LogPath:     m.logPath,
	}, nil
}

func (m *Manager) EnsureRunning(ctx context.Context, allowStart bool) (Status, error) {
	status, err := m.Status(ctx)
	if err != nil {
		return Status{}, err
	}
	if status.Running {
		return status, nil
	}
	if !allowStart {
		return status, domain.ErrPermissionDenied
	}
	if !status.Installed {
		if _, err := m.Install(ctx); err != nil {
			return Status{}, err
		}
	}
	return m.Start(ctx)
}

func normalizePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}
	clean := filepath.Clean(trimmed)
	if filepath.IsAbs(clean) {
		return clean, nil
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func ensureConfigPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("config path is required")
	}
	return normalizePath(trimmed)
}

func ensureLogDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func resolveBinaryPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		trimmed = "mcpv"
	}
	if filepath.IsAbs(trimmed) {
		if info, err := os.Stat(trimmed); err == nil && !info.IsDir() {
			return trimmed, nil
		}
	}
	if info, err := os.Stat(trimmed); err == nil && !info.IsDir() {
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	if resolved, err := exec.LookPath(trimmed); err == nil {
		return resolved, nil
	}
	if execPath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), trimmed)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", domain.ErrExecutableNotFound
}

func execCommand(ctx context.Context, name string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return string(output), exitCode, err
}

func formatCommandError(name string, args []string, output string, err error, exitCode int) error {
	if output != "" {
		return fmt.Errorf("%s %s failed (exit=%d): %s", name, strings.Join(args, " "), exitCode, strings.TrimSpace(output))
	}
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return fmt.Errorf("%s %s failed", name, strings.Join(args, " "))
}
