package telemetry

import (
	"time"

	"mcpv/internal/domain"
)

type NoopMetrics struct{}

func NewNoopMetrics() *NoopMetrics {
	return &NoopMetrics{}
}

func (n *NoopMetrics) ObserveRoute(_ domain.RouteMetric) {}

func (n *NoopMetrics) AddInflightRoutes(_ string, _ int) {}

func (n *NoopMetrics) ObservePoolWait(_ string, _ time.Duration, _ domain.PoolWaitOutcome) {}

func (n *NoopMetrics) ObserveInstanceStart(_ string, _ time.Duration, _ error) {}

func (n *NoopMetrics) ObserveInstanceStartCause(_ string, _ domain.StartCauseReason) {}

func (n *NoopMetrics) ObserveInstanceStop(_ string, _ error) {}

func (n *NoopMetrics) SetStartingInstances(_ string, _ int) {}

func (n *NoopMetrics) SetActiveInstances(_ string, _ int) {}

func (n *NoopMetrics) SetPoolCapacityRatio(_ string, _ float64) {}

func (n *NoopMetrics) SetPoolWaiters(_ string, _ int) {}

func (n *NoopMetrics) ObservePoolAcquireFailure(_ string, _ domain.AcquireFailureReason) {}

func (n *NoopMetrics) ObserveSubAgentTokens(_ string, _ string, _ int) {}

func (n *NoopMetrics) ObserveSubAgentLatency(_ string, _ string, _ time.Duration) {}

func (n *NoopMetrics) ObserveSubAgentFilterPrecision(_ string, _ string, _ float64) {}

func (n *NoopMetrics) RecordReloadSuccess(_ domain.CatalogUpdateSource, _ domain.ReloadAction) {}

func (n *NoopMetrics) RecordReloadFailure(_ domain.CatalogUpdateSource, _ domain.ReloadAction) {}

func (n *NoopMetrics) RecordReloadRestart(_ domain.CatalogUpdateSource, _ domain.ReloadAction) {}
func (n *NoopMetrics) ObserveReloadApply(_ domain.ReloadApplyMetric)                           {}
func (n *NoopMetrics) ObserveReloadRollback(_ domain.ReloadRollbackMetric)                     {}

var _ domain.Metrics = (*NoopMetrics)(nil)
