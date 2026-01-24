package telemetry

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mcpd/internal/domain"
)

func TestNewPrometheusMetrics(t *testing.T) {
	m := NewPrometheusMetrics(prometheus.NewRegistry())
	assert.NotNil(t, m)
	assert.NotNil(t, m.routeDuration)
	assert.NotNil(t, m.instanceStarts)
	assert.NotNil(t, m.instanceStops)
	assert.NotNil(t, m.activeInstances)
	assert.NotNil(t, m.poolCapacityRatio)
	assert.NotNil(t, m.subAgentTokens)
	assert.NotNil(t, m.subAgentLatency)
	assert.NotNil(t, m.subAgentFilterPrecision)
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
	m.ObserveInstanceStart("test-server", 0, nil)
	m.ObserveInstanceStop("test-server", nil)
	m.SetActiveInstances("test-server", 1)
	m.SetPoolCapacityRatio("test-server", 0.2)
	m.ObserveSubAgentTokens("openai", "gpt-4o", 128)
	m.ObserveSubAgentLatency("openai", "gpt-4o", 500*time.Millisecond)
	m.ObserveSubAgentFilterPrecision("openai", "gpt-4o", 0.5)

	metrics, err := registry.Gather()
	require.NoError(t, err)

	names := make([]string, 0, len(metrics))
	for _, m := range metrics {
		names = append(names, m.GetName())
	}

	assert.Contains(t, names, "mcpd_route_duration_seconds")
	assert.Contains(t, names, "mcpd_instance_starts_total")
	assert.Contains(t, names, "mcpd_instance_stops_total")
	assert.Contains(t, names, "mcpd_active_instances")
	assert.Contains(t, names, "mcpd_pool_capacity_ratio")
	assert.Contains(t, names, "mcpd_subagent_tokens_total")
	assert.Contains(t, names, "mcpd_subagent_latency_seconds")
	assert.Contains(t, names, "mcpd_subagent_filter_precision")
}

func TestPrometheusMetrics_ImplementsInterface(t *testing.T) {
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
		routeDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route2"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts2"}, []string{"server_type"}),
		instanceStops:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops2"}, []string{"server_type"}),
		activeInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active2"}, []string{"server_type"}),
	}
	assert.NotPanics(t, func() {
		m.ObserveInstanceStart("test-server", 1*time.Second, nil)
		m.ObserveInstanceStart("test-server", 2*time.Second, assert.AnError)
	})
}

func TestPrometheusMetrics_ObserveInstanceStop(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route3"}, []string{"server_type", "client", "status", "reason"}),
		instanceStarts:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts3"}, []string{"server_type"}),
		instanceStops:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops3"}, []string{"server_type"}),
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
