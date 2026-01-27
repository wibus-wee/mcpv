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

// TaskRunResult holds the output of a task runner.
type TaskRunResult struct {
	Result        json.RawMessage
	ProtocolError *ProtocolError
}

// TaskRunner executes a task workload and returns its result.
type TaskRunner func(ctx context.Context) (TaskRunResult, error)

// TaskManager manages task lifecycle and results.
type TaskManager interface {
	Create(ctx context.Context, owner string, opts TaskCreateOptions, run TaskRunner) (Task, error)
	Get(ctx context.Context, owner, taskID string) (Task, error)
	List(ctx context.Context, owner, cursor string, limit int) (TaskPage, error)
	Result(ctx context.Context, owner, taskID string) (TaskResult, error)
	Cancel(ctx context.Context, owner, taskID string) error
}
