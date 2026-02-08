package reload

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"mcpv/internal/domain"
)

func TestReloadTransactionApply_RollbackSuccess(t *testing.T) {
	sequence := make([]string, 0, 4)
	applyErr := errors.New("apply failed")

	steps := []Step{
		{
			Name: "step1",
			Apply: func(context.Context) error {
				sequence = append(sequence, "apply1")
				return nil
			},
			Rollback: func(context.Context) error {
				sequence = append(sequence, "rollback1")
				return nil
			},
		},
		{
			Name: "step2",
			Apply: func(context.Context) error {
				sequence = append(sequence, "apply2")
				return applyErr
			},
			Rollback: func(context.Context) error {
				sequence = append(sequence, "rollback2")
				return nil
			},
		},
	}

	transaction := NewTransaction(nil, zap.NewNop())
	err := transaction.Apply(context.Background(), steps, domain.ReloadModeLenient)
	require.Error(t, err)

	var applyStageErr ApplyError
	require.ErrorAs(t, err, &applyStageErr)
	require.Equal(t, "step2", applyStageErr.Stage)

	require.Equal(t, []string{"apply1", "apply2", "rollback1"}, sequence)
}

func TestReloadTransactionApply_RollbackFailure(t *testing.T) {
	applyErr := errors.New("apply failed")
	rollbackErr := errors.New("rollback failed")

	steps := []Step{
		{
			Name: "step1",
			Apply: func(context.Context) error {
				return nil
			},
			Rollback: func(context.Context) error {
				return rollbackErr
			},
		},
		{
			Name: "step2",
			Apply: func(context.Context) error {
				return applyErr
			},
			Rollback: func(context.Context) error {
				return nil
			},
		},
	}

	transaction := NewTransaction(nil, zap.NewNop())
	err := transaction.Apply(context.Background(), steps, domain.ReloadModeLenient)
	require.Error(t, err)
	require.ErrorIs(t, err, applyErr)
	require.ErrorIs(t, err, rollbackErr)
}
