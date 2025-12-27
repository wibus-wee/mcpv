package telemetry

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"mcpd/internal/domain"
)

func TestNewPrometheusMetrics(t *testing.T) {
	m := NewPrometheusMetrics()
	assert.NotNil(t, m)
	assert.NotNil(t, m.routeDuration)
	assert.NotNil(t, m.instanceStarts)
	assert.NotNil(t, m.instanceStops)
	assert.NotNil(t, m.activeInstances)
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
			[]string{"server_type", "status"},
		),
		instanceStarts:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts"}, []string{"spec_key"}),
		instanceStops:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops"}, []string{"spec_key"}),
		activeInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active"}, []string{"spec_key"}),
	}
	registry.MustRegister(m.routeDuration)

	tests := []struct {
		name       string
		serverType string
		duration   time.Duration
		err        error
	}{
		{"success", "test-server", 100 * time.Millisecond, nil},
		{"error", "test-server", 50 * time.Millisecond, assert.AnError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				m.ObserveRoute(tt.serverType, tt.duration, tt.err)
			})
		})
	}
}

func TestPrometheusMetrics_ObserveInstanceStart(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route2"}, []string{"server_type", "status"}),
		instanceStarts:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts2"}, []string{"spec_key"}),
		instanceStops:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops2"}, []string{"spec_key"}),
		activeInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active2"}, []string{"spec_key"}),
	}
	assert.NotPanics(t, func() {
		m.ObserveInstanceStart("test-server", 1*time.Second, nil)
		m.ObserveInstanceStart("test-server", 2*time.Second, assert.AnError)
	})
}

func TestPrometheusMetrics_ObserveInstanceStop(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route3"}, []string{"server_type", "status"}),
		instanceStarts:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts3"}, []string{"spec_key"}),
		instanceStops:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops3"}, []string{"spec_key"}),
		activeInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active3"}, []string{"spec_key"}),
	}
	assert.NotPanics(t, func() {
		m.ObserveInstanceStop("test-server", nil)
		m.ObserveInstanceStop("test-server", assert.AnError)
	})
}

func TestPrometheusMetrics_SetActiveInstances(t *testing.T) {
	m := &PrometheusMetrics{
		routeDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_route4"}, []string{"server_type", "status"}),
		instanceStarts:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_starts4"}, []string{"spec_key"}),
		instanceStops:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_stops4"}, []string{"spec_key"}),
		activeInstances: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_active4"}, []string{"spec_key"}),
	}
	assert.NotPanics(t, func() {
		m.SetActiveInstances("test-server", 0)
		m.SetActiveInstances("test-server", 5)
		m.SetActiveInstances("test-server", 10)
	})
}
