package main

import (
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"mcpd/internal/app"
	"mcpd/internal/ui"
)

func main() {
	// 1. 初始化日志
	logger := createLogger()
	defer func() {
		_ = logger.Sync()
	}()

	coreApp := app.New(logger)

	wailsService := ui.NewWailsService(coreApp, logger)

	wailsApp := application.New(application.Options{
		Name:        "MCPD",
		Description: "MCP Server Manager",
		Services: []application.Service{
			application.NewService(wailsService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(Assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		LogLevel: slog.LevelInfo,
	})

	wailsService.SetWailsApp(wailsApp)

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
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

	logger.Info("starting MCPD Wails application")
	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

func createLogger() *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)

	logger, err := config.Build()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	return logger
}
