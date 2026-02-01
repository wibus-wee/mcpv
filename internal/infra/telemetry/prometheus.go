package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"mcpv/internal/domain"
)

type PrometheusMetrics struct {
	routeDuration           *prometheus.HistogramVec
	inflightRoutes          *prometheus.GaugeVec
	poolWaitDuration        *prometheus.HistogramVec
	instanceStarts          *prometheus.CounterVec
	instanceStops           *prometheus.CounterVec
	instanceStartDuration   *prometheus.HistogramVec
	instanceStartResults    *prometheus.CounterVec
	instanceStartCauses     *prometheus.CounterVec
	instanceStopResults     *prometheus.CounterVec
	startingInstances       *prometheus.GaugeVec
	activeInstances         *prometheus.GaugeVec
	poolCapacityRatio       *prometheus.GaugeVec
	poolWaiters             *prometheus.GaugeVec
	poolAcquireFailures     *prometheus.CounterVec
	subAgentTokens          *prometheus.CounterVec
	subAgentLatency         *prometheus.HistogramVec
	subAgentFilterPrecision *prometheus.HistogramVec
	reloadSuccesses         *prometheus.CounterVec
	reloadFailures          *prometheus.CounterVec
	reloadRestarts          *prometheus.CounterVec
	reloadApplyTotal        *prometheus.CounterVec
	reloadApplyDuration     *prometheus.HistogramVec
	reloadRollbackTotal     *prometheus.CounterVec
	reloadRollbackDuration  *prometheus.HistogramVec
	governanceOutcome       *prometheus.HistogramVec
	governanceRejections    *prometheus.CounterVec
	pluginLifecycle         *prometheus.CounterVec
	pluginHandshakeDuration *prometheus.HistogramVec
	pluginStatus            *prometheus.GaugeVec
}

func NewPrometheusMetrics(registerer prometheus.Registerer) *PrometheusMetrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}
	factory := promauto.With(registerer)

	return &PrometheusMetrics{
		routeDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpv_route_duration_seconds",
				Help:    "Duration of route requests in seconds",
				Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"server_type", "client", "status", "reason"},
		),
		inflightRoutes: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcpv_inflight_routes",
				Help: "Number of inflight route requests",
			},
			[]string{"server_type"},
		),
		poolWaitDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpv_pool_wait_seconds",
				Help:    "Time spent waiting for pool capacity in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"server_type", "outcome"},
		),
		instanceStarts: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_instance_starts_total",
				Help: "Total number of instance start attempts",
			},
			[]string{"server_type"},
		),
		instanceStops: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_instance_stops_total",
				Help: "Total number of instance stops",
			},
			[]string{"server_type"},
		),
		instanceStartDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpv_instance_start_duration_seconds",
				Help:    "Duration of instance start attempts in seconds",
				Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
			},
			[]string{"server_type", "result"},
		),
		instanceStartResults: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_instance_start_result_total",
				Help: "Total number of instance start results",
			},
			[]string{"server_type", "result"},
		),
		instanceStartCauses: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_instance_start_cause_total",
				Help: "Total number of instance start causes",
			},
			[]string{"server_type", "reason"},
		),
		instanceStopResults: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_instance_stop_result_total",
				Help: "Total number of instance stop results",
			},
			[]string{"server_type", "result"},
		),
		startingInstances: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcpv_instance_starting",
				Help: "Current number of instances starting",
			},
			[]string{"server_type"},
		),
		activeInstances: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcpv_active_instances",
				Help: "Current number of active instances",
			},
			[]string{"server_type"},
		),
		poolCapacityRatio: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcpv_pool_capacity_ratio",
				Help: "Ratio of busy calls to total pool capacity",
			},
			[]string{"server_type"},
		),
		poolWaiters: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcpv_pool_waiters",
				Help: "Number of waiting acquisition requests per pool",
			},
			[]string{"server_type"},
		),
		poolAcquireFailures: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_pool_acquire_fail_total",
				Help: "Total number of pool acquire failures",
			},
			[]string{"server_type", "reason"},
		),
		subAgentTokens: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_subagent_tokens_total",
				Help: "Total number of tokens consumed by SubAgent LLM calls",
			},
			[]string{"provider", "model"},
		),
		subAgentLatency: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpv_subagent_latency_seconds",
				Help:    "Latency of SubAgent LLM calls in seconds",
				Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10, 30},
			},
			[]string{"provider", "model"},
		),
		subAgentFilterPrecision: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpv_subagent_filter_precision",
				Help:    "Ratio of tool selection after deduplication in SubAgent",
				Buckets: []float64{.1, .25, .5, .75, .9, 1},
			},
			[]string{"provider", "model"},
		),
		pluginLifecycle: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_plugin_lifecycle_total",
				Help: "Total number of plugin lifecycle events",
			},
			[]string{"category", "plugin", "outcome"},
		),
		pluginHandshakeDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpv_plugin_handshake_duration_seconds",
				Help:    "Duration of plugin handshakes",
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"category", "plugin", "outcome"},
		),
		pluginStatus: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "mcpv_plugin_status",
				Help: "Plugin running state (1=running, 0=stopped)",
			},
			[]string{"category", "plugin", "state"},
		),
		reloadSuccesses: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_reload_success_total",
				Help: "Total number of successful catalog reload actions",
			},
			[]string{"source", "action"},
		),
		reloadFailures: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_reload_failure_total",
				Help: "Total number of failed catalog reload actions",
			},
			[]string{"source", "action"},
		),
		reloadRestarts: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_reload_restart_total",
				Help: "Total number of catalog reload actions requiring restart",
			},
			[]string{"source", "action"},
		),
		reloadApplyTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_reload_apply_total",
				Help: "Total number of reload apply attempts",
			},
			[]string{"mode", "result", "summary"},
		),
		reloadApplyDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpv_reload_apply_duration_seconds",
				Help:    "Duration of reload apply attempts in seconds",
				Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"mode", "result"},
		),
		reloadRollbackTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_reload_rollback_total",
				Help: "Total number of reload rollback attempts",
			},
			[]string{"mode", "result", "summary"},
		),
		reloadRollbackDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpv_reload_rollback_duration_seconds",
				Help:    "Duration of reload rollback attempts in seconds",
				Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"mode", "result"},
		),
		governanceOutcome: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcpv_governance_call_duration_seconds",
				Help:    "Duration of governance plugin calls",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
			},
			[]string{"category", "plugin", "flow", "outcome"},
		),
		governanceRejections: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcpv_governance_rejections_total",
				Help: "Total number of governance plugin rejections",
			},
			[]string{"category", "plugin", "flow", "code"},
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

