package services

import (
	"context"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/ui"
	"mcpv/internal/ui/events"
)

const (
	// logRingBufferSize is the size of the async ring buffer for log events.
	// This prevents UI event emission blocking the log stream.
	logRingBufferSize = 512
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
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	doneOnce   sync.Once
	minLevel   domain.LogLevel
	emitCh     chan domain.LogEntry // async ring buffer
	emitDone   chan struct{}
	emitWg     sync.WaitGroup
	droppedCnt *uint64 // atomic counter for dropped events
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
		return ui.NewError(ui.ErrCodeInternal, "Manager not initialized")
	}

	cp, err := manager.GetControlPlane()
	if err != nil {
		s.logger.Error("StartLogStream failed: GetControlPlane error", zap.Error(err))
		return err
	}

	var baseCtx context.Context
	switch ctx {
	case nil:
		s.logger.Warn("StartLogStream received nil context, falling back to Background")
		baseCtx = context.Background()
	default:
		if ctx.Err() != nil {
			s.logger.Warn("StartLogStream received canceled context, detaching cancelation", zap.Error(ctx.Err()))
		}
		baseCtx = context.WithoutCancel(ctx)
	}

	level := domain.LogLevel(minLevel)
	s.logger.Info("Calling ControlPlane.StreamLogsAllServers", zap.String("level", string(level)))

	streamCtx, cancel := context.WithCancel(baseCtx)
	var droppedCnt uint64
	session := &logSession{
		ctx:        streamCtx,
		cancel:     cancel,
		done:       make(chan struct{}),
		minLevel:   level,
		emitCh:     make(chan domain.LogEntry, logRingBufferSize),
		emitDone:   make(chan struct{}),
		droppedCnt: &droppedCnt,
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
		return ui.MapDomainError(err)
	}

	s.logger.Info("StreamLogs channel created successfully")

	// Start async emitter to prevent Wails event blocking
	session.emitWg.Add(1)
	go s.asyncEmitter(session)

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
		s.sendToRingBuffer(session, entry)
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
			s.sendToRingBuffer(session, entry)
			if count == 1 {
				s.logger.Info("First log entry sent to async emitter")
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

	// Close emit channel and wait for emitter to finish
	close(session.emitCh)
	session.emitWg.Wait()
	session.markDone()
}

// sendToRingBuffer sends a log entry to the ring buffer using non-blocking send.
// If the buffer is full, it drops the oldest entry (ring buffer semantics).
func (s *LogService) sendToRingBuffer(session *logSession, entry domain.LogEntry) {
	select {
	case session.emitCh <- entry:
		// Successfully sent
	default:
		// Buffer full, drop and count
		atomic.AddUint64(session.droppedCnt, 1)
		s.logger.Warn("log event ring buffer full, dropping entry",
			zap.String("logger", entry.Logger),
		)
	}
}

// asyncEmitter runs in a goroutine and emits log events to Wails asynchronously.
// This prevents slow UI event processing from blocking the log stream.
func (s *LogService) asyncEmitter(session *logSession) {
	defer session.emitWg.Done()

	for entry := range session.emitCh {
		events.EmitLogEntry(s.deps.wailsApp(), entry)
	}

	// Report dropped events on shutdown if any
	dropped := atomic.LoadUint64(session.droppedCnt)
	if dropped > 0 {
		s.logger.Warn("log stream ended with dropped events",
			zap.Uint64("droppedCount", dropped),
		)
	}
}
