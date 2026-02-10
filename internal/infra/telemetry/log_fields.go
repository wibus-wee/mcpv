package telemetry

import (
	"time"

	"go.uber.org/zap"
)

const (
	FieldEvent      = "event"
	FieldServerType = "serverType"
	FieldInstanceID = "instanceID"
	FieldState      = "state"
	FieldDurationMs = "duration_ms"
	FieldLogSource  = "log_source"
	FieldLogStream  = "stream"
	FieldRequestID  = "request_id"
	FieldTraceID    = "trace_id"
	FieldSpanID     = "span_id"
)

const (
	EventStartAttempt      = "start_attempt"
	EventStartSuccess      = "start_success"
	EventStartFailure      = "start_failure"
	EventInitializeFailure = "initialize_failure"
	EventPingFailure       = "ping_failure"
	EventRouteError        = "route_error"
	EventIdleReap          = "idle_reap"
	EventStopSuccess       = "stop_success"
	EventStopFailure       = "stop_failure"
)

const (
	LogSourceCore       = "core"
	LogSourceDownstream = "downstream"
	LogSourceUI         = "ui"
)

func EventField(event string) zap.Field {
	return zap.String(FieldEvent, event)
}

func ServerTypeField(serverType string) zap.Field {
	return zap.String(FieldServerType, serverType)
}

func InstanceIDField(instanceID string) zap.Field {
	return zap.String(FieldInstanceID, instanceID)
}

func StateField(state string) zap.Field {
	return zap.String(FieldState, state)
}

func DurationField(duration time.Duration) zap.Field {
	return zap.Int64(FieldDurationMs, duration.Milliseconds())
}

func RequestIDField(value string) zap.Field {
	return zap.String(FieldRequestID, value)
}

func TraceIDField(value string) zap.Field {
	return zap.String(FieldTraceID, value)
}

func SpanIDField(value string) zap.Field {
	return zap.String(FieldSpanID, value)
}
