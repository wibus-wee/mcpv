package services

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	catalogeditor "mcpv/internal/infra/catalog/editor"
	catalogloader "mcpv/internal/infra/catalog/loader"
	"mcpv/internal/infra/rpc"
	"mcpv/internal/ui"
	"mcpv/internal/ui/mapping"
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
	if s.deps.isRemoteMode() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()

		remote, err := s.deps.remoteControlPlane()
		if err != nil {
			return ConfigModeResponse{Mode: "unknown", Path: ""}
		}
		mode, err := remote.GetConfigMode(ctx)
		if err != nil {
			return ConfigModeResponse{Mode: "unknown", Path: ""}
		}
		return ConfigModeResponse{
			Mode:       mode.Mode,
			Path:       mode.Path,
			IsWritable: mode.IsWritable,
		}
	}

	manager := s.deps.manager()
	if manager == nil {
		return ConfigModeResponse{Mode: "unknown", Path: ""}
	}

	path := manager.GetConfigPath()
	if path == "" {
		return ConfigModeResponse{Mode: "unknown", Path: ""}
	}

	editor := catalogeditor.NewEditor(path, s.logger)
	info, err := editor.Inspect(context.Background())
	if err != nil {
		return ConfigModeResponse{Mode: "unknown", Path: path}
	}
	return ConfigModeResponse{
		Mode:       "file",
		Path:       info.Path,
		IsWritable: info.IsWritable,
	}
}

