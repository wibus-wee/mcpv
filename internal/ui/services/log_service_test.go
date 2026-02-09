package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/ui"
)

func TestLogServiceStartLogStreamWithoutManager(t *testing.T) {
	deps := NewServiceDeps(nil, zap.NewNop())
	service := NewLogService(deps)

	err := service.StartLogStream(context.Background(), "info")
	if err == nil {
		t.Fatal("expected error when manager is nil")
	}
	uiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if uiErr.Code != ui.ErrCodeInternal {
		t.Fatalf("expected error code %s, got %s", ui.ErrCodeInternal, uiErr.Code)
	}
}

func TestLogServiceSendToRingBufferDropsWhenFull(t *testing.T) {
	deps := NewServiceDeps(nil, zap.NewNop())
	service := NewLogService(deps)

	var dropped uint64
	session := &logSession{
		emitCh:     make(chan domain.LogEntry, 1),
		droppedCnt: &dropped,
	}

	first := domain.LogEntry{Logger: "first", Level: domain.LogLevelInfo, Timestamp: time.Now()}
	second := domain.LogEntry{Logger: "second", Level: domain.LogLevelInfo, Timestamp: time.Now()}

	service.sendToRingBuffer(session, first)
	service.sendToRingBuffer(session, second)

	if got := atomic.LoadUint64(&dropped); got != 1 {
		t.Fatalf("expected 1 dropped entry, got %d", got)
	}
	if got := len(session.emitCh); got != 1 {
		t.Fatalf("expected 1 entry in buffer, got %d", got)
	}

	entry := <-session.emitCh
	if entry.Logger != "first" {
		t.Fatalf("expected first entry to be kept, got %s", entry.Logger)
	}
}

func TestLogServiceFinishLogSessionClearsState(t *testing.T) {
	deps := NewServiceDeps(nil, zap.NewNop())
	service := NewLogService(deps)

	session := &logSession{
		done:   make(chan struct{}),
		emitCh: make(chan domain.LogEntry),
	}
	service.logSession = session

	service.finishLogSession(session)

	if service.currentLogSession() != nil {
		t.Fatal("expected current log session to be cleared")
	}
	select {
	case <-session.done:
		// ok
	default:
		t.Fatal("expected session done channel to be closed")
	}
	if _, ok := <-session.emitCh; ok {
		t.Fatal("expected emit channel to be closed")
	}
}

func TestLogServiceStopLogStreamCancelsSession(t *testing.T) {
	deps := NewServiceDeps(nil, zap.NewNop())
	service := NewLogService(deps)

	ctx, cancel := context.WithCancel(context.Background())
	session := &logSession{
		ctx:    ctx,
		cancel: cancel,
	}
	service.logSession = session

	service.StopLogStream()

	select {
	case <-ctx.Done():
		// ok
	default:
		t.Fatal("expected StopLogStream to cancel session context")
	}
}

func TestLogServiceReplaceLogSessionCancelsPrevious(t *testing.T) {
	deps := NewServiceDeps(nil, zap.NewNop())
	service := NewLogService(deps)

	ctx1, cancel1 := context.WithCancel(context.Background())
	session1 := &logSession{ctx: ctx1, cancel: cancel1}
	service.logSession = session1

	ctx2, cancel2 := context.WithCancel(context.Background())
	session2 := &logSession{ctx: ctx2, cancel: cancel2}

	service.replaceLogSession(session2)

	select {
	case <-ctx1.Done():
		// ok
	default:
		t.Fatal("expected previous session to be canceled")
	}
	select {
	case <-ctx2.Done():
		t.Fatal("did not expect new session to be canceled")
	default:
		// ok
	}
}
