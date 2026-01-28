package telemetry

import (
	"time"

	"mcpd/internal/domain"
)

type NoopMetrics struct{}

func NewNoopMetrics() *NoopMetrics {
	return &NoopMetrics{}
}

func (n *NoopMetrics) ObserveRoute(_ domain.RouteMetric) {}

func (n *NoopMetrics) ObserveInstanceStart(_ string, _ time.Duration, _ error) {}

func (n *NoopMetrics) ObserveInstanceStop(_ string, _ error) {}

func (n *NoopMetrics) SetActiveInstances(_ string, _ int) {}

func (n *NoopMetrics) SetPoolCapacityRatio(_ string, _ float64) {}

func (n *NoopMetrics) ObserveSubAgentTokens(_ string, _ string, _ int) {}

func (n *NoopMetrics) ObserveSubAgentLatency(_ string, _ string, _ time.Duration) {}

func (n *NoopMetrics) ObserveSubAgentFilterPrecision(_ string, _ string, _ float64) {}

var _ domain.Metrics = (*NoopMetrics)(nil)
