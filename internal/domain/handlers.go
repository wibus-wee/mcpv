package domain

import (
	"context"
	"encoding/json"
)

// SamplingHandler handles sampling/createMessage requests.
type SamplingHandler interface {
	CreateMessage(ctx context.Context, params *SamplingRequest) (*SamplingResult, error)
}

// ElicitationHandler handles elicitation/create requests.
type ElicitationHandler interface {
	Elicit(ctx context.Context, params *ElicitationRequest) (*ElicitationResult, error)
}

// TaskRunner executes a task workload and returns its result.
type TaskRunner func(ctx context.Context) (json.RawMessage, *ProtocolError, error)

// TaskManager manages task lifecycle and results.
type TaskManager interface {
	Create(ctx context.Context, owner string, opts TaskCreateOptions, run TaskRunner) (Task, error)
	Get(ctx context.Context, owner, taskID string) (Task, bool)
	List(ctx context.Context, owner, cursor string, limit int) (TaskPage, error)
	Result(ctx context.Context, owner, taskID string) (TaskResult, error)
	Cancel(ctx context.Context, owner, taskID string) error
}