func (p *PrometheusMetrics) AddInflightRoutes(serverType string, delta int) {
	p.inflightRoutes.WithLabelValues(serverType).Add(float64(delta))
}

func (p *PrometheusMetrics) ObservePoolWait(serverType string, duration time.Duration, outcome domain.PoolWaitOutcome) {
	p.poolWaitDuration.WithLabelValues(serverType, string(outcome)).Observe(duration.Seconds())
}

func (p *PrometheusMetrics) ObserveInstanceStart(serverType string, duration time.Duration, err error) {
	p.instanceStarts.WithLabelValues(serverType).Inc()
	result := "success"
	if err != nil {
		result = "error"
	}
	p.instanceStartResults.WithLabelValues(serverType, result).Inc()
	p.instanceStartDuration.WithLabelValues(serverType, result).Observe(duration.Seconds())
}

func (p *PrometheusMetrics) ObserveInstanceStartCause(serverType string, reason domain.StartCauseReason) {
	if reason == "" {
		reason = domain.StartCauseReason("unknown")
	}
	p.instanceStartCauses.WithLabelValues(serverType, string(reason)).Inc()
}

func (p *PrometheusMetrics) ObserveInstanceStop(serverType string, err error) {
	p.instanceStops.WithLabelValues(serverType).Inc()
	result := "success"
	if err != nil {
		result = "error"
	}
	p.instanceStopResults.WithLabelValues(serverType, result).Inc()
}

func (p *PrometheusMetrics) SetStartingInstances(serverType string, count int) {
	p.startingInstances.WithLabelValues(serverType).Set(float64(count))
}

func (p *PrometheusMetrics) SetActiveInstances(serverType string, count int) {
	p.activeInstances.WithLabelValues(serverType).Set(float64(count))
}

func (p *PrometheusMetrics) SetPoolCapacityRatio(serverType string, ratio float64) {
	p.poolCapacityRatio.WithLabelValues(serverType).Set(ratio)
}

func (p *PrometheusMetrics) SetPoolWaiters(serverType string, count int) {
	p.poolWaiters.WithLabelValues(serverType).Set(float64(count))
}

