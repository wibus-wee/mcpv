package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"mcpd/internal/app"
	"mcpd/internal/domain"
	"mcpd/internal/infra/catalog"
)

// WailsService 为 Wails 前端提供 Go 服务接口
// 职责：桥接核心 app 层，不包含业务逻辑
type WailsService struct {
	coreApp *app.App
	logger  *zap.Logger
	wails   *application.App
	manager *Manager

	// Log streaming
	logMu        sync.Mutex
	logCancel    context.CancelFunc
	logStreaming bool
}

// NewWailsService 创建 Wails 服务适配层
func NewWailsService(coreApp *app.App, logger *zap.Logger) *WailsService {
	return &WailsService{
		coreApp: coreApp,
		logger:  logger.Named("wails-service"),
	}
}

// SetWailsApp 注入 Wails 应用实例（用于事件发送）
func (s *WailsService) SetWailsApp(wails *application.App) {
	s.wails = wails
}

// HandleURLScheme 处理 URL Scheme 唤起
// 职责：解析 URL 并转发给核心层或触发前端导航
func (s *WailsService) HandleURLScheme(rawURL string) error {
	s.logger.Info("received URL scheme", zap.String("url", rawURL))

	parsed, err := url.Parse(rawURL)
	if err != nil {
		s.logger.Error("failed to parse URL", zap.Error(err))
		return fmt.Errorf("invalid URL: %w", err)
	}

	// 提取路径和查询参数
	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")

	// 根据路径分发事件给前端
	if len(parts) > 0 {
		action := parts[0]
		s.emitNavigationEvent(action, parsed.Query())
	}

	return nil
}

// emitNavigationEvent 向前端发送导航事件
func (s *WailsService) emitNavigationEvent(action string, query url.Values) {
	if s.wails == nil {
		s.logger.Warn("wails app not set, cannot emit event")
		return
	}

	eventData := map[string]interface{}{
		"action": action,
		"params": query,
	}

	s.wails.Event.Emit("navigate", eventData)
	s.logger.Debug("emitted navigation event", zap.String("action", action))
}

// GetVersion 获取应用版本信息（示例导出方法）
// Wails 会自动为导出方法生成 JS 绑定
func (s *WailsService) GetVersion() string {
	return "0.1.0" // TODO: 从核心层或构建信息获取
}

// Ping 健康检查接口（示例）
func (s *WailsService) Ping(ctx context.Context) string {
	s.logger.Debug("ping received")
	return "pong"
}

// SetManager 注入 Manager 实例
func (s *WailsService) SetManager(manager *Manager) {
	s.manager = manager
}

