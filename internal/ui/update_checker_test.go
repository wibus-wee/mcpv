package ui

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/ui/types"
)

func TestUpdateCheckerStopWithTimeout_NotStarted(t *testing.T) {
	checker := NewUpdateChecker(zap.NewNop(), types.UpdateCheckOptions{})

	err := checker.StopWithTimeout(10 * time.Millisecond)

	require.NoError(t, err)
}

func TestUpdateCheckerStopWithTimeout_Success(t *testing.T) {
	checker := NewUpdateChecker(zap.NewNop(), types.UpdateCheckOptions{})
	done := make(chan struct{})
	checker.mu.Lock()
	checker.ticker = time.NewTicker(time.Hour)
	checker.stop = make(chan struct{})
	checker.done = done
	checker.mu.Unlock()

	go func() {
		time.Sleep(10 * time.Millisecond)
		close(done)
	}()

	err := checker.StopWithTimeout(200 * time.Millisecond)

	require.NoError(t, err)
}

func TestUpdateCheckerStopWithTimeout_Timeout(t *testing.T) {
	checker := NewUpdateChecker(zap.NewNop(), types.UpdateCheckOptions{})
	done := make(chan struct{})
	checker.mu.Lock()
	checker.ticker = time.NewTicker(time.Hour)
	checker.stop = make(chan struct{})
	checker.done = done
	checker.mu.Unlock()

	err := checker.StopWithTimeout(10 * time.Millisecond)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	close(done)
}
