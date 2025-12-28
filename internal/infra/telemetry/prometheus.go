package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"mcpd/internal/domain"
)

type PrometheusMetrics struct {
	routeDuration   *prometheus.HistogramVec
	instanceStarts  *prometheus.CounterVec
	instanceStops   *prometheus.CounterVec
	activeInstances *prometheus.GaugeVec
}

func NewPrometheusMetrics(registerer prometheus.Registerer) *PrometheusMetrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}
	factory := promauto.With(registerer)

	return &PrometheusMetrics{
		routeDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpd_route_duration_seconds",
				Help:    "Duration of route requests in seconds",
				Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"server_type", "status"},
		),
		instanceStarts: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpd_instance_starts_total",
				Help: "Total number of instance start attempts",
			},
			[]string{"spec_key"},
		),
		instanceStops: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpd_instance_stops_total",
				Help: "Total number of instance stops",
			},
			[]string{"spec_key"},
		),
		activeInstances: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcpd_active_instances",
				Help: "Current number of active instances",
			},
			[]string{"spec_key"},
		),
	}
}

func (p *PrometheusMetrics) ObserveRoute(serverType string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	p.routeDuration.WithLabelValues(serverType, status).Observe(duration.Seconds())
}

func (p *PrometheusMetrics) ObserveInstanceStart(specKey string, duration time.Duration, err error) {
	p.instanceStarts.WithLabelValues(specKey).Inc()
}

func (p *PrometheusMetrics) ObserveInstanceStop(specKey string, err error) {
	p.instanceStops.WithLabelValues(specKey).Inc()
}

func (p *PrometheusMetrics) SetActiveInstances(specKey string, count int) {
	p.activeInstances.WithLabelValues(specKey).Set(float64(count))
}

var _ domain.Metrics = (*PrometheusMetrics)(nil)
