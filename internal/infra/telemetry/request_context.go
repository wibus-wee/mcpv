package telemetry

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const RequestIDHeader = "x-request-id"

type requestContextKey struct{}

type RequestMeta struct {
	RequestID string
	TraceID   string
	SpanID    string
}

func (m RequestMeta) IsZero() bool {
	return m.RequestID == "" && m.TraceID == "" && m.SpanID == ""
}

func WithRequestMeta(ctx context.Context, meta RequestMeta) context.Context {
	if meta.IsZero() {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestContextKey{}, meta)
}

func RequestMetaFromContext(ctx context.Context) (RequestMeta, bool) {
	if ctx == nil {
		return RequestMeta{}, false
	}
	meta, ok := ctx.Value(requestContextKey{}).(RequestMeta)
	return meta, ok && !meta.IsZero()
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	meta, ok := RequestMetaFromContext(ctx)
	if !ok || meta.RequestID == "" {
		return "", false
	}
	return meta.RequestID, true
}

func NewRequestID() string {
	return uuid.NewString()
}

func TraceSpanFromContext(ctx context.Context) (string, string) {
	if ctx == nil {
		return "", ""
	}
	span := trace.SpanFromContext(ctx)
	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		return "", ""
	}
	return spanCtx.TraceID().String(), spanCtx.SpanID().String()
}

func BuildRequestMeta(ctx context.Context, requestID string) RequestMeta {
	traceID, spanID := TraceSpanFromContext(ctx)
	return RequestMeta{
		RequestID: requestID,
		TraceID:   traceID,
		SpanID:    spanID,
	}
}

func EnsureRequestMeta(ctx context.Context, requestID string) (context.Context, RequestMeta) {
	if existing, ok := RequestMetaFromContext(ctx); ok {
		if requestID == "" {
			requestID = existing.RequestID
		}
	}
	if requestID == "" {
		requestID = NewRequestID()
	}
	meta := BuildRequestMeta(ctx, requestID)
	return WithRequestMeta(ctx, meta), meta
}

func RequestFields(meta RequestMeta) []zap.Field {
	if meta.IsZero() {
		return nil
	}
	fields := make([]zap.Field, 0, 3)
	if meta.RequestID != "" {
		fields = append(fields, RequestIDField(meta.RequestID))
	}
	if meta.TraceID != "" {
		fields = append(fields, TraceIDField(meta.TraceID))
	}
	if meta.SpanID != "" {
		fields = append(fields, SpanIDField(meta.SpanID))
	}
	return fields
}

func RequestFieldsFromContext(ctx context.Context) []zap.Field {
	meta, ok := RequestMetaFromContext(ctx)
	if !ok {
		return nil
	}
	return RequestFields(meta)
}

func LoggerWithRequest(ctx context.Context, base *zap.Logger) *zap.Logger {
	logger := base
	if logger == nil {
		logger = zap.NewNop()
	}
	fields := RequestFieldsFromContext(ctx)
	if len(fields) == 0 {
		return logger
	}
	return logger.With(fields...)
}
