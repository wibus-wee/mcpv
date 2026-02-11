package services

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"mcpv/internal/buildinfo"
	"mcpv/internal/ui"
)

// SystemService exposes system-level utility APIs.
type SystemService struct {
	deps       *ServiceDeps
	logger     *zap.Logger
	downloader *ui.UpdateDownloader
	installer  *ui.UpdateInstaller
}

func NewSystemService(deps *ServiceDeps) *SystemService {
	logger := deps.loggerNamed("system-service")
	return &SystemService{
		deps:       deps,
		logger:     logger,
		downloader: ui.NewUpdateDownloader(logger),
		installer: ui.NewUpdateInstaller(logger, func() {
			manager := deps.manager()
			if manager != nil {
				manager.Shutdown()
			}
		}),
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

// StartUpdateDownload starts downloading the latest update asset.
func (s *SystemService) StartUpdateDownload(ctx context.Context, req UpdateDownloadRequest) (UpdateDownloadProgress, error) {
	if s.downloader == nil {
		s.downloader = ui.NewUpdateDownloader(s.logger)
	}
	return s.downloader.Start(ctx, req)
}

// GetUpdateDownloadProgress returns current download progress.
func (s *SystemService) GetUpdateDownloadProgress() (UpdateDownloadProgress, error) {
	if s.downloader == nil {
		s.downloader = ui.NewUpdateDownloader(s.logger)
	}
	return s.downloader.Progress(), nil
}

// StartUpdateInstall starts installing the downloaded update.
func (s *SystemService) StartUpdateInstall(ctx context.Context, req UpdateInstallRequest) (UpdateInstallProgress, error) {
	if s.installer == nil {
		s.installer = ui.NewUpdateInstaller(s.logger, func() {
			manager := s.deps.manager()
			if manager != nil {
				manager.Shutdown()
			}
		})
	}
	return s.installer.Start(ctx, req)
}

// GetUpdateInstallProgress returns current install progress.
func (s *SystemService) GetUpdateInstallProgress() (UpdateInstallProgress, error) {
	if s.installer == nil {
		s.installer = ui.NewUpdateInstaller(s.logger, func() {
			manager := s.deps.manager()
			if manager != nil {
				manager.Shutdown()
			}
		})
	}
	return s.installer.Progress(), nil
}

// ResolvemcpvmcpPath resolves the mcpvmcp executable path.
func (s *SystemService) ResolvemcpvmcpPath() string {
	return ui.ResolveMcpvmcpPath()
}
