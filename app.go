package main

import (
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"mcpd/internal/app"
	"mcpd/internal/infra/telemetry"
	"mcpd/internal/ui"
)

func main() {
	// 1. 初始化日志（带 LogBroadcaster）
	logger, logBroadcaster := createLoggerWithBroadcaster()
	defer func() {
		_ = logger.Sync()
	}()

	// 2. 创建核心应用和 UI Manager
	coreApp := app.NewWithBroadcaster(logger, logBroadcaster)
	configPath := "runtime.yaml" // Default config file for UI boot; override via flags or settings.

	uiLogger := logger.With(zap.String(telemetry.FieldLogSource, telemetry.LogSourceUI))
	serviceRegistry := ui.NewServiceRegistry(coreApp, uiLogger)
	manager := ui.NewManager(nil, coreApp, configPath)
	serviceRegistry.SetManager(manager)

	wailsApp := application.New(application.Options{
		Name:        "MCPD",
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

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "mcpd",
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

	uiLogger.Info("starting MCPD Wails application")
	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
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

func createLogger() *zap.Logger {
	logger, _ := createLoggerWithBroadcaster()
	return logger
}
