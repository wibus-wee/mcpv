package scheduler

import (
	"context"
	"errors"

	"mcpv/internal/domain"
)

func wrapSchedulerError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, ErrNoCapacity) || errors.Is(err, ErrStickyBusy) {
		return domain.Wrap(domain.CodeUnavailable, op, err)
	}
	if errors.Is(err, ErrNotImplemented) {
		return domain.Wrap(domain.CodeNotImplemented, op, err)
	}
	if code, ok := domain.CodeFrom(err); ok {
		return domain.Wrap(code, op, err)
	}
	return domain.Wrap(domain.CodeInternal, op, err)
}