// GetRuntimeConfig loads runtime configuration from the config file.
func (s *ConfigService) GetRuntimeConfig(ctx context.Context) (RuntimeConfigDetail, error) {
	if s.deps.isRemoteMode() {
		remote, err := s.deps.remoteControlPlane()
		if err != nil {
			return RuntimeConfigDetail{}, err
		}
		runtimeCfg, err := remote.GetRuntimeConfig(ctx)
		if err != nil {
			return RuntimeConfigDetail{}, ui.MapDomainError(err)
		}
		return mapping.MapRuntimeConfigDetail(runtimeCfg), nil
	}

	manager := s.deps.manager()
	if manager == nil {
		return RuntimeConfigDetail{}, ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	path := strings.TrimSpace(manager.GetConfigPath())
	if path == "" {
		return RuntimeConfigDetail{}, ui.NewError(ui.ErrCodeInvalidConfig, "Configuration path is not available")
	}

	loader := catalogloader.NewLoader(s.logger)
	runtime, err := loader.LoadRuntimeConfig(ctx, path)
	if err != nil {
		return RuntimeConfigDetail{}, ui.NewErrorWithDetails(
			ui.ErrCodeInvalidConfig,
			"Failed to load runtime config",
			err.Error(),
		)
	}
	return mapping.MapRuntimeConfigDetail(runtime), nil
}

// OpenConfigInEditor opens config path in system editor.
func (s *ConfigService) OpenConfigInEditor(ctx context.Context) error {
	if s.deps.isRemoteMode() {
		return ui.NewError(ui.ErrCodeInvalidState, "Remote configuration cannot be opened locally")
	}
	manager := s.deps.manager()
	if manager == nil {
		return ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}

	path := manager.GetConfigPath()
	if path == "" {
		return ui.NewError(ui.ErrCodeNotFound, "No configuration path configured")
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
		return ui.NewError(ui.ErrCodeInternal, fmt.Sprintf("Unsupported platform: %s", runtime.GOOS))
	}

	if err := cmd.Start(); err != nil {
		s.logger.Error("failed to open config in editor", zap.Error(err), zap.String("path", path))
		return ui.NewError(ui.ErrCodeInternal, fmt.Sprintf("Failed to open editor: %v", err))
	}

	s.logger.Info("opened config in editor", zap.String("path", path))
	return nil
}

// ReloadConfig triggers a configuration reload in the running Core.
func (s *ConfigService) ReloadConfig(ctx context.Context) error {
	if s.deps.isRemoteMode() {
		remote, err := s.deps.remoteControlPlane()
		if err != nil {
			return err
		}
		if err := remote.ReloadConfig(ctx); err != nil {
			return ui.MapDomainError(err)
		}
		return nil
	}
	manager := s.deps.manager()
	if manager == nil {
		return ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}
	return manager.ReloadConfig(ctx)
}

// UpdateRuntimeConfig writes runtime updates into the config file.
func (s *ConfigService) UpdateRuntimeConfig(ctx context.Context, req UpdateRuntimeConfigRequest) error {
	if s.deps.isRemoteMode() {
		remote, err := s.deps.remoteControlPlane()
		if err != nil {
			return err
		}
		payload := rpc.RuntimeConfigUpdatePayload{
			RouteTimeoutSeconds:         req.RouteTimeoutSeconds,
			PingIntervalSeconds:         req.PingIntervalSeconds,
			ToolRefreshSeconds:          req.ToolRefreshSeconds,
			ToolRefreshConcurrency:      req.ToolRefreshConcurrency,
			ClientCheckSeconds:          req.ClientCheckSeconds,
			ClientInactiveSeconds:       req.ClientInactiveSeconds,
			ServerInitRetryBaseSeconds:  req.ServerInitRetryBaseSeconds,
			ServerInitRetryMaxSeconds:   req.ServerInitRetryMaxSeconds,
			ServerInitMaxRetries:        req.ServerInitMaxRetries,
			ReloadMode:                  req.ReloadMode,
			BootstrapMode:               req.BootstrapMode,
			BootstrapConcurrency:        req.BootstrapConcurrency,
			BootstrapTimeoutSeconds:     req.BootstrapTimeoutSeconds,
			DefaultActivationMode:       req.DefaultActivationMode,
			ExposeTools:                 req.ExposeTools,
			ToolNamespaceStrategy:       req.ToolNamespaceStrategy,
			ProxyMode:                   req.ProxyMode,
			ProxyURL:                    req.ProxyURL,
			ProxyNoProxy:                req.ProxyNoProxy,
			ObservabilityListenAddress:  req.ObservabilityListenAddress,
			ObservabilityMetricsEnabled: req.ObservabilityMetricsEnabled,
			ObservabilityHealthzEnabled: req.ObservabilityHealthzEnabled,
		}
		if err := remote.UpdateRuntimeConfig(ctx, payload); err != nil {
			return ui.MapDomainError(err)
		}
		return nil
	}

	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}
	update := catalogeditor.RuntimeConfigUpdate{
		RouteTimeoutSeconds:         req.RouteTimeoutSeconds,
		PingIntervalSeconds:         req.PingIntervalSeconds,
		ToolRefreshSeconds:          req.ToolRefreshSeconds,
		ToolRefreshConcurrency:      req.ToolRefreshConcurrency,
		ClientCheckSeconds:          req.ClientCheckSeconds,
		ClientInactiveSeconds:       req.ClientInactiveSeconds,
		ServerInitRetryBaseSeconds:  req.ServerInitRetryBaseSeconds,
		ServerInitRetryMaxSeconds:   req.ServerInitRetryMaxSeconds,
		ServerInitMaxRetries:        req.ServerInitMaxRetries,
		ReloadMode:                  req.ReloadMode,
		BootstrapMode:               req.BootstrapMode,
		BootstrapConcurrency:        req.BootstrapConcurrency,
		BootstrapTimeoutSeconds:     req.BootstrapTimeoutSeconds,
		DefaultActivationMode:       req.DefaultActivationMode,
		ExposeTools:                 req.ExposeTools,
		ToolNamespaceStrategy:       req.ToolNamespaceStrategy,
		ProxyMode:                   req.ProxyMode,
		ProxyURL:                    req.ProxyURL,
		ProxyNoProxy:                req.ProxyNoProxy,
		ObservabilityListenAddress:  req.ObservabilityListenAddress,
		ObservabilityMetricsEnabled: req.ObservabilityMetricsEnabled,
		ObservabilityHealthzEnabled: req.ObservabilityHealthzEnabled,
	}
	if err := editor.UpdateRuntimeConfig(ctx, update); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// ImportMcpServers writes imported MCP servers into the config file.
func (s *ConfigService) ImportMcpServers(ctx context.Context, req ImportMcpServersRequest) error {
	importReq := catalogeditor.ImportRequest{
		Servers: make([]domain.ServerSpec, 0, len(req.Servers)),
	}
	for _, server := range req.Servers {
		importReq.Servers = append(importReq.Servers, domain.ServerSpec{
			Name:            strings.TrimSpace(server.Name),
			Transport:       domain.TransportKind(strings.TrimSpace(server.Transport)),
			Cmd:             append([]string{}, server.Cmd...),
			Env:             server.Env,
			Cwd:             strings.TrimSpace(server.Cwd),
			Tags:            append([]string(nil), server.Tags...),
			ProtocolVersion: strings.TrimSpace(server.ProtocolVersion),
			HTTP:            mapStreamableHTTPConfig(server.HTTP),
		})
	}

	if s.deps.isRemoteMode() {
		remote, err := s.deps.remoteControlPlane()
		if err != nil {
			return err
		}
		if err := remote.ImportServers(ctx, importReq.Servers); err != nil {
			return ui.MapDomainError(err)
		}
		return nil
	}

	editor, err := s.deps.catalogEditor()
	if err != nil {
		return err
	}

	if err := editor.ImportServers(ctx, importReq); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

func mapStreamableHTTPConfig(cfg *StreamableHTTPConfigDetail) *domain.StreamableHTTPConfig {
	if cfg == nil {
		return nil
	}
	headers := cfg.Headers
	if headers == nil {
		headers = make(map[string]string)
	}
	var proxyCfg *domain.ProxyConfig
	if cfg.Proxy != nil {
		proxyCfg = &domain.ProxyConfig{
			Mode:    domain.ProxyMode(strings.TrimSpace(cfg.Proxy.Mode)),
			URL:     strings.TrimSpace(cfg.Proxy.URL),
			NoProxy: strings.TrimSpace(cfg.Proxy.NoProxy),
		}
	}
	return &domain.StreamableHTTPConfig{
		Endpoint:   strings.TrimSpace(cfg.Endpoint),
		Headers:    headers,
		MaxRetries: cfg.MaxRetries,
		Proxy:      proxyCfg,
	}
}
