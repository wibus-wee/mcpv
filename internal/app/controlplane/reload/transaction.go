package reload

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"mcpv/internal/domain"
)

type Transaction struct {
	observer *Observer
	logger   *zap.Logger
}

func NewTransaction(observer *Observer, logger *zap.Logger) *Transaction {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Transaction{
		observer: observer,
		logger:   logger,
	}
}

func (t *Transaction) Apply(ctx context.Context, steps []Step, mode domain.ReloadMode) error {
	applied := make([]Step, 0, len(steps))
	for _, step := range steps {
		if err := step.Apply(ctx); err != nil {
			applyErr := WrapStage(step.Name, err)
			rollbackStart := time.Now()
			if rollbackErr := t.rollbackSteps(ctx, applied); rollbackErr != nil {
				rollbackDuration := time.Since(rollbackStart)
				if t.observer != nil {
					t.observer.ObserveReloadRollback(mode, domain.ReloadRollbackResultFailure, step.Name, rollbackDuration)
				}
				t.logger.Warn("config reload rollback failed",
					zap.String("failure_stage", step.Name),
					zap.Duration("latency", rollbackDuration),
					zap.Error(rollbackErr),
				)
				return errors.Join(applyErr, WrapStage("rollback", rollbackErr))
			}
			rollbackDuration := time.Since(rollbackStart)
			if t.observer != nil {
				t.observer.ObserveReloadRollback(mode, domain.ReloadRollbackResultSuccess, step.Name, rollbackDuration)
			}
			t.logger.Info("config reload rolled back",
				zap.String("failure_stage", step.Name),
				zap.Duration("latency", rollbackDuration),
			)
			return applyErr
		}
		applied = append(applied, step)
	}
	return nil
}

func (t *Transaction) rollbackSteps(ctx context.Context, steps []Step) error {
	if len(steps) == 0 {
		return nil
	}
	var rollbackErr error
	for i := len(steps) - 1; i >= 0; i-- {
		step := steps[i]
		if step.Rollback == nil {
			continue
		}
		if err := step.Rollback(ctx); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	return rollbackErr
}
