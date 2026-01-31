package telemetry

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcpv/internal/domain"
)

func TestNewPrometheusMetrics(t *testing.T) {
	m := NewPrometheusMetrics(prometheus.NewRegistry())
	assert.NotNil(t, m)
	assert.NotNil(t, m.routeDuration)
	assert.NotNil(t, m.inflightRoutes)
	assert.NotNil(t, m.poolWaitDuration)
	assert.NotNil(t, m.instanceStarts)
	assert.NotNil(t, m.instanceStops)
	assert.NotNil(t, m.instanceStartDuration)
	assert.NotNil(t, m.instanceStartResults)
	assert.NotNil(t, m.instanceStartCauses)
	assert.NotNil(t, m.instanceStopResults)
	assert.NotNil(t, m.startingInstances)
	assert.NotNil(t, m.activeInstances)
	assert.NotNil(t, m.poolCapacityRatio)
	assert.NotNil(t, m.poolWaiters)
	assert.NotNil(t, m.poolAcquireFailures)
	assert.NotNil(t, m.subAgentTokens)
	assert.NotNil(t, m.subAgentLatency)
	assert.NotNil(t, m.subAgentFilterPrecision)
	assert.NotNil(t, m.reloadSuccesses)
	assert.NotNil(t, m.reloadFailures)
	assert.NotNil(t, m.reloadRestarts)
	assert.NotNil(t, m.reloadRollbackTotal)
	assert.NotNil(t, m.reloadRollbackDuration)
}

