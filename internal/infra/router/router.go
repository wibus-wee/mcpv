package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"go.uber.org/zap"

	"mcpv/internal/domain"
	"mcpv/internal/infra/telemetry"
)

type BasicRouter struct {
	scheduler domain.Scheduler
	timeout   atomic.Int64
	logger    *zap.Logger
}

type Options struct {
	Timeout time.Duration
	Logger  *zap.Logger
}

func NewBasicRouter(scheduler domain.Scheduler, opts Options) *BasicRouter {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = time.Duration(domain.DefaultRouteTimeoutSeconds) * time.Second
	}
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	router := &BasicRouter{
		scheduler: scheduler,
		logger:    logger.Named("router"),
	}
	router.timeout.Store(int64(timeout))
	return router
}

// SetTimeout updates the route timeout duration.
func (r *BasicRouter) SetTimeout(timeout time.Duration) {
	if timeout <= 0 {
		timeout = time.Duration(domain.DefaultRouteTimeoutSeconds) * time.Second
	}
	r.timeout.Store(int64(timeout))
}

func (r *BasicRouter) Route(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage) (json.RawMessage, error) {
	return r.RouteWithOptions(ctx, serverType, specKey, routingKey, payload, domain.RouteOptions{AllowStart: true})
}

func (r *BasicRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, opts domain.RouteOptions) (json.RawMessage, error) {
	start := time.Now()

	method, isCall, err := extractMethod(payload)
	if err != nil {
		decodeErr := domain.Wrap(domain.CodeInvalidArgument, "route decode", err)
		routeErr := domain.NewRouteError(domain.RouteStageDecode, decodeErr)
		r.logRouteError(serverType, "", nil, start, routeErr)
		return nil, routeErr
	}
	if method == "" || !isCall {
		routeErr := domain.NewRouteError(domain.RouteStageValidate, domain.ErrInvalidRequest)
		r.logRouteError(serverType, method, nil, start, routeErr)
		return nil, routeErr
	}

	var inst *domain.Instance
	if opts.AllowStart {
		inst, err = r.scheduler.Acquire(ctx, specKey, routingKey)
	} else {
		inst, err = r.scheduler.AcquireReady(ctx, specKey, routingKey)
	}
	if err != nil {
		routeErr := domain.NewRouteError(domain.RouteStageAcquire, err)
		if opts.AllowStart || !errors.Is(err, domain.ErrNoReadyInstance) {
			r.logRouteError(serverType, method, nil, start, routeErr)
		}
		return nil, routeErr
	}
	defer func() { _ = r.scheduler.Release(ctx, inst) }()

	if inst.Conn() == nil {
		err := fmt.Errorf("%w: instance has no connection: %s", domain.ErrConnectionClosed, inst.ID())
		callErr := domain.Wrap(domain.CodeUnavailable, "route call", err)
		routeErr := domain.NewRouteError(domain.RouteStageCall, callErr)
		r.logRouteError(serverType, method, inst, start, routeErr)
		return nil, routeErr
	}

	if !domain.MethodAllowed(inst.Capabilities(), method) {
		routeErr := domain.NewRouteError(domain.RouteStageValidate, domain.ErrMethodNotAllowed)
		r.logRouteError(serverType, method, inst, start, routeErr)
		return nil, routeErr
	}

	callCtx, cancel := context.WithTimeout(ctx, r.timeoutDuration())
	defer cancel()

	callStarted := time.Now()
	resp, err := inst.Conn().Call(callCtx, payload)
	inst.RecordCall(time.Since(callStarted), err)
	if err != nil {
		callErr := domain.Wrap(domain.CodeUnavailable, "route call", err)
		routeErr := domain.NewRouteError(domain.RouteStageCall, callErr)
		r.logRouteError(serverType, method, inst, start, routeErr)
		return nil, routeErr
	}

	return resp, nil
}

func (r *BasicRouter) timeoutDuration() time.Duration {
	return time.Duration(r.timeout.Load())
}

func (r *BasicRouter) logRouteError(serverType, method string, inst *domain.Instance, start time.Time, err error) {
	fields := []zap.Field{
		telemetry.EventField(telemetry.EventRouteError),
		telemetry.ServerTypeField(serverType),
		telemetry.DurationField(time.Since(start)),
		zap.Error(err),
	}
	if method != "" {
		fields = append(fields, zap.String("method", method))
	}
	if inst != nil {
		fields = append(fields,
			telemetry.InstanceIDField(inst.ID()),
			telemetry.StateField(string(inst.State())),
		)
	}
	r.logger.Warn("route failed", fields...)
}

func extractMethod(payload json.RawMessage) (string, bool, error) {
	msg, err := jsonrpc.DecodeMessage(payload)
	if err != nil {
		return "", false, err
	}
	if req, ok := msg.(*jsonrpc.Request); ok {
		return req.Method, req.ID.IsValid(), nil
	}
	return "", false, nil
}
