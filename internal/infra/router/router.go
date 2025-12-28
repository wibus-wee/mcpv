package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"go.uber.org/zap"

	"mcpd/internal/domain"
	"mcpd/internal/infra/telemetry"
)

type BasicRouter struct {
	scheduler domain.Scheduler
	timeout   time.Duration
	logger    *zap.Logger
	metrics   domain.Metrics
}

type RouterOptions struct {
	Timeout time.Duration
	Logger  *zap.Logger
	Metrics domain.Metrics
}

func NewBasicRouter(scheduler domain.Scheduler, opts RouterOptions) *BasicRouter {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = time.Duration(domain.DefaultRouteTimeoutSeconds) * time.Second
	}
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BasicRouter{
		scheduler: scheduler,
		timeout:   timeout,
		logger:    logger.Named("router"),
		metrics:   opts.Metrics,
	}
}

func (r *BasicRouter) Route(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage) (json.RawMessage, error) {
	return r.RouteWithOptions(ctx, serverType, specKey, routingKey, payload, domain.RouteOptions{AllowStart: true})
}

func (r *BasicRouter) RouteWithOptions(ctx context.Context, serverType, specKey, routingKey string, payload json.RawMessage, opts domain.RouteOptions) (json.RawMessage, error) {
	start := time.Now()

	method, isCall, err := extractMethod(payload)
	if err != nil {
		decodeErr := fmt.Errorf("decode request: %w", err)
		r.logRouteError(serverType, "", nil, start, decodeErr)
		return nil, decodeErr
	}
	if method == "" || !isCall {
		r.logRouteError(serverType, method, nil, start, domain.ErrInvalidRequest)
		return nil, domain.ErrInvalidRequest
	}

	var inst *domain.Instance
	if opts.AllowStart {
		inst, err = r.scheduler.Acquire(ctx, specKey, routingKey)
	} else {
		inst, err = r.scheduler.AcquireReady(ctx, specKey, routingKey)
	}
	if err != nil {
		r.observeRoute(serverType, start, err)
		if opts.AllowStart || !errors.Is(err, domain.ErrNoReadyInstance) {
			r.logRouteError(serverType, method, nil, start, err)
		}
		return nil, err
	}
	defer func() { _ = r.scheduler.Release(ctx, inst) }()

	if inst.Conn == nil {
		err := fmt.Errorf("instance has no connection: %s", inst.ID)
		r.observeRoute(serverType, start, err)
		r.logRouteError(serverType, method, inst, start, err)
		return nil, err
	}

	if !domain.MethodAllowed(inst.Capabilities, method) {
		r.observeRoute(serverType, start, domain.ErrMethodNotAllowed)
		r.logRouteError(serverType, method, inst, start, domain.ErrMethodNotAllowed)
		return nil, domain.ErrMethodNotAllowed
	}

	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if err := inst.Conn.Send(callCtx, payload); err != nil {
		sendErr := fmt.Errorf("send request: %w", err)
		r.observeRoute(serverType, start, sendErr)
		r.logRouteError(serverType, method, inst, start, sendErr)
		return nil, sendErr
	}

	resp, err := inst.Conn.Recv(callCtx)
	if err != nil {
		recvErr := fmt.Errorf("receive response: %w", err)
		r.observeRoute(serverType, start, recvErr)
		r.logRouteError(serverType, method, inst, start, recvErr)
		return nil, recvErr
	}

	r.observeRoute(serverType, start, nil)
	return resp, nil
}

func (r *BasicRouter) observeRoute(serverType string, start time.Time, err error) {
	if r.metrics == nil {
		return
	}
	r.metrics.ObserveRoute(serverType, time.Since(start), err)
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
			telemetry.InstanceIDField(inst.ID),
			telemetry.StateField(string(inst.State)),
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