func TestNewPrometheusMetrics_UsesProvidedRegistry(t *testing.T) {
	registry := prometheus.NewRegistry()

	m := NewPrometheusMetrics(registry)
	m.ObserveRoute(domain.RouteMetric{
		ServerType: "test-server",
		Client:     "client",
		Status:     domain.RouteStatusSuccess,
		Reason:     domain.RouteReasonSuccess,
		Duration:   10 * time.Millisecond,
	})
	m.AddInflightRoutes("test-server", 1)
	m.AddInflightRoutes("test-server", -1)
	m.ObservePoolWait("test-server", 50*time.Millisecond, domain.PoolWaitOutcomeSignaled)
	m.ObserveInstanceStart("test-server", 0, nil)
	m.ObserveInstanceStartCause("test-server", domain.StartCauseToolCall)
	m.ObserveInstanceStop("test-server", nil)
	m.SetStartingInstances("test-server", 2)
	m.SetActiveInstances("test-server", 1)
	m.SetPoolCapacityRatio("test-server", 0.2)
	m.SetPoolWaiters("test-server", 3)
	m.ObservePoolAcquireFailure("test-server", domain.AcquireFailureNoCapacity)
	m.ObserveSubAgentTokens("openai", "gpt-4o", 128)
	m.ObserveSubAgentLatency("openai", "gpt-4o", 500*time.Millisecond)
	m.ObserveSubAgentFilterPrecision("openai", "gpt-4o", 0.5)
	m.RecordReloadSuccess(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
	m.RecordReloadFailure(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
	m.RecordReloadRestart(domain.CatalogUpdateSourceManual, domain.ReloadActionEntry)
	m.ObserveReloadApply(domain.ReloadApplyMetric{
		Mode:     domain.ReloadModeLenient,
		Result:   domain.ReloadApplyResultSuccess,
		Summary:  "ok",
		Duration: 10 * time.Millisecond,
	})
	m.ObserveReloadRollback(domain.ReloadRollbackMetric{
		Mode:     domain.ReloadModeLenient,
		Result:   domain.ReloadRollbackResultSuccess,
		Summary:  "ok",
		Duration: 10 * time.Millisecond,
	})

	metrics, err := registry.Gather()
	require.NoError(t, err)

	names := make([]string, 0, len(metrics))
	for _, m := range metrics {
		names = append(names, m.GetName())
	}

	assert.Contains(t, names, "mcpv_route_duration_seconds")
	assert.Contains(t, names, "mcpv_inflight_routes")
	assert.Contains(t, names, "mcpv_pool_wait_seconds")
	assert.Contains(t, names, "mcpv_instance_starts_total")
	assert.Contains(t, names, "mcpv_instance_stops_total")
	assert.Contains(t, names, "mcpv_instance_start_duration_seconds")
	assert.Contains(t, names, "mcpv_instance_start_result_total")
	assert.Contains(t, names, "mcpv_instance_start_cause_total")
	assert.Contains(t, names, "mcpv_instance_stop_result_total")
	assert.Contains(t, names, "mcpv_instance_starting")
	assert.Contains(t, names, "mcpv_active_instances")
	assert.Contains(t, names, "mcpv_pool_capacity_ratio")
	assert.Contains(t, names, "mcpv_pool_waiters")
	assert.Contains(t, names, "mcpv_pool_acquire_fail_total")
	assert.Contains(t, names, "mcpv_subagent_tokens_total")
	assert.Contains(t, names, "mcpv_subagent_latency_seconds")
	assert.Contains(t, names, "mcpv_subagent_filter_precision")
	assert.Contains(t, names, "mcpv_reload_success_total")
	assert.Contains(t, names, "mcpv_reload_failure_total")
	assert.Contains(t, names, "mcpv_reload_restart_total")
	assert.Contains(t, names, "mcpv_reload_rollback_total")
	assert.Contains(t, names, "mcpv_reload_rollback_duration_seconds")
}

func TestPrometheusMetrics_ImplementsInterface(t *testing.T) {
	t.Helper()
	var _ domain.Metrics = (*PrometheusMetrics)(nil)
}

func TestPrometheusMetrics_ObserveRoute(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := &PrometheusMetrics{
		routeDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "test_route_duration_seconds",
				Help:    "Test route duration",
				Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"server_type", "client", "status", "reason"},
		),
		instanceStarts:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts"}, []string{"server_type"}),
		instanceStops:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops"}, []string{"server_type"}),
		activeInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active"}, []string{"server_type"}),
	}
	registry.MustRegister(m.routeDuration)

	tests := []struct {
		name   string
		metric domain.RouteMetric
	}{
		{
			name: "success",
			metric: domain.RouteMetric{
				ServerType: "test-server",
				Client:     "client",
				Status:     domain.RouteStatusSuccess,
				Reason:     domain.RouteReasonSuccess,
				Duration:   100 * time.Millisecond,
			},
		},
		{
			name: "error",
			metric: domain.RouteMetric{
				ServerType: "test-server",
				Client:     "client",
				Status:     domain.RouteStatusError,
				Reason:     domain.RouteReasonUnknown,
				Duration:   50 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				m.ObserveRoute(tt.metric)
			})
		})
	}
}

func TestPrometheusMetrics_ObserveInstanceStart(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route2"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts2"}, []string{"server_type"}),
		instanceStops:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops2"}, []string{"server_type"}),
		instanceStartDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: "test_start_duration"},
			[]string{"server_type", "result"},
		),
		instanceStartResults: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_start_results"},
			[]string{"server_type", "result"},
		),
		instanceStopResults: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_stop_results"},
			[]string{"server_type", "result"},
		),
		activeInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active2"}, []string{"server_type"}),
	}
	assert.NotPanics(t, func() {
		m.ObserveInstanceStart("test-server", 1*time.Second, nil)
		m.ObserveInstanceStart("test-server", 2*time.Second, assert.AnError)
	})
}

func TestPrometheusMetrics_ObserveInstanceStop(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route3"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts3"}, []string{"server_type"}),
		instanceStops:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops3"}, []string{"server_type"}),
		instanceStartDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: "test_start_duration3"},
			[]string{"server_type", "result"},
		),
		instanceStartResults: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_start_results3"},
			[]string{"server_type", "result"},
		),
		instanceStopResults: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_stop_results3"},
			[]string{"server_type", "result"},
		),
		activeInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active3"}, []string{"server_type"}),
	}
	assert.NotPanics(t, func() {
		m.ObserveInstanceStop("test-server", nil)
		m.ObserveInstanceStop("test-server", assert.AnError)
	})
}

