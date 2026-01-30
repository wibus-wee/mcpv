package ui

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

	"mcpd/internal/app"
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
	return app.Version
}

// Ping responds with pong.
func (s *SystemService) Ping(_ context.Context) string {
	s.logger.Debug("ping received")
	return "pong"
}

// ResolveMcpdmcpPath resolves the mcpdmcp executable path.
func (s *SystemService) ResolveMcpdmcpPath() string {
	name := "mcpdmcp"
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
