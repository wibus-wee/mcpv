package services

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/buildinfo"
)

// SystemService exposes system-level utility APIs.
type SystemService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewSystemService(deps *ServiceDeps) *SystemService {
	return &SystemService{
		deps:   deps,
		logger: deps.loggerNamed("system-service"),
	}
}

// HandleURLScheme handles URL Scheme invocations.
func (s *SystemService) HandleURLScheme(rawURL string) error {
	s.logger.Info("received URL scheme", zap.String("url", rawURL))

	parsed, err := url.Parse(rawURL)
	if err != nil {
		s.logger.Error("failed to parse URL", zap.Error(err))
		return fmt.Errorf("invalid URL: %w", err)
	}

	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) > 0 {
		action := parts[0]
		s.emitNavigationEvent(action, parsed.Query())
	}

	return nil
}

func (s *SystemService) emitNavigationEvent(action string, query url.Values) {
	wails := s.deps.wailsApp()
	if wails == nil {
		s.logger.Warn("wails app not set, cannot emit event")
		return
	}

	eventData := map[string]interface{}{
		"action": action,
		"params": query,
	}

	wails.Event.Emit("navigate", eventData)
	s.logger.Debug("emitted navigation event", zap.String("action", action))
}

// GetVersion returns app version.
func (s *SystemService) GetVersion() string {
	return buildinfo.Version
}

// Ping responds with pong.
func (s *SystemService) Ping(_ context.Context) string {
	s.logger.Debug("ping received")
	return "pong"
}

// GetUpdateCheckOptions returns current update checker options.
func (s *SystemService) GetUpdateCheckOptions() (UpdateCheckOptions, error) {
	checker, err := s.deps.updateChecker()
	if err != nil {
		return UpdateCheckOptions{}, err
	}
	return checker.Options(), nil
}

// SetUpdateCheckOptions updates update checker options.
func (s *SystemService) SetUpdateCheckOptions(opts UpdateCheckOptions) (UpdateCheckOptions, error) {
	checker, err := s.deps.updateChecker()
	if err != nil {
		return UpdateCheckOptions{}, err
	}
	return checker.SetOptions(opts), nil
}

// CheckForUpdates triggers an immediate update check.
func (s *SystemService) CheckForUpdates(ctx context.Context) (UpdateCheckResult, error) {
	checker, err := s.deps.updateChecker()
	if err != nil {
		return UpdateCheckResult{}, err
	}
	return checker.CheckNow(ctx)
}

// ResolvemcpvmcpPath resolves the mcpvmcp executable path.
func (s *SystemService) ResolvemcpvmcpPath() string {
	name := "mcpvmcp"
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
