package app

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"mcpv/internal/infra/telemetry"
)

// LoggingConfig configures logging wiring.
type LoggingConfig struct {
	Logger      *zap.Logger
	Broadcaster *telemetry.LogBroadcaster
}

// Logging bundles the logger and broadcaster.
type Logging struct {
	Logger      *zap.Logger
	Broadcaster *telemetry.LogBroadcaster
}

// NewLogging constructs logging dependencies.
func NewLogging(cfg LoggingConfig) Logging {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	logger = logger.With(zap.String(telemetry.FieldLogSource, telemetry.LogSourceCore)).Named("app")

	if cfg.Broadcaster != nil {
		return Logging{
			Logger:      logger,
			Broadcaster: cfg.Broadcaster,
		}
	}

	logs := telemetry.NewLogBroadcaster(zapcore.DebugLevel)
	logger = logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, logs.Core())
	}))

	return Logging{
		Logger:      logger,
		Broadcaster: logs,
	}
}

// NewLogger returns the logger from a Logging bundle.
func NewLogger(logging Logging) *zap.Logger {
	return logging.Logger
}

// NewLogBroadcaster returns the broadcaster from a Logging bundle.
func NewLogBroadcaster(logging Logging) *telemetry.LogBroadcaster {
	return logging.Broadcaster
}
