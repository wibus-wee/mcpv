package ui

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"mcpd/internal/domain"
)

// LogService streams logs to the frontend.
type LogService struct {
	deps   *ServiceDeps
	logger *zap.Logger

	logMu        sync.Mutex
	logCancel    context.CancelFunc
	logStreaming bool
}

func NewLogService(deps *ServiceDeps) *LogService {
	return &LogService{
		deps:   deps,
		logger: deps.loggerNamed("log-service"),
	}
}

// StartLogStream starts log streaming via Wails events.
func (s *LogService) StartLogStream(ctx context.Context, minLevel string) error {
	s.logger.Info("StartLogStream called", zap.String("minLevel", minLevel))
	s.logMu.Lock()
	defer s.logMu.Unlock()

	manager := s.deps.manager()
	if manager == nil {
		s.logger.Error("StartLogStream failed: Manager not initialized")
		return NewUIError(ErrCodeInternal, "Manager not initialized")
	}

	if s.logStreaming {
		s.logger.Warn("StartLogStream: Log stream already active")
		return NewUIError(ErrCodeInvalidState, "Log stream already active")
	}

	cp, err := manager.GetControlPlane()
	if err != nil {
		s.logger.Error("StartLogStream failed: GetControlPlane error", zap.Error(err))
		return err
	}

	s.logger.Info("Context status before creating streamCtx",
		zap.Bool("ctx.Done", ctx.Done() != nil),
		zap.Any("ctx.Err", ctx.Err()),
	)

	select {
	case <-ctx.Done():
		s.logger.Error("Input context is already cancelled!", zap.Error(ctx.Err()))
		s.logStreaming = false
		return NewUIError(ErrCodeInternal, "Input context already cancelled")
	default:
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	s.logCancel = cancel
	s.logStreaming = true

	level := domain.LogLevel(minLevel)
	s.logger.Info("Calling ControlPlane.StreamLogsAllProfiles", zap.String("level", string(level)))

	logCh, err := cp.StreamLogsAllProfiles(streamCtx, level)
	if err != nil {
		s.logger.Error("StreamLogs failed", zap.Error(err))
		s.logStreaming = false
		s.logCancel = nil
		return MapDomainError(err)
	}

	s.logger.Info("StreamLogs channel created successfully")

	go s.handleLogStream(streamCtx, logCh)

	s.logger.Info("log stream started, background goroutine launched", zap.String("minLevel", minLevel))
	return nil
}

func (s *LogService) handleLogStream(ctx context.Context, logCh <-chan domain.LogEntry) {
	s.logger.Info("handleLogStream: goroutine started, waiting for log entries",
		zap.Bool("ctx.Done", ctx.Done() != nil),
		zap.Any("ctx.Err", ctx.Err()),
	)
	count := 0

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
		count++
		s.logger.Info("Received FIRST log entry",
			zap.String("logger", entry.Logger),
			zap.String("level", string(entry.Level)),
		)
		emitLogEntry(s.deps.wailsApp(), entry)
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
			emitLogEntry(s.deps.wailsApp(), entry)
			if count == 1 {
				s.logger.Info("First log entry emitted to frontend")
			}
		}
	}
}

// StopLogStream stops log streaming.
func (s *LogService) StopLogStream() {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if s.logCancel != nil {
		s.logCancel()
		s.logCancel = nil
	}
	s.logStreaming = false
}
