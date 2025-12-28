package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"mcpd/internal/app"
	"mcpd/internal/domain"
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

	// 对 Wails UI,使用内部 caller 标识
	snapshot, err := cp.ListTools(ctx, "wails-ui")
	if err != nil {
		return nil, MapDomainError(err)
	}

	// 更新缓存
	if s.manager != nil {
		s.manager.GetSharedState().SetToolSnapshot(snapshot)
	}

	// 转换为前端类型
	result := make([]ToolEntry, 0, len(snapshot.Tools))
	for _, tool := range snapshot.Tools {
		result = append(result, ToolEntry{
			Name:     tool.Name,
			ToolJSON: tool.ToolJSON,
		})
	}
	return result, nil
}

// ListResources 列出资源
func (s *WailsService) ListResources(ctx context.Context, cursor string) (*ResourcePage, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	page, err := cp.ListResources(ctx, "wails-ui", cursor)
	if err != nil {
		return nil, MapDomainError(err)
	}

	// 转换为前端类型
	result := &ResourcePage{
		NextCursor: page.NextCursor,
		Resources:  make([]ResourceEntry, 0, len(page.Snapshot.Resources)),
	}
	for _, res := range page.Snapshot.Resources {
		result.Resources = append(result.Resources, ResourceEntry{
			URI:          res.URI,
			ResourceJSON: res.ResourceJSON,
		})
	}
	return result, nil
}

// ListPrompts 列出提示模板
func (s *WailsService) ListPrompts(ctx context.Context, cursor string) (*PromptPage, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	page, err := cp.ListPrompts(ctx, "wails-ui", cursor)
	if err != nil {
		return nil, MapDomainError(err)
	}

	// 转换为前端类型
	result := &PromptPage{
		NextCursor: page.NextCursor,
		Prompts:    make([]PromptEntry, 0, len(page.Snapshot.Prompts)),
	}
	for _, p := range page.Snapshot.Prompts {
		result.Prompts = append(result.Prompts, PromptEntry{
			Name:       p.Name,
			PromptJSON: p.PromptJSON,
		})
	}
	return result, nil
}

// CallTool 调用指定工具
func (s *WailsService) CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error) {
	cp, err := s.getControlPlane()
	if err != nil {
		return nil, err
	}

	result, err := cp.CallTool(ctx, "wails-ui", name, args, routingKey)
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

	result, err := cp.ReadResource(ctx, "wails-ui", uri)
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

	result, err := cp.GetPrompt(ctx, "wails-ui", name, args)
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

// StartLogStream 开始日志流（通过 Wails 事件推送）
func (s *WailsService) StartLogStream(ctx context.Context, minLevel string) error {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.manager == nil {
		return NewUIError(ErrCodeInternal, "Manager not initialized")
	}

	// 已经在流式传输
	if s.logStreaming {
		return NewUIError(ErrCodeInvalidState, "Log stream already active")
	}

	cp, err := s.manager.GetControlPlane()
	if err != nil {
		return err
	}

	// 创建可取消的上下文
	streamCtx, cancel := context.WithCancel(ctx)
	s.logCancel = cancel
	s.logStreaming = true

	// 转换日志级别
	level := domain.LogLevel(minLevel)

	// 启动流式传输
	logCh, err := cp.StreamLogs(streamCtx, "wails-ui", level)
	if err != nil {
		s.logStreaming = false
		s.logCancel = nil
		return MapDomainError(err)
	}

	// 后台协程处理日志推送
	go s.handleLogStream(streamCtx, logCh)

	s.logger.Debug("log stream started", zap.String("minLevel", minLevel))
	return nil
}

// handleLogStream 处理日志流推送
func (s *WailsService) handleLogStream(ctx context.Context, logCh <-chan domain.LogEntry) {
	for {
		select {
		case <-ctx.Done():
			s.logMu.Lock()
			s.logStreaming = false
			s.logCancel = nil
			s.logMu.Unlock()
			s.logger.Debug("log stream stopped")
			return
		case entry, ok := <-logCh:
			if !ok {
				s.logMu.Lock()
				s.logStreaming = false
				s.logCancel = nil
				s.logMu.Unlock()
				s.logger.Debug("log stream channel closed")
				return
			}
			// Reuse existing event emitter
			emitLogEntry(s.wails, entry)
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
