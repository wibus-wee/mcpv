package diagnostics

import (
	"context"
	"fmt"
	"time"
)

type contextKey string

const attemptIDKey contextKey = "diagnosticsAttemptID"

// WithAttemptID attaches the attempt ID to the context.
func WithAttemptID(ctx context.Context, attemptID string) context.Context {
	if ctx == nil || attemptID == "" {
		return ctx
	}
	return context.WithValue(ctx, attemptIDKey, attemptID)
}

// AttemptIDFromContext returns the attempt ID if present.
func AttemptIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value := ctx.Value(attemptIDKey)
	attemptID, ok := value.(string)
	return attemptID, ok && attemptID != ""
}

// EnsureAttemptID returns a context that carries an attempt ID.
// If an attempt ID already exists, it is returned unchanged.
func EnsureAttemptID(ctx context.Context, specKey string, now time.Time) (context.Context, string) {
	if attemptID, ok := AttemptIDFromContext(ctx); ok {
		return ctx, attemptID
	}
	attemptID := NewAttemptID(specKey, now)
	return WithAttemptID(ctx, attemptID), attemptID
}

// NewAttemptID creates a new attempt ID for a spec key.
func NewAttemptID(specKey string, now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return fmt.Sprintf("%s-%d", specKey, now.UnixNano())
}
