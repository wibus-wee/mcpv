package telemetry

import (
	"context"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

type ObservabilityControllerOptions struct {
	DefaultMetricsEnabled bool
	DefaultHealthzEnabled bool
	Registry              prometheus.Gatherer
	Health                *HealthTracker
	Logger                *zap.Logger
}

type ObservabilityController struct {
	mu       sync.Mutex
	defaults ObservabilityControllerOptions
	current  observabilityState
	cancel   context.CancelFunc
	runID    uint64
}

type observabilityState struct {
	addr           string
	metricsEnabled bool
	healthzEnabled bool
}

func (s observabilityState) enabled() bool {
	return s.metricsEnabled || s.healthzEnabled
}

func (s observabilityState) equal(other observabilityState) bool {
	return s.addr == other.addr &&
		s.metricsEnabled == other.metricsEnabled &&
		s.healthzEnabled == other.healthzEnabled
}

func NewObservabilityController(opts ObservabilityControllerOptions) *ObservabilityController {
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &ObservabilityController{
		defaults: opts,
	}
}

func (c *ObservabilityController) Apply(ctx context.Context, cfg domain.ObservabilityConfig) error {
	if c == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	state := resolveObservabilityState(c.defaults, cfg)

	c.mu.Lock()
	defer c.mu.Unlock()

	if !state.enabled() {
		if c.cancel != nil {
			c.cancel()
			c.cancel = nil
		}
		c.current = state
		return nil
	}

	if c.current.equal(state) && c.cancel != nil {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.current = state
	c.runID++
	runID := c.runID

	c.defaults.Logger.Info("starting observability server", zap.String("addr", state.addr))
	go func() {
		err := StartHTTPServer(runCtx, HTTPServerOptions{
			Addr:          state.addr,
			EnableMetrics: state.metricsEnabled,
			EnableHealthz: state.healthzEnabled,
			Health:        c.defaults.Health,
			Registry:      c.defaults.Registry,
		}, c.defaults.Logger)
		if err != nil {
			c.defaults.Logger.Error("observability server failed", zap.Error(err))
		}
		c.mu.Lock()
		if c.runID == runID {
			c.cancel = nil
		}
		c.mu.Unlock()
	}()

	return nil
}

func resolveObservabilityState(defaults ObservabilityControllerOptions, cfg domain.ObservabilityConfig) observabilityState {
	addr := strings.TrimSpace(cfg.ListenAddress)
	if addr == "" {
		addr = domain.DefaultObservabilityListenAddress
	}
	metricsEnabled := defaults.DefaultMetricsEnabled
	if cfg.MetricsEnabled != nil {
		metricsEnabled = *cfg.MetricsEnabled
	}
	healthzEnabled := defaults.DefaultHealthzEnabled
	if cfg.HealthzEnabled != nil {
		healthzEnabled = *cfg.HealthzEnabled
	}
	return observabilityState{
		addr:           addr,
		metricsEnabled: metricsEnabled,
		healthzEnabled: healthzEnabled,
	}
}
