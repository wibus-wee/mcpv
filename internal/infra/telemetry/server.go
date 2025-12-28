package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type HTTPServerOptions struct {
	Addr          string
	EnableMetrics bool
	EnableHealthz bool
	Health        *HealthTracker
	Registry      prometheus.Gatherer
}

func StartHTTPServer(ctx context.Context, opts HTTPServerOptions, logger *zap.Logger) error {
	if logger == nil {
		logger = zap.NewNop()
	}
	if !opts.EnableMetrics && !opts.EnableHealthz {
		return nil
	}

	addr := opts.Addr
	if addr == "" {
		addr = "0.0.0.0:9090"
	}

	registry := opts.Registry
	if registry == nil {
		registry = prometheus.DefaultGatherer
	}

	mux := http.NewServeMux()
	if opts.EnableMetrics {
		mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	}
	if opts.EnableHealthz {
		mux.Handle("/healthz", healthHandler(opts.Health))
	}

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	errChan := make(chan error, 1)
	go func() {
		logger.Info("observability server listening",
			zap.String("addr", server.Addr),
			zap.Bool("metrics", opts.EnableMetrics),
			zap.Bool("healthz", opts.EnableHealthz),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return fmt.Errorf("observability server failed to start: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("observability server shutdown error", zap.Error(err))
			return err
		}
		logger.Info("observability server stopped")
		return nil
	}
}

func StartMetricsServer(ctx context.Context, port int, logger *zap.Logger) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	return StartHTTPServer(ctx, HTTPServerOptions{
		Addr:          addr,
		EnableMetrics: true,
	}, logger)
}

func healthHandler(tracker *HealthTracker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		report := HealthReport{Status: "ok"}
		if tracker != nil {
			report = tracker.Report()
		}

		status := http.StatusOK
		if report.Status != "ok" {
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(report)
	})
}