func TestPrometheusMetrics_SetActiveInstances(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route4"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts4"}, []string{"server_type"}),
		instanceStops:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops4"}, []string{"server_type"}),
		activeInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active4"}, []string{"server_type"}),
	}
	assert.NotPanics(t, func() {
		m.SetActiveInstances("test-server", 0)
		m.SetActiveInstances("test-server", 5)
		m.SetActiveInstances("test-server", 10)
	})
}

func TestPrometheusMetrics_SetPoolCapacityRatio(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:     prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route5"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts5"}, []string{"server_type"}),
		instanceStops:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops5"}, []string{"server_type"}),
		activeInstances:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active5"}, []string{"server_type"}),
		poolCapacityRatio: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_capacity"}, []string{"server_type"}),
	}
	assert.NotPanics(t, func() {
		m.SetPoolCapacityRatio("test-server", 0)
		m.SetPoolCapacityRatio("test-server", 0.5)
		m.SetPoolCapacityRatio("test-server", 1)
	})
}

func TestPrometheusMetrics_AddInflightRoutes(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route6"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts6"}, []string{"server_type"}),
		instanceStops:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops6"}, []string{"server_type"}),
		inflightRoutes: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_inflight"}, []string{"server_type"}),
	}
	assert.NotPanics(t, func() {
		m.AddInflightRoutes("test-server", 1)
		m.AddInflightRoutes("test-server", 2)
		m.AddInflightRoutes("test-server", -1)
	})
}

func TestPrometheusMetrics_SetPoolWaiters(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route7"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts7"}, []string{"server_type"}),
		instanceStops:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops7"}, []string{"server_type"}),
		poolWaiters:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_waiters"}, []string{"server_type"}),
	}
	assert.NotPanics(t, func() {
		m.SetPoolWaiters("test-server", 0)
		m.SetPoolWaiters("test-server", 2)
		m.SetPoolWaiters("test-server", 5)
	})
}

func TestPrometheusMetrics_ObservePoolWait(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route8"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts8"}, []string{"server_type"}),
		instanceStops:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops8"}, []string{"server_type"}),
		poolWaitDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_wait"}, []string{"server_type", "outcome"}),
	}
	assert.NotPanics(t, func() {
		m.ObservePoolWait("test-server", 200*time.Millisecond, domain.PoolWaitOutcomeSignaled)
		m.ObservePoolWait("test-server", 0, domain.PoolWaitOutcomeCanceled)
	})
}

func TestPrometheusMetrics_ObserveInstanceStartCause(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:       prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route9"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts9"}, []string{"server_type"}),
		instanceStops:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops9"}, []string{"server_type"}),
		instanceStartCauses: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_causes"}, []string{"server_type", "reason"}),
	}
	assert.NotPanics(t, func() {
		m.ObserveInstanceStartCause("test-server", domain.StartCauseToolCall)
		m.ObserveInstanceStartCause("test-server", "")
	})
}

func TestPrometheusMetrics_SetStartingInstances(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:     prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route10"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts10"}, []string{"server_type"}),
		instanceStops:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops10"}, []string{"server_type"}),
		startingInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_starting"}, []string{"server_type"}),
	}
	assert.NotPanics(t, func() {
		m.SetStartingInstances("test-server", 0)
		m.SetStartingInstances("test-server", 3)
	})
}

func TestPrometheusMetrics_ObservePoolAcquireFailure(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:       prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route11"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts11"}, []string{"server_type"}),
		instanceStops:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops11"}, []string{"server_type"}),
		poolAcquireFailures: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_fail"}, []string{"server_type", "reason"}),
	}
	assert.NotPanics(t, func() {
		m.ObservePoolAcquireFailure("test-server", domain.AcquireFailureNoReady)
		m.ObservePoolAcquireFailure("test-server", domain.AcquireFailureNoCapacity)
		m.ObservePoolAcquireFailure("test-server", domain.AcquireFailureStickyBusy)
	})
}
