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

// RouteMetric captures metrics for a routed request.
type RouteMetric struct {
	ServerType string
	Client     string
	Status     RouteStatus
	Reason     RouteReason
	Duration   time.Duration
}

// Metrics records operational metrics for routing and instances.
type Metrics interface {
	ObserveRoute(metric RouteMetric)
	ObserveInstanceStart(serverType string, duration time.Duration, err error)
	ObserveInstanceStop(serverType string, err error)
	SetActiveInstances(serverType string, count int)
	SetPoolCapacityRatio(serverType string, ratio float64)
	ObserveSubAgentTokens(provider string, model string, tokens int)
	ObserveSubAgentLatency(provider string, model string, duration time.Duration)
	ObserveSubAgentFilterPrecision(provider string, model string, ratio float64)
}
