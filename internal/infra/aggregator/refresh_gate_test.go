package aggregator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRefreshGate_SerializesAcquire(t *testing.T) {
	gate := NewRefreshGate()
	ctx := context.Background()

	require.NoError(t, gate.Acquire(ctx))

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		done <- gate.Acquire(ctx)
	}()

	<-started
	select {
	case err := <-done:
		t.Fatalf("unexpected acquire: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	gate.Release()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("acquire timeout")
	}
}
