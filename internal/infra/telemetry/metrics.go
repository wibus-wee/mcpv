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

var _ domain.Metrics = (*NoopMetrics)(nil)
