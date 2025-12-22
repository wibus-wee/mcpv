package router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"

	"mcpd/internal/domain"
)

type BasicRouter struct {
	scheduler domain.Scheduler
	capLookup CapabilityLookup
	timeout   time.Duration
}

type CapabilityLookup interface {
	Allowed(serverType string, method string) bool
}

type NoopCapabilities struct{}

func (n NoopCapabilities) Allowed(serverType, method string) bool { return true }

func NewBasicRouter(scheduler domain.Scheduler) *BasicRouter {
	return &BasicRouter{
		scheduler: scheduler,
		capLookup: NoopCapabilities{},
		timeout:   10 * time.Second,
	}
}

func (r *BasicRouter) Route(ctx context.Context, serverType, routingKey string, payload json.RawMessage) (json.RawMessage, error) {
	method := extractMethod(payload)
	if !r.capLookup.Allowed(serverType, method) {
		return nil, domain.ErrMethodNotAllowed
	}

	inst, err := r.scheduler.Acquire(ctx, serverType, routingKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.scheduler.Release(ctx, inst) }()

	if inst.Conn == nil {
		return nil, fmt.Errorf("instance has no connection: %s", inst.ID)
	}

	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if err := inst.Conn.Send(callCtx, payload); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	resp, err := inst.Conn.Recv(callCtx)
	if err != nil {
		return nil, fmt.Errorf("receive response: %w", err)
	}

	return resp, nil
}

func extractMethod(payload json.RawMessage) string {
	msg, err := jsonrpc.DecodeMessage(payload)
	if err != nil {
		return ""
	}
	if req, ok := msg.(*jsonrpc.Request); ok {
		return req.Method
	}
	return ""
}
