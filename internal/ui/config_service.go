package ui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/catalog"
)

// ConfigService exposes configuration management APIs.
type ConfigService struct {
	deps   *ServiceDeps
	logger *zap.Logger
}

func NewConfigService(deps *ServiceDeps) *ConfigService {
	return &ConfigService{
		deps:   deps,
		logger: deps.loggerNamed("config-service"),
	}
}

// GetConfigPath returns configuration path.
func (s *ConfigService) GetConfigPath() string {
	manager := s.deps.manager()
	if manager == nil {
		return ""
	}
	return manager.GetConfigPath()
}

// GetConfigMode returns configuration mode metadata.
func (s *ConfigService) GetConfigMode() ConfigModeResponse {
	manager := s.deps.manager()
	if manager == nil {
		return ConfigModeResponse{Mode: "unknown", Path: ""}
	}

	path := manager.GetConfigPath()
	if path == "" {
		return ConfigModeResponse{Mode: "unknown", Path: ""}
	}

	editor := catalog.NewEditor(path, s.logger)
	info, err := editor.Inspect(context.Background())
	if err != nil {
		return ConfigModeResponse{Mode: "unknown", Path: path}
	}
	return ConfigModeResponse{
		Mode:       "directory",
		Path:       info.Path,
		IsWritable: info.IsWritable,
	}
}

// OpenConfigInEditor opens config path in system editor.
func (s *ConfigService) OpenConfigInEditor(ctx context.Context) error {
	manager := s.deps.manager()
	if manager == nil {
		return NewUIError(ErrCodeInternal, "Manager not initialized")
	}

	path := manager.GetConfigPath()
	if path == "" {
		return NewUIError(ErrCodeNotFound, "No configuration path configured")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", path)
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd", "/c", "start", "", path)
	case "linux":
		cmd = exec.CommandContext(ctx, "xdg-open", path)
	default:
		return NewUIError(ErrCodeInternal, fmt.Sprintf("Unsupported platform: %s", runtime.GOOS))
	}

	if err := cmd.Start(); err != nil {
		s.logger.Error("failed to open config in editor", zap.Error(err), zap.String("path", path))
		return NewUIError(ErrCodeInternal, fmt.Sprintf("Failed to open editor: %v", err))
	}

	s.logger.Info("opened config in editor", zap.String("path", path))
	return nil
}

// ReloadConfig triggers a configuration reload in the running Core.
func (s *ConfigService) ReloadConfig(ctx context.Context) error {
	manager := s.deps.manager()
	if manager == nil {
		return NewUIError(ErrCodeInternal, "Manager not initialized")
	}
	return manager.ReloadConfig(ctx)
}

// UpdateRuntimeConfig writes runtime.yaml updates to the profile store.
func (s *ConfigService) UpdateRuntimeConfig(ctx context.Context, req UpdateRuntimeConfigRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	update := catalog.RuntimeConfigUpdate{
		RouteTimeoutSeconds:        req.RouteTimeoutSeconds,
		PingIntervalSeconds:        req.PingIntervalSeconds,
		ToolRefreshSeconds:         req.ToolRefreshSeconds,
		ToolRefreshConcurrency:     req.ToolRefreshConcurrency,
		CallerCheckSeconds:         req.CallerCheckSeconds,
		CallerInactiveSeconds:      req.CallerInactiveSeconds,
		ServerInitRetryBaseSeconds: req.ServerInitRetryBaseSeconds,
		ServerInitRetryMaxSeconds:  req.ServerInitRetryMaxSeconds,
		ServerInitMaxRetries:       req.ServerInitMaxRetries,
		BootstrapMode:              req.BootstrapMode,
		BootstrapConcurrency:       req.BootstrapConcurrency,
		BootstrapTimeoutSeconds:    req.BootstrapTimeoutSeconds,
		DefaultActivationMode:      req.DefaultActivationMode,
		ExposeTools:                req.ExposeTools,
		ToolNamespaceStrategy:      req.ToolNamespaceStrategy,
	}
	if err := editor.UpdateRuntimeConfig(ctx, update); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// ImportMcpServers writes imported MCP servers into selected profiles.
func (s *ConfigService) ImportMcpServers(ctx context.Context, req ImportMcpServersRequest) error {
	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}

	importReq := catalog.ImportRequest{
		Profiles: req.Profiles,
		Servers:  make([]domain.ServerSpec, 0, len(req.Servers)),
	}
	for _, server := range req.Servers {
		importReq.Servers = append(importReq.Servers, domain.ServerSpec{
			Name: strings.TrimSpace(server.Name),
			Cmd:  append([]string{}, server.Cmd...),
			Env:  server.Env,
			Cwd:  strings.TrimSpace(server.Cwd),
		})
	}

	if err := editor.ImportServers(ctx, importReq); err != nil {
		return mapCatalogError(err)
	}
	return nil
}
