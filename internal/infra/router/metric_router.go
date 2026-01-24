package router

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"mcpd/internal/domain"
)

type MetricRouter struct {
	inner   domain.Router
	metrics domain.Metrics
}

func NewMetricRouter(inner domain.Router, metrics domain.Metrics) *MetricRouter {
	return &MetricRouter{
		inner:   inner,
		metrics: metrics,
	}
}

func (r *MetricRouter) Route(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage) (json.RawMessage, error) {
	return r.RouteWithOptions(ctx, serverType, specKey, routingKey, payload, domain.RouteOptions{AllowStart: true})
}

func (r *MetricRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, opts domain.RouteOptions) (json.RawMessage, error) {
	start := time.Now()
	resp, err := r.inner.RouteWithOptions(ctx, serverType, specKey, routingKey, payload, opts)
	r.observe(ctx, serverType, time.Since(start), err)
	return resp, err
}

func (r *MetricRouter) observe(ctx context.Context, serverType string, duration time.Duration, err error) {
	if r.metrics == nil {
		return
	}
	meta, _ := domain.RouteContextFrom(ctx)
	status, reason := classifyRouteResult(err)
	r.metrics.ObserveRoute(domain.RouteMetric{
		ServerType: serverType,
		Client:     meta.Client,
		Status:     status,
		Reason:     reason,
		Duration:   duration,
	})
}

func classifyRouteResult(err error) (domain.RouteStatus, domain.RouteReason) {
	if err == nil {
		return domain.RouteStatusSuccess, domain.RouteReasonSuccess
	}
	if errors.Is(err, domain.ErrMethodNotAllowed) {
		return domain.RouteStatusError, domain.RouteReasonMethodNotAllowed
	}
	if errors.Is(err, domain.ErrInvalidRequest) {
		return domain.RouteStatusError, domain.RouteReasonInvalidRequest
	}
	if errors.Is(err, domain.ErrConnectionClosed) {
		return domain.RouteStatusError, domain.RouteReasonConnClosed
	}
	if stage, ok := domain.RouteStageFrom(err); ok {
		switch stage {
		case domain.RouteStageDecode, domain.RouteStageValidate:
			return domain.RouteStatusError, domain.RouteReasonInvalidRequest
		case domain.RouteStageAcquire:
			if errors.Is(err, context.DeadlineExceeded) {
				return domain.RouteStatusError, domain.RouteReasonTimeoutColdStart
			}
			return domain.RouteStatusError, domain.RouteReasonAcquireFailed
		case domain.RouteStageCall:
			if errors.Is(err, context.DeadlineExceeded) {
				return domain.RouteStatusError, domain.RouteReasonTimeoutExecution
			}
			return domain.RouteStatusError, domain.RouteReasonExecutionFailed
		}
	}
	return domain.RouteStatusError, domain.RouteReasonUnknown
}
