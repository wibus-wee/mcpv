package domain

import "context"

type RouteContext struct {
	Caller  string
	Profile string
}

type routeContextKey struct{}

func WithRouteContext(ctx context.Context, meta RouteContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, routeContextKey{}, meta)
}

func RouteContextFrom(ctx context.Context) (RouteContext, bool) {
	if ctx == nil {
		return RouteContext{}, false
	}
	meta, ok := ctx.Value(routeContextKey{}).(RouteContext)
	return meta, ok
}
