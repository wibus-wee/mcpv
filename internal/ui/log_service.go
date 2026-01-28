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

	logMu      sync.Mutex
	logSession *logSession
}

func NewLogService(deps *ServiceDeps) *LogService {
	return &LogService{
		deps:   deps,
		logger: deps.loggerNamed("log-service"),
	}
}

type logSession struct {
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	doneOnce sync.Once
	minLevel domain.LogLevel
}

func (s *logSession) markDone() {
	s.doneOnce.Do(func() {
		close(s.done)
	})
}

// StartLogStream starts log streaming via Wails events.
func (s *LogService) StartLogStream(ctx context.Context, minLevel string) error {
	s.logger.Info("StartLogStream called", zap.String("minLevel", minLevel))

	manager := s.deps.manager()
	if manager == nil {
		s.logger.Error("StartLogStream failed: Manager not initialized")
		return NewError(ErrCodeInternal, "Manager not initialized")
	}

	cp, err := manager.GetControlPlane()
	if err != nil {
		s.logger.Error("StartLogStream failed: GetControlPlane error", zap.Error(err))
		return err
	}

	if ctx == nil {
		s.logger.Warn("StartLogStream received nil context, falling back to Background")
		ctx = context.Background()
	}

	s.logger.Info("Context status before creating streamCtx",
		zap.Bool("ctx.Done", ctx.Done() != nil),
		zap.Any("ctx.Err", ctx.Err()),
	)

	select {
	case <-ctx.Done():
		s.logger.Error("Input context is already cancelled!", zap.Error(ctx.Err()))
		return NewError(ErrCodeInternal, "Input context already cancelled")
	default:
	}

	level := domain.LogLevel(minLevel)
	s.logger.Info("Calling ControlPlane.StreamLogsAllServers", zap.String("level", string(level)))

	streamCtx, cancel := context.WithCancel(ctx)
	session := &logSession{
		ctx:      streamCtx,
		cancel:   cancel,
		done:     make(chan struct{}),
		minLevel: level,
	}
	prev := s.replaceLogSession(session)
	if prev != nil {
		s.logger.Info("Restarting log stream with new session",
			zap.String("minLevel", string(level)),
		)
	}

	logCh, err := cp.StreamLogsAllServers(streamCtx, level)
	if err != nil {
		s.logger.Error("StreamLogs failed", zap.Error(err))
		s.finishLogSession(session)
		return MapDomainError(err)
	}

	s.logger.Info("StreamLogs channel created successfully")

	go s.handleLogStream(session, logCh)

	s.logger.Info("log stream started, background goroutine launched", zap.String("minLevel", minLevel))
	return nil
}

func (s *LogService) handleLogStream(session *logSession, logCh <-chan domain.LogEntry) {
	ctx := session.ctx
	defer s.finishLogSession(session)
	s.logger.Info("handleLogStream: goroutine started, waiting for log entries",
		zap.Bool("ctx.Done", ctx.Done() != nil),
		zap.Any("ctx.Err", ctx.Err()),
	)
	count := 0

	select {
	case entry, ok := <-logCh:
		if !ok {
			s.logger.Error("handleLogStream: channel was already closed!")
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
			s.logger.Info("log stream stopped by context", zap.Int("totalEntriesProcessed", count))
			return
		case entry, ok := <-logCh:
			if !ok {
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
	session := s.currentLogSession()
	if session != nil {
		session.cancel()
	}
}

func (s *LogService) currentLogSession() *logSession {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	return s.logSession
}

func (s *LogService) replaceLogSession(next *logSession) *logSession {
	s.logMu.Lock()
	prev := s.logSession
	s.logSession = next
	s.logMu.Unlock()

	// Cancel the previous session immediately; the active session is canceled
	// by StopLogStream or by its parent context.
	if prev != nil {
		prev.cancel()
	}
	return prev
}

func (s *LogService) finishLogSession(session *logSession) {
	s.logMu.Lock()
	if s.logSession == session {
		s.logSession = nil
	}
	s.logMu.Unlock()
	session.markDone()
}