// ResolveMcpdmcpPath 返回可执行的 mcpdmcp 路径，优先 PATH，其次同目录打包副本，失败时退回命令名
func (s *WailsService) ResolveMcpdmcpPath() string {
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

// CoreStateResponse 前端获取 Core 状态的响应
type CoreStateResponse struct {
	State  string `json:"state"`
	Uptime int64  `json:"uptime"`
	Error  string `json:"error,omitempty"`
}

// GetCoreState 获取 Core 当前状态
func (s *WailsService) GetCoreState() CoreStateResponse {
	if s.manager == nil {
		return CoreStateResponse{State: "unknown"}
	}

	state, err, uptime := s.manager.GetState()
	resp := CoreStateResponse{
		State:  string(state),
		Uptime: uptime,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

// StartCore 启动 Core
func (s *WailsService) StartCore(ctx context.Context) error {
	if s.manager == nil {
		return NewUIError(ErrCodeInternal, "Manager not initialized")
	}
	return s.manager.Start(ctx)
}

// StopCore 停止 Core
func (s *WailsService) StopCore() error {
	if s.manager == nil {
		return NewUIError(ErrCodeInternal, "Manager not initialized")
	}
	return s.manager.Stop()
}

// RestartCore 重启 Core
func (s *WailsService) RestartCore(ctx context.Context) error {
	if s.manager == nil {
		return NewUIError(ErrCodeInternal, "Manager not initialized")
	}
	return s.manager.Restart(ctx)
}

// ListTools 列出所有可用工具
func (s *WailsService) ListTools(ctx context.Context) ([]ToolEntry, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	snapshot, err := cp.ListToolsAllProfiles(ctx)
	if err != nil {
		return nil, MapDomainError(err)
	}

	// 更新缓存
	if s.manager != nil {
		s.manager.GetSharedState().SetToolSnapshot(snapshot)
	}

	return mapToolEntries(snapshot), nil
}

// ListResources 列出资源
func (s *WailsService) ListResources(ctx context.Context, cursor string) (*ResourcePage, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	page, err := cp.ListResourcesAllProfiles(ctx, cursor)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapResourcePage(page), nil
}

// ListPrompts 列出提示模板
func (s *WailsService) ListPrompts(ctx context.Context, cursor string) (*PromptPage, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	page, err := cp.ListPromptsAllProfiles(ctx, cursor)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapPromptPage(page), nil
}

// CallTool 调用指定工具
func (s *WailsService) CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	specKey := s.extractSpecKeyFromCache(name)
	result, err := cp.CallToolAllProfiles(ctx, name, args, routingKey, specKey)
	if err != nil {
		return nil, MapDomainError(err)
	}
	return result, nil
}

// ReadResource 读取资源内容
func (s *WailsService) ReadResource(ctx context.Context, uri string) (json.RawMessage, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	result, err := cp.ReadResourceAllProfiles(ctx, uri, "")
	if err != nil {
		return nil, MapDomainError(err)
	}
	return result, nil
}

// GetPrompt 获取提示模板内容
func (s *WailsService) GetPrompt(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	result, err := cp.GetPromptAllProfiles(ctx, name, args, "")
	if err != nil {
		return nil, MapDomainError(err)
	}
	return result, nil
}

// InfoResponse 控制面板信息响应
type InfoResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Build   string `json:"build"`
}

// GetInfo 获取控制面板信息
func (s *WailsService) GetInfo(ctx context.Context) (InfoResponse, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return InfoResponse{}, err
	}

	info, err := cp.Info(ctx)
	if err != nil {
		return InfoResponse{}, MapDomainError(err)
	}
	return InfoResponse{
		Name:    info.Name,
		Version: info.Version,
		Build:   info.Build,
	}, nil
}

// getControlPlane 获取 ControlPlane,统一错误处理
func (s *WailsService) getControlPlane() (domain.ControlPlane, error) {
	if s.manager == nil {
		return nil, NewUIError(ErrCodeInternal, "Manager not initialized")
	}
	return s.manager.GetControlPlane()
}

// extractSpecKeyFromCache 从缓存的 tool snapshot 中提取 specKey
func (s *WailsService) extractSpecKeyFromCache(toolName string) string {
	if s.manager == nil {
		return ""
	}
	snapshot := s.manager.GetSharedState().GetToolSnapshot()
	for _, tool := range snapshot.Tools {
		if tool.Name == toolName {
			return tool.SpecKey
		}
	}
	return ""
}

// StartLogStream 开始日志流（通过 Wails 事件推送）
func (s *WailsService) StartLogStream(ctx context.Context, minLevel string) error {
	s.logger.Info("StartLogStream called", zap.String("minLevel", minLevel))
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.manager == nil {
		s.logger.Error("StartLogStream failed: Manager not initialized")
		return NewUIError(ErrCodeInternal, "Manager not initialized")
	}

	// 已经在流式传输
	if s.logStreaming {
		s.logger.Warn("StartLogStream: Log stream already active")
		return NewUIError(ErrCodeInvalidState, "Log stream already active")
	}

	cp, err := s.manager.GetControlPlane()
	if err != nil {
		s.logger.Error("StartLogStream failed: GetControlPlane error", zap.Error(err))
		return err
	}

	// 创建可取消的上下文
	s.logger.Info("Context status before creating streamCtx",
		zap.Bool("ctx.Done", ctx.Done() != nil),
		zap.Any("ctx.Err", ctx.Err()),
	)

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		s.logger.Error("Input context is already cancelled!", zap.Error(ctx.Err()))
		s.logStreaming = false
		return NewUIError(ErrCodeInternal, "Input context already cancelled")
	default:
		// context is still valid
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	s.logCancel = cancel
	s.logStreaming = true

	// 转换日志级别
	level := domain.LogLevel(minLevel)
	s.logger.Info("Calling ControlPlane.StreamLogsAllProfiles", zap.String("level", string(level)))

	// 启动流式传输
	logCh, err := cp.StreamLogsAllProfiles(streamCtx, level)
	if err != nil {
		s.logger.Error("StreamLogs failed", zap.Error(err))
		s.logStreaming = false
		s.logCancel = nil
		return MapDomainError(err)
	}

	s.logger.Info("StreamLogs channel created successfully")

	// 后台协程处理日志推送
	go s.handleLogStream(streamCtx, logCh)

	s.logger.Info("log stream started, background goroutine launched", zap.String("minLevel", minLevel))
	return nil
}

