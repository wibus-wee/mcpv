package ui

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"

	"mcpd/internal/app"
)

// WailsService 为 Wails 前端提供 Go 服务接口
// 职责：桥接核心 app 层，不包含业务逻辑
type WailsService struct {
	coreApp *app.App
	logger  *zap.Logger
	wails   *application.App
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