func (p *PrometheusMetrics) ObservePoolAcquireFailure(serverType string, reason domain.AcquireFailureReason) {
	p.poolAcquireFailures.WithLabelValues(serverType, string(reason)).Inc()
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

func (p *PrometheusMetrics) RecordReloadSuccess(source domain.CatalogUpdateSource, action domain.ReloadAction) {
	p.reloadSuccesses.WithLabelValues(string(source), string(action)).Inc()
}

func (p *PrometheusMetrics) RecordReloadFailure(source domain.CatalogUpdateSource, action domain.ReloadAction) {
	p.reloadFailures.WithLabelValues(string(source), string(action)).Inc()
}

func (p *PrometheusMetrics) RecordReloadRestart(source domain.CatalogUpdateSource, action domain.ReloadAction) {
	p.reloadRestarts.WithLabelValues(string(source), string(action)).Inc()
}

func (p *PrometheusMetrics) ObserveReloadApply(metric domain.ReloadApplyMetric) {
	mode := string(metric.Mode)
	if mode == "" {
		mode = string(domain.DefaultReloadMode)
	}
	result := string(metric.Result)
	if result == "" {
		result = string(domain.ReloadApplyResultSuccess)
	}
	summary := metric.Summary
	if summary == "" {
		summary = "none"
	}
	p.reloadApplyTotal.WithLabelValues(mode, result, summary).Inc()
	p.reloadApplyDuration.WithLabelValues(mode, result).Observe(metric.Duration.Seconds())
}
func (p *PrometheusMetrics) ObserveReloadRollback(metric domain.ReloadRollbackMetric) {
	mode := string(metric.Mode)
	if mode == "" {
		mode = string(domain.DefaultReloadMode)
	}
	result := string(metric.Result)
	if result == "" {
		result = string(domain.ReloadRollbackResultSuccess)
	}
	summary := metric.Summary
	if summary == "" {
		summary = "none"
	}
	p.reloadRollbackTotal.WithLabelValues(mode, result, summary).Inc()
	p.reloadRollbackDuration.WithLabelValues(mode, result).Observe(metric.Duration.Seconds())
}

func (p *PrometheusMetrics) RecordGovernanceOutcome(metric domain.GovernanceOutcomeMetric) {
	if p.governanceOutcome == nil {
		return
	}
	plugin := metric.Plugin
	if plugin == "" {
		plugin = "unnamed"
	}
	outcome := string(metric.Outcome)
	if outcome == "" {
		outcome = string(domain.GovernanceOutcomeContinue)
	}
	p.governanceOutcome.WithLabelValues(
		string(metric.Category),
		plugin,
		string(metric.Flow),
		outcome,
	).Observe(metric.Duration.Seconds())
}

func (p *PrometheusMetrics) RecordGovernanceRejection(metric domain.GovernanceRejectionMetric) {
	if p.governanceRejections == nil {
		return
	}
	plugin := metric.Plugin
	if plugin == "" {
		plugin = "unnamed"
	}
	code := metric.Code
	if code == "" {
		code = "rejected"
	}
	p.governanceRejections.WithLabelValues(
		string(metric.Category),
		plugin,
		string(metric.Flow),
		code,
	).Inc()
}

func (p *PrometheusMetrics) RecordPluginStart(metric domain.PluginStartMetric) {
	if p.pluginLifecycle == nil || metric.Plugin == "" {
		return
	}
	outcome := "success"
	if !metric.Success {
		outcome = "failure"
	}
	category := string(metric.Category)
	if category == "" {
		category = "unknown"
	}
	p.pluginLifecycle.WithLabelValues(category, metric.Plugin, outcome).Inc()
}

func (p *PrometheusMetrics) RecordPluginHandshake(metric domain.PluginHandshakeMetric) {
	if p.pluginHandshakeDuration == nil || metric.Plugin == "" {
		return
	}
	outcome := "success"
	if !metric.Succeeded {
		outcome = "failure"
	}
	category := string(metric.Category)
	if category == "" {
		category = "unknown"
	}
	duration := metric.Duration.Seconds()
	if duration < 0 {
		duration = 0
	}
	p.pluginHandshakeDuration.WithLabelValues(category, metric.Plugin, outcome).Observe(duration)
}

func (p *PrometheusMetrics) SetPluginRunning(category domain.PluginCategory, name string, running bool) {
	if p.pluginStatus == nil || name == "" {
		return
	}
	cat := string(category)
	if cat == "" {
		cat = "unknown"
	}
	stateRunning := "running"
	stateStopped := "stopped"
	p.pluginStatus.WithLabelValues(cat, name, stateRunning).Set(boolToFloat(running))
	p.pluginStatus.WithLabelValues(cat, name, stateStopped).Set(boolToFloat(!running))
}

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

var _ domain.Metrics = (*PrometheusMetrics)(nil)