// handleLogStream 处理日志流推送
func (s *WailsService) handleLogStream(ctx context.Context, logCh <-chan domain.LogEntry) {
	s.logger.Info("handleLogStream: goroutine started, waiting for log entries",
		zap.Bool("ctx.Done", ctx.Done() != nil),
		zap.Any("ctx.Err", ctx.Err()),
	)
	count := 0

	// Test if channel is already closed
	select {
	case entry, ok := <-logCh:
		if !ok {
			s.logger.Error("handleLogStream: channel was already closed!")
			s.logMu.Lock()
			s.logStreaming = false
			s.logCancel = nil
			s.logMu.Unlock()
			return
		}
		// Put the entry back by processing it
		count++
		s.logger.Info("Received FIRST log entry",
			zap.String("logger", entry.Logger),
			zap.String("level", string(entry.Level)),
		)
		// Reuse existing event emitter
		emitLogEntry(s.wails, entry)
	default:
		s.logger.Info("handleLogStream: no immediate entries, entering select loop")
	}

	for {
		select {
		case <-ctx.Done():
			s.logMu.Lock()
			s.logStreaming = false
			s.logCancel = nil
			s.logMu.Unlock()
			s.logger.Info("log stream stopped by context", zap.Int("totalEntriesProcessed", count))
			return
		case entry, ok := <-logCh:
			if !ok {
				s.logMu.Lock()
				s.logStreaming = false
				s.logCancel = nil
				s.logMu.Unlock()
				s.logger.Info("log stream channel closed", zap.Int("totalEntriesProcessed", count))
				return
			}
			count++
			s.logger.Debug("Received log entry",
				zap.Int("count", count),
				zap.String("logger", entry.Logger),
				zap.String("level", string(entry.Level)),
				zap.String("timestamp", entry.Timestamp.Format("15:04:05.000")),
			)
			// Reuse existing event emitter
			emitLogEntry(s.wails, entry)
			if count == 1 {
				s.logger.Info("First log entry emitted to frontend")
			}
		}
	}
}

// StopLogStream 停止日志流
func (s *WailsService) StopLogStream() {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.logCancel != nil {
		s.logCancel()
		s.logCancel = nil
	}
	s.logStreaming = false
}

// =============================================================================
// Configuration Management Methods
// =============================================================================

// GetConfigPath 获取配置文件路径
func (s *WailsService) GetConfigPath() string {
	if s.manager == nil {
		return ""
	}
	return s.manager.GetConfigPath()
}

