package scheduler

import (
	"context"
	"time"

	"mcpv/internal/domain"
)

func (s *BasicScheduler) applyStartCause(ctx context.Context, inst *domain.Instance, started time.Time) {
	if inst == nil {
		return
	}
	cause, ok := domain.StartCauseFromContext(ctx)
	if !ok {
		return
	}
	if cause.Timestamp.IsZero() {
		cause.Timestamp = started
	}
	if !shouldOverrideCause(inst.LastStartCause(), cause) {
		return
	}
	inst.SetLastStartCause(&cause)
}

func (s *BasicScheduler) applyStartCauseLocked(state *poolState, cause domain.StartCause, started time.Time) {
	if cause.Timestamp.IsZero() {
		cause.Timestamp = started
	}
	for _, inst := range state.instances {
		s.updateInstanceCauseLocked(inst.instance, cause)
	}
	for _, inst := range state.draining {
		s.updateInstanceCauseLocked(inst.instance, cause)
	}
}

func (s *BasicScheduler) updateInstanceCauseLocked(inst *domain.Instance, cause domain.StartCause) {
	if inst == nil {
		return
	}
	if !shouldOverrideCause(inst.LastStartCause(), cause) {
		return
	}
	inst.SetLastStartCause(&cause)
}

func shouldOverrideCause(existing *domain.StartCause, next domain.StartCause) bool {
	if existing == nil {
		return true
	}
	if existing.Reason == domain.StartCauseBootstrap && next.Reason != domain.StartCauseBootstrap {
		return true
	}
	return false
}
