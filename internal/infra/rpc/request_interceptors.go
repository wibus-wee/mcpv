package rpc

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"mcpv/internal/infra/telemetry"
)

type requestContextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *requestContextServerStream) Context() context.Context {
	return s.ctx
}

func requestContextUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, _ = ensureRequestMeta(ctx)
		return handler(ctx, req)
	}
}

func requestContextStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, _ := ensureRequestMeta(stream.Context())
		wrapped := &requestContextServerStream{ServerStream: stream, ctx: ctx}
		return handler(srv, wrapped)
	}
}

func requestContextUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = injectRequestID(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func requestContextStreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = injectRequestID(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

func ensureRequestMeta(ctx context.Context) (context.Context, telemetry.RequestMeta) {
	requestID := requestIDFromMetadata(ctx)
	return telemetry.EnsureRequestMeta(ctx, requestID)
}

func requestIDFromMetadata(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(telemetry.RequestIDHeader)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func injectRequestID(ctx context.Context) context.Context {
	if ctx == nil {
		return ctx
	}
	requestID, ok := telemetry.RequestIDFromContext(ctx)
	if !ok || requestID == "" {
		return ctx
	}
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		if len(md.Get(telemetry.RequestIDHeader)) > 0 {
			return ctx
		}
		md = md.Copy()
	} else {
		md = metadata.New(nil)
	}
	md.Set(telemetry.RequestIDHeader, requestID)
	return metadata.NewOutgoingContext(ctx, md)
}
