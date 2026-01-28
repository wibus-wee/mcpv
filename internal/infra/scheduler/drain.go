package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"mcpd/internal/infra/telemetry"
)

func (s *BasicScheduler) startDrain(specKey string, inst *trackedInstance, timeout time.Duration, reason string) {
	inst.drainOnce.Do(func() {
		inst.drainDone = make(chan struct{})

		s.logger.Info("drain started",
			telemetry.EventField("drain_start"),
			telemetry.ServerTypeField(specKey),
			telemetry.InstanceIDField(inst.instance.ID()),
			zap.Int("busyCount", inst.instance.BusyCount()),
			zap.Duration("timeout", timeout),
		)

		go func() {
			timer := time.NewTimer(timeout)
			defer timer.Stop()

			timedOut := false
			select {
			case <-inst.drainDone:
			case <-timer.C:
				timedOut = true
			}

			state := s.getPool(specKey, inst.instance.Spec())
			state.mu.Lock()
			state.removeDrainingLocked(inst)
			state.mu.Unlock()

			finalReason := reason
			if timedOut {
				finalReason = "drain timeout"
				s.logger.Warn("drain timeout, forcing stop",
					telemetry.EventField("drain_timeout"),
					telemetry.ServerTypeField(specKey),
					telemetry.InstanceIDField(inst.instance.ID()),
				)
			} else {
				s.logger.Info("drain completed",
					telemetry.EventField("drain_complete"),
					telemetry.ServerTypeField(specKey),
					telemetry.InstanceIDField(inst.instance.ID()),
				)
			}

			err := s.stopInstance(context.Background(), state.spec, inst.instance, finalReason)
			s.observeInstanceStop(inst.instance.Spec().Name, err)
			s.recordInstanceStop(state)
		}()

		state := s.getPool(specKey, inst.instance.Spec())
		state.mu.Lock()
		busy := inst.instance.BusyCount()
		state.mu.Unlock()
		if busy == 0 {
			select {
			case <-inst.drainDone:
			default:
				close(inst.drainDone)
			}
		}
	})
}
