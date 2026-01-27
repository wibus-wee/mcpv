package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestStartMetricsServer_Success(t *testing.T) {
	// Use random port to avoid conflicts
	listener := mustListen(t)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- StartMetricsServer(ctx, port, zap.NewNop())
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test /metrics endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "# HELP")

	// Trigger graceful shutdown
	cancel()

	// Wait for server to stop
	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop in time")
	}
}

func TestStartMetricsServer_PortInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Skipf("skip test due to listen error: %v", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Try to start metrics server on the same port (should fail quickly)
	err := StartMetricsServer(ctx, port, zap.NewNop())
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "address already in use"))
}

func TestStartMetricsServer_GracefulShutdown(t *testing.T) {
	listener := mustListen(t)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		err := StartMetricsServer(ctx, port, zap.NewNop())
		assert.NoError(t, err)
		close(done)
	}()

	waitForHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/metrics", port), http.StatusOK, false)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for graceful shutdown
	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("graceful shutdown timed out")
	}
}

func TestStartHTTPServer_Healthz(t *testing.T) {
	listener := mustListen(t)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tracker := NewHealthTracker()
	beat := tracker.Register("test-loop", 200*time.Millisecond)

	errChan := make(chan error, 1)
	go func() {
		errChan <- StartHTTPServer(ctx, HTTPServerOptions{
			Addr:          fmt.Sprintf("127.0.0.1:%d", port),
			EnableHealthz: true,
			Health:        tracker,
		}, zap.NewNop())
	}()

	beat.Beat()
	waitForHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/healthz", port), http.StatusOK, true)
	waitForHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/healthz", port), http.StatusServiceUnavailable, true)

	cancel()

	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop in time")
	}
}

func mustListen(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skip test due to listen error: %v", err)
	}
	return listener
}

func waitForHTTPStatus(t *testing.T, url string, status int, expectJSON bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != status {
			return false
		}
		if expectJSON {
			var report HealthReport
			if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
				return false
			}
			if status == http.StatusOK && report.Status != "ok" {
				return false
			}
		}
		return true
	}, 2*time.Second, 25*time.Millisecond)
}