// GetConfigMode returns configuration mode metadata.
func (s *WailsService) GetConfigMode() ConfigModeResponse {
	if s.manager == nil {
		return ConfigModeResponse{Mode: "unknown", Path: ""}
	}

	path := s.manager.GetConfigPath()
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

// OpenConfigInEditor opens the configuration file/directory in the system default editor
func (s *WailsService) OpenConfigInEditor(ctx context.Context) error {
	if s.manager == nil {
		return NewUIError(ErrCodeInternal, "Manager not initialized")
	}

	path := s.manager.GetConfigPath()
	if path == "" {
		return NewUIError(ErrCodeNotFound, "No configuration path configured")
	}

	// Platform-specific open command
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

// ImportMcpServers writes imported MCP servers into selected profiles.
func (s *WailsService) ImportMcpServers(ctx context.Context, req ImportMcpServersRequest) error {
	editor, err := s.catalogEditor()
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

// SetServerDisabled updates the disabled state for a server in a profile.
func (s *WailsService) SetServerDisabled(ctx context.Context, req UpdateServerStateRequest) error {
	editor, err := s.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.SetServerDisabled(ctx, req.Profile, req.Server, req.Disabled); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// DeleteServer removes a server from a profile.
func (s *WailsService) DeleteServer(ctx context.Context, req DeleteServerRequest) error {
	editor, err := s.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.DeleteServer(ctx, req.Profile, req.Server); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// CreateProfile creates a new profile file in the profile store.
func (s *WailsService) CreateProfile(ctx context.Context, req CreateProfileRequest) error {
	editor, err := s.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.CreateProfile(ctx, req.Name); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// DeleteProfile deletes a profile file from the profile store.
func (s *WailsService) DeleteProfile(ctx context.Context, req DeleteProfileRequest) error {
	editor, err := s.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.DeleteProfile(ctx, req.Name); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// SetCallerMapping updates a caller to profile mapping.
func (s *WailsService) SetCallerMapping(ctx context.Context, req UpdateCallerMappingRequest) error {
	editor, err := s.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.SetCallerMapping(ctx, req.Caller, req.Profile); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// RemoveCallerMapping removes a caller to profile mapping.
func (s *WailsService) RemoveCallerMapping(ctx context.Context, caller string) error {
	editor, err := s.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.RemoveCallerMapping(ctx, caller); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// GetRuntimeStatus returns the runtime status of all server pools
func (s *WailsService) GetRuntimeStatus(ctx context.Context) ([]ServerRuntimeStatus, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	pools, err := cp.GetPoolStatus(ctx)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapRuntimeStatuses(pools), nil
}

// GetServerInitStatus returns per-server initialization status
func (s *WailsService) GetServerInitStatus(ctx context.Context) ([]ServerInitStatus, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	statuses, err := cp.GetServerInitStatus(ctx)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapServerInitStatuses(statuses), nil
}

// ListProfiles 列出所有 profiles
func (s *WailsService) ListProfiles(ctx context.Context) ([]ProfileSummary, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	store := cp.GetProfileStore()

	// 提取所有 profile 名称
	names := make([]string, 0, len(store.Profiles))
	for name := range store.Profiles {
		names = append(names, name)
	}

	// 排序：default 优先，其余按字母顺序
	slices.SortStableFunc(names, func(a, b string) int {
		if a == domain.DefaultProfileName {
			return -1
		}
		if b == domain.DefaultProfileName {
			return 1
		}
		return strings.Compare(a, b)
	})

	// 按排序后的顺序构建结果
	result := make([]ProfileSummary, 0, len(store.Profiles))
	for _, name := range names {
		profile := store.Profiles[name]
		result = append(result, ProfileSummary{
			Name:        name,
			ServerCount: len(profile.Catalog.Specs),
			IsDefault:   name == domain.DefaultProfileName,
		})
	}

	return result, nil
}

// GetProfile 获取指定 profile 的详细信息
func (s *WailsService) GetProfile(ctx context.Context, name string) (*ProfileDetail, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	store := cp.GetProfileStore()
	profile, ok := store.Profiles[name]
	if !ok {
		return nil, NewUIError(ErrCodeNotFound, fmt.Sprintf("Profile %q not found", name))
	}

	servers := make([]ServerSpecDetail, 0, len(profile.Catalog.Specs))
	for _, spec := range profile.Catalog.Specs {
		specKey, err := domain.SpecFingerprint(spec)
		if err != nil {
			return nil, NewUIError(ErrCodeInternal, fmt.Sprintf("spec fingerprint for %q: %v", spec.Name, err))
		}
		servers = append(servers, mapServerSpecDetail(spec, specKey))
	}

	return &ProfileDetail{
		Name:    profile.Name,
		Runtime: mapRuntimeConfigDetail(profile.Catalog.Runtime),
		Servers: servers,
		SubAgent: ProfileSubAgentConfigDetail{
			Enabled: profile.Catalog.SubAgent.Enabled,
		},
	}, nil
}

// GetCallers 获取 caller 到 profile 的映射
func (s *WailsService) GetCallers(ctx context.Context) (map[string]string, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	store := cp.GetProfileStore()
	// Return a copy to prevent external modification
	result := make(map[string]string, len(store.Callers))
	for k, v := range store.Callers {
		result[k] = v
	}

	return result, nil
}

// GetActiveCallers returns active caller registrations.
func (s *WailsService) GetActiveCallers(ctx context.Context) ([]ActiveCaller, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	callers, err := cp.ListActiveCallers(ctx)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return mapActiveCallers(callers), nil
}

func (s *WailsService) catalogEditor() (*catalog.Editor, error) {
	if s.manager == nil {
		return nil, NewUIError(ErrCodeInternal, "Manager not initialized")
	}
	path := strings.TrimSpace(s.manager.GetConfigPath())
	if path == "" {
		return nil, NewUIError(ErrCodeInvalidConfig, "Configuration path is not available")
	}
	return catalog.NewEditor(path, s.logger), nil
}

func mapCatalogError(err error) error {
	if err == nil {
		return nil
	}
	var editorErr *catalog.EditorError
	if errors.As(err, &editorErr) {
		detail := ""
		if editorErr.Err != nil {
			detail = editorErr.Err.Error()
		}
		switch editorErr.Kind {
		case catalog.EditorErrorInvalidRequest:
			return NewUIErrorWithDetails(ErrCodeInvalidRequest, editorErr.Message, detail)
		case catalog.EditorErrorProfileNotFound:
			return NewUIErrorWithDetails(ErrCodeProfileNotFound, editorErr.Message, detail)
		case catalog.EditorErrorInvalidConfig:
			return NewUIErrorWithDetails(ErrCodeInvalidConfig, editorErr.Message, detail)
		default:
			return NewUIErrorWithDetails(ErrCodeInvalidConfig, editorErr.Message, detail)
		}
	}
	return NewUIErrorWithDetails(ErrCodeInvalidConfig, "Failed to update configuration", err.Error())
}

// =============================================================================
// SubAgent Configuration Methods
// =============================================================================

// GetSubAgentConfig returns the runtime-level SubAgent LLM provider configuration.
func (s *WailsService) GetSubAgentConfig(ctx context.Context) (SubAgentConfigDetail, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return SubAgentConfigDetail{}, err
	}

	// Get runtime config from any profile (they share the same runtime config)
	store := cp.GetProfileStore()
	for _, profile := range store.Profiles {
		cfg := profile.Catalog.Runtime.SubAgent
		return SubAgentConfigDetail{
			Model:              cfg.Model,
			Provider:           cfg.Provider,
			APIKeyEnvVar:       cfg.APIKeyEnvVar,
			BaseURL:            cfg.BaseURL,
			MaxToolsPerRequest: cfg.MaxToolsPerRequest,
			FilterPrompt:       cfg.FilterPrompt,
		}, nil
	}

	return SubAgentConfigDetail{}, nil
}

// GetProfileSubAgentConfig returns the per-profile SubAgent enabled state.
func (s *WailsService) GetProfileSubAgentConfig(ctx context.Context, profileName string) (ProfileSubAgentConfigDetail, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return ProfileSubAgentConfigDetail{}, err
	}

	store := cp.GetProfileStore()
	profile, ok := store.Profiles[profileName]
	if !ok {
		return ProfileSubAgentConfigDetail{}, NewUIError(ErrCodeNotFound, fmt.Sprintf("Profile %q not found", profileName))
	}

	return ProfileSubAgentConfigDetail{
		Enabled: profile.Catalog.SubAgent.Enabled,
	}, nil
}

// SetProfileSubAgentEnabled updates the per-profile SubAgent enabled state.
func (s *WailsService) SetProfileSubAgentEnabled(ctx context.Context, req UpdateProfileSubAgentRequest) error {
	editor, err := s.catalogEditor()
	if err != nil {
		return err
	}
	if err := editor.SetProfileSubAgentEnabled(ctx, req.Profile, req.Enabled); err != nil {
		return mapCatalogError(err)
	}
	return nil
}

// IsSubAgentAvailable returns whether the SubAgent infrastructure is configured at runtime level.
func (s *WailsService) IsSubAgentAvailable(ctx context.Context) bool {
	cp, err := s.getControlPlane()
	if err != nil {
		return false
	}
	return cp.IsSubAgentEnabled()
}
