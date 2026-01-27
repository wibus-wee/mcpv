package domain

import (
	"encoding/json"
	"time"
)

// SamplingContent represents a single sampling content item.
type SamplingContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// SamplingMessage represents a single message in a sampling request.
type SamplingMessage struct {
	Role    string          `json:"role"`
	Content SamplingContent `json:"content"`
}

// ModelHint is a model preference hint.
type ModelHint struct {
	Name string `json:"name"`
}

// ModelPreferences captures model selection hints.
type ModelPreferences struct {
	Hints                []ModelHint `json:"hints,omitempty"`
	IntelligencePriority *float64    `json:"intelligencePriority,omitempty"`
	SpeedPriority        *float64    `json:"speedPriority,omitempty"`
	CostPriority         *float64    `json:"costPriority,omitempty"`
}

// SamplingRequest defines the inputs for sampling/createMessage.
type SamplingRequest struct {
	IncludeContext   string            `json:"includeContext,omitempty"`
	MaxTokens        int64             `json:"maxTokens,omitempty"`
	Messages         []SamplingMessage `json:"messages"`
	Metadata         any               `json:"metadata,omitempty"`
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	StopSequences    []string          `json:"stopSequences,omitempty"`
	SystemPrompt     string            `json:"systemPrompt,omitempty"`
	Temperature      float64           `json:"temperature,omitempty"`
}

// SamplingResult represents the response to sampling/createMessage.
type SamplingResult struct {
	Role       string          `json:"role"`
	Content    SamplingContent `json:"content"`
	Model      string          `json:"model,omitempty"`
	StopReason string          `json:"stopReason,omitempty"`
}

// ElicitationRequest defines the inputs for elicitation/create.
type ElicitationRequest struct {
	Message         string          `json:"message,omitempty"`
	Mode            string          `json:"mode,omitempty"`
	RequestedSchema json.RawMessage `json:"requestedSchema,omitempty"`
	ElicitationID   string          `json:"elicitationId,omitempty"`
	URL             string          `json:"url,omitempty"`
}

// ElicitationResult represents the response to elicitation/create.
type ElicitationResult struct {
	Action  string         `json:"action"`
	Content map[string]any `json:"content,omitempty"`
}

// TaskStatus describes task lifecycle status.
type TaskStatus string

const (
	TaskStatusWorking       TaskStatus = "working"
	TaskStatusCompleted     TaskStatus = "completed"
	TaskStatusFailed        TaskStatus = "failed"
	TaskStatusCancelled     TaskStatus = "cancelled"
	TaskStatusInputRequired TaskStatus = "input_required"
)

// Task describes a tracked task.
type Task struct {
	TaskID        string     `json:"taskId"`
	Status        TaskStatus `json:"status"`
	StatusMessage string     `json:"statusMessage,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	LastUpdatedAt time.Time  `json:"lastUpdatedAt"`
	TTL           *int64     `json:"ttl,omitempty"`
	PollInterval  *int64     `json:"pollInterval,omitempty"`
}

// TaskResult holds the final result for a task.
type TaskResult struct {
	Status TaskStatus      `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ProtocolError  `json:"error,omitempty"`
}

// TaskPage represents a paginated task list.
type TaskPage struct {
	Tasks      []Task `json:"tasks"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// TaskCreateOptions captures task creation preferences.
type TaskCreateOptions struct {
	TTL          *int64 `json:"ttl,omitempty"`
	PollInterval *int64 `json:"pollInterval,omitempty"`
}
