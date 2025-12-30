package telemetry

import (
	"time"

	"mcpd/internal/domain"
)

type NoopMetrics struct{}

func NewNoopMetrics() *NoopMetrics {
	return &NoopMetrics{}
}

func (n *NoopMetrics) ObserveRoute(serverType string, duration time.Duration, err error) {}

func (n *NoopMetrics) ObserveInstanceStart(specKey string, duration time.Duration, err error) {}

func (n *NoopMetrics) ObserveInstanceStop(specKey string, err error) {}

func (n *NoopMetrics) SetActiveInstances(specKey string, count int) {}

func (n *NoopMetrics) ObserveSubAgentTokens(provider string, model string, tokens int) {}

func (n *NoopMetrics) ObserveSubAgentLatency(provider string, model string, duration time.Duration) {}

func (n *NoopMetrics) ObserveSubAgentFilterPrecision(provider string, model string, ratio float64) {}

var _ domain.Metrics = (*NoopMetrics)(nil)
