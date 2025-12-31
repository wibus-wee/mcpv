package app

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"mcpd/internal/infra/telemetry"
)

type LoggingConfig struct {
	Logger      *zap.Logger
	Broadcaster *telemetry.LogBroadcaster
}

type Logging struct {
	Logger      *zap.Logger
	Broadcaster *telemetry.LogBroadcaster
}

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

func NewLogger(logging Logging) *zap.Logger {
	return logging.Logger
}

func NewLogBroadcaster(logging Logging) *telemetry.LogBroadcaster {
	return logging.Broadcaster
}
