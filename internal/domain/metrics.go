package domain

import "time"

// RouteStatus labels the outcome of a routed request.
type RouteStatus string

const (
	// RouteStatusSuccess indicates a successful route.
	RouteStatusSuccess RouteStatus = "success"
	// RouteStatusError indicates a failed route.
	RouteStatusError RouteStatus = "error"
)

// RouteReason describes why a routed request ended with a status.
type RouteReason string

const (
	// RouteReasonSuccess indicates the request succeeded.
	RouteReasonSuccess RouteReason = "success"
	// RouteReasonInvalidRequest indicates the request was invalid.
	RouteReasonInvalidRequest RouteReason = "invalid_request"
	// RouteReasonMethodNotAllowed indicates the method is not allowed.
	RouteReasonMethodNotAllowed RouteReason = "method_not_allowed"
	// RouteReasonTimeoutColdStart indicates a cold-start timeout.
	RouteReasonTimeoutColdStart RouteReason = "timeout_cold_start"
	// RouteReasonTimeoutExecution indicates execution timed out.
	RouteReasonTimeoutExecution RouteReason = "timeout_execution"
	// RouteReasonConnClosed indicates the connection closed unexpectedly.
	RouteReasonConnClosed RouteReason = "conn_closed"
	// RouteReasonAcquireFailed indicates instance acquisition failed.
	RouteReasonAcquireFailed RouteReason = "acquire_failed"
	// RouteReasonExecutionFailed indicates tool execution failed.
	RouteReasonExecutionFailed RouteReason = "execution_failed"
	// RouteReasonUnknown indicates an unknown failure.
	RouteReasonUnknown RouteReason = "unknown"
)

// PoolWaitOutcome describes how an acquire wait ended.
type PoolWaitOutcome string

const (
	// PoolWaitOutcomeSignaled indicates the wait was released by a signal.
	PoolWaitOutcomeSignaled PoolWaitOutcome = "signaled"
	// PoolWaitOutcomeCanceled indicates the wait ended by cancellation.
	PoolWaitOutcomeCanceled PoolWaitOutcome = "canceled"
	// PoolWaitOutcomeTimeout indicates the wait ended by deadline.
	PoolWaitOutcomeTimeout PoolWaitOutcome = "timeout"
)

// AcquireFailureReason describes why acquire failed.
type AcquireFailureReason string

const (
	// AcquireFailureNoReady indicates no ready instances were available.
	AcquireFailureNoReady AcquireFailureReason = "no_ready"
	// AcquireFailureNoCapacity indicates no capacity was available.
	AcquireFailureNoCapacity AcquireFailureReason = "no_capacity"
	// AcquireFailureStickyBusy indicates a sticky instance was busy.
	AcquireFailureStickyBusy AcquireFailureReason = "sticky_busy"
)

// ReloadAction describes which reload action triggered the metric.
type ReloadAction string

const (
	// ReloadActionEntry indicates a reload entry point.
	ReloadActionEntry ReloadAction = "entry"
	// ReloadActionServerAdd indicates a server was added.
	ReloadActionServerAdd ReloadAction = "server_add"
	// ReloadActionServerRemove indicates a server was removed.
	ReloadActionServerRemove ReloadAction = "server_remove"
	// ReloadActionServerUpdate indicates a server was updated.
	ReloadActionServerUpdate ReloadAction = "server_update"
	// ReloadActionServerReplace indicates a server was replaced.
	ReloadActionServerReplace ReloadAction = "server_replace"
)

// RouteMetric captures metrics for a routed request.
type RouteMetric struct {
	ServerType string
	Client     string
	Status     RouteStatus
	Reason     RouteReason
	Duration   time.Duration
}

// ReloadApplyResult describes the outcome of a reload apply.
type ReloadApplyResult string

const (
	// ReloadApplyResultSuccess indicates reload apply succeeded.
	ReloadApplyResultSuccess ReloadApplyResult = "success"
	// ReloadApplyResultFailure indicates reload apply failed.
	ReloadApplyResultFailure ReloadApplyResult = "failure"
)

// ReloadApplyMetric captures metrics for reload apply attempts.
type ReloadApplyMetric struct {
	Mode     ReloadMode
	Result   ReloadApplyResult
	Summary  string
	Duration time.Duration
}

// ReloadRollbackResult describes the outcome of a reload rollback.
type ReloadRollbackResult string

const (
	// ReloadRollbackResultSuccess indicates reload rollback succeeded.
	ReloadRollbackResultSuccess ReloadRollbackResult = "success"
	// ReloadRollbackResultFailure indicates reload rollback failed.
	ReloadRollbackResultFailure ReloadRollbackResult = "failure"
)

// ReloadRollbackMetric captures metrics for reload rollback attempts.
type ReloadRollbackMetric struct {
	Mode     ReloadMode
	Result   ReloadRollbackResult
	Summary  string
	Duration time.Duration
}

// Metrics records operational metrics for routing and instances.
type Metrics interface {
	ObserveRoute(metric RouteMetric)
	AddInflightRoutes(serverType string, delta int)
	ObservePoolWait(serverType string, duration time.Duration, outcome PoolWaitOutcome)
	ObserveInstanceStart(serverType string, duration time.Duration, err error)
	ObserveInstanceStartCause(serverType string, reason StartCauseReason)
	ObserveInstanceStop(serverType string, err error)
	SetStartingInstances(serverType string, count int)
	SetActiveInstances(serverType string, count int)
	SetPoolCapacityRatio(serverType string, ratio float64)
	SetPoolWaiters(serverType string, count int)
	ObservePoolAcquireFailure(serverType string, reason AcquireFailureReason)
	ObserveSubAgentTokens(provider string, model string, tokens int)
	ObserveSubAgentLatency(provider string, model string, duration time.Duration)
	ObserveSubAgentFilterPrecision(provider string, model string, ratio float64)
	RecordReloadSuccess(source CatalogUpdateSource, action ReloadAction)
	RecordReloadFailure(source CatalogUpdateSource, action ReloadAction)
	RecordReloadRestart(source CatalogUpdateSource, action ReloadAction)
	ObserveReloadApply(metric ReloadApplyMetric)
	ObserveReloadRollback(metric ReloadRollbackMetric)
}
