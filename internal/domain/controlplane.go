package domain

import (
	"context"
	"encoding/json"
	"time"
)

type ControlPlaneInfo struct {
	Name    string
	Version string
	Build   string
}

type ToolDefinition struct {
	Name     string
	ToolJSON json.RawMessage
}

type ToolSnapshot struct {
	ETag  string
	Tools []ToolDefinition
}

type ToolTarget struct {
	ServerType string
	SpecKey    string
	ToolName   string
}

type ResourceDefinition struct {
	URI          string
	ResourceJSON json.RawMessage
}

type ResourceSnapshot struct {
	ETag      string
	Resources []ResourceDefinition
}

type ResourceTarget struct {
	ServerType string
	SpecKey    string
	URI        string
}

type ResourcePage struct {
	Snapshot   ResourceSnapshot
	NextCursor string
}

type PromptDefinition struct {
	Name       string
	PromptJSON json.RawMessage
}

type PromptSnapshot struct {
	ETag    string
	Prompts []PromptDefinition
}

type PromptTarget struct {
	ServerType string
	SpecKey    string
	PromptName string
}

type PromptPage struct {
	Snapshot   PromptSnapshot
	NextCursor string
}

type LogLevel string

const (
	LogLevelDebug     LogLevel = "debug"
	LogLevelInfo      LogLevel = "info"
	LogLevelNotice    LogLevel = "notice"
	LogLevelWarning   LogLevel = "warning"
	LogLevelError     LogLevel = "error"
	LogLevelCritical  LogLevel = "critical"
	LogLevelAlert     LogLevel = "alert"
	LogLevelEmergency LogLevel = "emergency"
)

type LogEntry struct {
	Logger    string
	Level     LogLevel
	Timestamp time.Time
	DataJSON  json.RawMessage
}

type ControlPlane interface {
	Info(ctx context.Context) (ControlPlaneInfo, error)
	RegisterCaller(ctx context.Context, caller string, pid int) (string, error)
	UnregisterCaller(ctx context.Context, caller string) error
	ListTools(ctx context.Context, caller string) (ToolSnapshot, error)
	WatchTools(ctx context.Context, caller string) (<-chan ToolSnapshot, error)
	CallTool(ctx context.Context, caller, name string, args json.RawMessage, routingKey string) (json.RawMessage, error)
	ListResources(ctx context.Context, caller string, cursor string) (ResourcePage, error)
	WatchResources(ctx context.Context, caller string) (<-chan ResourceSnapshot, error)
	ReadResource(ctx context.Context, caller, uri string) (json.RawMessage, error)
	ListPrompts(ctx context.Context, caller string, cursor string) (PromptPage, error)
	WatchPrompts(ctx context.Context, caller string) (<-chan PromptSnapshot, error)
	GetPrompt(ctx context.Context, caller, name string, args json.RawMessage) (json.RawMessage, error)
	StreamLogs(ctx context.Context, caller string, minLevel LogLevel) (<-chan LogEntry, error)
}
