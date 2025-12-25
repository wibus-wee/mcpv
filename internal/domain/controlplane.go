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
	ToolName   string
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
	ListTools(ctx context.Context) (ToolSnapshot, error)
	WatchTools(ctx context.Context) (<-chan ToolSnapshot, error)
	CallTool(ctx context.Context, name string, args json.RawMessage, routingKey string) (json.RawMessage, error)
	StreamLogs(ctx context.Context, minLevel LogLevel) (<-chan LogEntry, error)
}
