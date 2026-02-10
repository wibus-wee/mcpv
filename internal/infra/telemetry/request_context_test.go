package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestEnsureRequestMetaGeneratesID(t *testing.T) {
	ctx, meta := EnsureRequestMeta(context.Background(), "")
	require.NotEmpty(t, meta.RequestID)

	got, ok := RequestIDFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, meta.RequestID, got)
}

func TestEnsureRequestMetaUsesProvidedID(t *testing.T) {
	ctx, meta := EnsureRequestMeta(context.Background(), "req-123")
	require.Equal(t, "req-123", meta.RequestID)

	got, ok := RequestIDFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "req-123", got)
}

func TestTraceSpanFromContext(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("0123456789abcdef")
	require.NoError(t, err)
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	gotTraceID, gotSpanID := TraceSpanFromContext(ctx)
	require.Equal(t, traceID.String(), gotTraceID)
	require.Equal(t, spanID.String(), gotSpanID)
}

func TestRequestFields(t *testing.T) {
	fields := RequestFields(RequestMeta{
		RequestID: "req-1",
		TraceID:   "trace-1",
		SpanID:    "span-1",
	})
	require.Len(t, fields, 3)
	require.Equal(t, FieldRequestID, fields[0].Key)
	require.Equal(t, FieldTraceID, fields[1].Key)
	require.Equal(t, FieldSpanID, fields[2].Key)
}
