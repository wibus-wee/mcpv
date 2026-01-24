package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"mcpd/internal/domain"
)

type PrometheusMetrics struct {
	routeDuration           *prometheus.HistogramVec
	instanceStarts          *prometheus.CounterVec
	instanceStops           *prometheus.CounterVec
	activeInstances         *prometheus.GaugeVec
	poolCapacityRatio       *prometheus.GaugeVec
	subAgentTokens          *prometheus.CounterVec
	subAgentLatency         *prometheus.HistogramVec
	subAgentFilterPrecision *prometheus.HistogramVec
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
			[]string{"server_type", "client", "status", "reason"},
		),
		instanceStarts: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpd_instance_starts_total",
				Help: "Total number of instance start attempts",
			},
			[]string{"server_type"},
		),
		instanceStops: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpd_instance_stops_total",
				Help: "Total number of instance stops",
			},
			[]string{"server_type"},
		),
		activeInstances: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcpd_active_instances",
				Help: "Current number of active instances",
			},
			[]string{"server_type"},
		),
		poolCapacityRatio: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcpd_pool_capacity_ratio",
				Help: "Ratio of busy calls to total pool capacity",
			},
			[]string{"server_type"},
		),
		subAgentTokens: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpd_subagent_tokens_total",
				Help: "Total number of tokens consumed by SubAgent LLM calls",
			},
			[]string{"provider", "model"},
		),
		subAgentLatency: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpd_subagent_latency_seconds",
				Help:    "Latency of SubAgent LLM calls in seconds",
				Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10, 30},
			},
			[]string{"provider", "model"},
		),
		subAgentFilterPrecision: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpd_subagent_filter_precision",
				Help:    "Ratio of tool selection after deduplication in SubAgent",
				Buckets: []float64{.1, .25, .5, .75, .9, 1},
			},
			[]string{"provider", "model"},
		),
	}
}

func (p *PrometheusMetrics) ObserveRoute(metric domain.RouteMetric) {
	p.routeDuration.WithLabelValues(
		metric.ServerType,
		metric.Client,
		string(metric.Status),
		string(metric.Reason),
	).Observe(metric.Duration.Seconds())
}

func (p *PrometheusMetrics) ObserveInstanceStart(serverType string, duration time.Duration, err error) {
	p.instanceStarts.WithLabelValues(serverType).Inc()
}

func (p *PrometheusMetrics) ObserveInstanceStop(serverType string, err error) {
	p.instanceStops.WithLabelValues(serverType).Inc()
}

func (p *PrometheusMetrics) SetActiveInstances(serverType string, count int) {
	p.activeInstances.WithLabelValues(serverType).Set(float64(count))
}

func (p *PrometheusMetrics) SetPoolCapacityRatio(serverType string, ratio float64) {
	p.poolCapacityRatio.WithLabelValues(serverType).Set(ratio)
}

func (p *PrometheusMetrics) ObserveSubAgentTokens(provider string, model string, tokens int) {
	p.subAgentTokens.WithLabelValues(provider, model).Add(float64(tokens))
}

func (p *PrometheusMetrics) ObserveSubAgentLatency(provider string, model string, duration time.Duration) {
	p.subAgentLatency.WithLabelValues(provider, model).Observe(duration.Seconds())
}

func (p *PrometheusMetrics) ObserveSubAgentFilterPrecision(provider string, model string, ratio float64) {
	p.subAgentFilterPrecision.WithLabelValues(provider, model).Observe(ratio)
}

var _ domain.Metrics = (*PrometheusMetrics)(nil)
