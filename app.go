package main

import (
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"mcpv/internal/app"
	"mcpv/internal/infra/telemetry"
	"mcpv/internal/ui"
	"mcpv/internal/ui/services"
	"mcpv/internal/ui/types"
)

func main() {
	// 1. 初始化日志（带 LogBroadcaster）
	logger, logBroadcaster := createLoggerWithBroadcaster()
	defer func() {
		_ = logger.Sync()
	}()

	// 2. 创建核心应用和 UI Manager
	coreApp := app.NewWithBroadcaster(logger, logBroadcaster)
	configPath := ui.ResolveDefaultConfigPath()
	if err := ui.EnsureConfigFile(configPath); err != nil {
		logger.Warn("failed to prepare default config file", zap.Error(err), zap.String("config", configPath))
	}

	uiLogger := logger.With(zap.String(telemetry.FieldLogSource, telemetry.LogSourceUI))
	serviceRegistry := services.NewServiceRegistry(coreApp, uiLogger)
	manager := ui.NewManager(nil, coreApp, configPath)
	serviceRegistry.SetManager(manager)

	wailsApp := application.New(application.Options{
		Name:        "mcpv",
		Description: "MCP Server Manager",
		Services:    serviceRegistry.Services(),
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(Assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		LogLevel: slog.LevelInfo,
		OnShutdown: func() {
			manager.Shutdown()
		},
	})

	// 3. 注入 Wails 应用实例
	serviceRegistry.SetWailsApp(wailsApp)
	manager.SetWailsApp(wailsApp)

	updateChecker := ui.NewUpdateChecker(uiLogger, types.UpdateCheckOptions{
		IntervalHours:     24,
		IncludePrerelease: false,
	})
	manager.SetUpdateChecker(updateChecker)
	updateChecker.SetWailsApp(wailsApp)
	updateChecker.Start()

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "mcpv",
		Width:            1200,
		Height:           800,
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})

	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})

	// Handle deep link protocol invocations
	wailsApp.Event.On("ApplicationOpenURL", func(event *application.CustomEvent) {
		if rawURL, ok := event.Data.(string); ok {
			if err := manager.HandleDeepLink(rawURL); err != nil {
				uiLogger.Error("deep link handling failed", zap.Error(err), zap.String("url", rawURL))
			}
			// Always show window when deep link is triggered
			window.Show()
			window.Focus()
		}
	})

	uiLogger.Info("starting mcpv Wails application")
	if err := wailsApp.Run(); err != nil {
		logger.Error("wails run failed", zap.Error(err))
		return
	}
}

func createLoggerWithBroadcaster() (*zap.Logger, *telemetry.LogBroadcaster) {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)

	baseLogger, err := config.Build()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	// Create log broadcaster
	broadcaster := telemetry.NewLogBroadcaster(zapcore.InfoLevel)

	// Combine base logger with broadcaster
	logger := baseLogger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, broadcaster.Core())
	}))

	return logger, broadcaster
}
