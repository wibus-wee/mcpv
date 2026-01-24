package domain

import (
	"context"
	"time"
)

// RouteContext carries client metadata for routing.
type RouteContext struct {
	Client string
}

type routeContextKey struct{}

// StartCauseReason labels why an instance was started.
type StartCauseReason string

const (
	// StartCauseBootstrap indicates bootstrap triggered the start.
	StartCauseBootstrap StartCauseReason = "bootstrap"
	// StartCauseToolCall indicates a tool call triggered the start.
	StartCauseToolCall StartCauseReason = "tool_call"
	// StartCauseClientActivate indicates client activation triggered the start.
	StartCauseClientActivate StartCauseReason = "client_activate"
	// StartCausePolicyAlwaysOn indicates always-on policy triggered the start.
	StartCausePolicyAlwaysOn StartCauseReason = "policy_always_on"
	// StartCausePolicyMinReady indicates min-ready policy triggered the start.
	StartCausePolicyMinReady StartCauseReason = "policy_min_ready"
)

// StartCausePolicy captures policy details for a start cause.
type StartCausePolicy struct {
	ActivationMode ActivationMode `json:"activationMode"`
	MinReady       int            `json:"minReady"`
}

// StartCause describes why an instance was started.
type StartCause struct {
	Reason    StartCauseReason  `json:"reason"`
	Client    string            `json:"client,omitempty"`
	ToolName  string            `json:"toolName,omitempty"`
	Policy    *StartCausePolicy `json:"policy,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

type startCauseKey struct{}

// WithRouteContext attaches routing metadata to a context.
func WithRouteContext(ctx context.Context, meta RouteContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, routeContextKey{}, meta)
}

// RouteContextFrom extracts routing metadata from a context.
func RouteContextFrom(ctx context.Context) (RouteContext, bool) {
	if ctx == nil {
		return RouteContext{}, false
	}
	meta, ok := ctx.Value(routeContextKey{}).(RouteContext)
	return meta, ok
}

// WithStartCause attaches a start cause to a context.
func WithStartCause(ctx context.Context, cause StartCause) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, startCauseKey{}, cause)
}

// StartCauseFromContext extracts a start cause from a context.
func StartCauseFromContext(ctx context.Context) (StartCause, bool) {
	if ctx == nil {
		return StartCause{}, false
	}
	cause, ok := ctx.Value(startCauseKey{}).(StartCause)
	return cause, ok
}

// CloneStartCause deep-copies a start cause when it is non-nil.
func CloneStartCause(cause *StartCause) *StartCause {
	if cause == nil {
		return nil
	}
	copyCause := *cause
	if cause.Policy != nil {
		policyCopy := *cause.Policy
		copyCause.Policy = &policyCopy
	}
	return &copyCause
}
