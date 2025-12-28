package ui

import (
	"encoding/json"

	"github.com/wailsapp/wails/v3/pkg/application"

	"mcpd/internal/domain"
)

// Event name constants for Wails event emission
const (
	// Core lifecycle events
	EventCoreState = "core:state"

	// Data update events
	EventToolsUpdated     = "tools:updated"
	EventResourcesUpdated = "resources:updated"
	EventPromptsUpdated   = "prompts:updated"

	// Log streaming events
	EventLogEntry = "logs:entry"

	// Error events
	EventError = "error"
)

// CoreStateEvent represents core state changes
type CoreStateEvent struct {
	State  string  `json:"state"`
	Error  *string `json:"error,omitempty"`
	Uptime int64   `json:"uptime,omitempty"`
}

// ToolsUpdatedEvent represents tools snapshot updates
type ToolsUpdatedEvent struct {
	ETag  string      `json:"etag"`
	Tools []ToolEntry `json:"tools"`
}

// ResourcesUpdatedEvent represents resources snapshot updates
type ResourcesUpdatedEvent struct {
	ETag      string          `json:"etag"`
	Resources []ResourceEntry `json:"resources"`
}

// PromptsUpdatedEvent represents prompts snapshot updates
type PromptsUpdatedEvent struct {
	ETag    string        `json:"etag"`
	Prompts []PromptEntry `json:"prompts"`
}

// LogEntryEvent represents a single log entry
type LogEntryEvent struct {
	Logger    string          `json:"logger"`
	Level     string          `json:"level"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// ErrorEvent represents an error event
type ErrorEvent struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Helper functions for event emission

func emitCoreState(app *application.App, state string, err error) {
	if app == nil {
		return
	}
	event := CoreStateEvent{State: state}
	if err != nil {
		errMsg := err.Error()
		event.Error = &errMsg
	}
	app.Event.Emit(EventCoreState, event)
}

func emitToolsUpdated(app *application.App, snapshot domain.ToolSnapshot) {
	if app == nil {
		return
	}
	tools := make([]ToolEntry, 0, len(snapshot.Tools))
	for _, t := range snapshot.Tools {
		tools = append(tools, ToolEntry{
			Name:     t.Name,
			ToolJSON: t.ToolJSON,
		})
	}
	event := ToolsUpdatedEvent{
		ETag:  snapshot.ETag,
		Tools: tools,
	}
	app.Event.Emit(EventToolsUpdated, event)
}

func emitResourcesUpdated(app *application.App, snapshot domain.ResourceSnapshot) {
	if app == nil {
		return
	}
	resources := make([]ResourceEntry, 0, len(snapshot.Resources))
	for _, r := range snapshot.Resources {
		resources = append(resources, ResourceEntry{
			URI:          r.URI,
			ResourceJSON: r.ResourceJSON,
		})
	}
	event := ResourcesUpdatedEvent{
		ETag:      snapshot.ETag,
		Resources: resources,
	}
	app.Event.Emit(EventResourcesUpdated, event)
}

func emitPromptsUpdated(app *application.App, snapshot domain.PromptSnapshot) {
	if app == nil {
		return
	}
	prompts := make([]PromptEntry, 0, len(snapshot.Prompts))
	for _, p := range snapshot.Prompts {
		prompts = append(prompts, PromptEntry{
			Name:       p.Name,
			PromptJSON: p.PromptJSON,
		})
	}
	event := PromptsUpdatedEvent{
		ETag:    snapshot.ETag,
		Prompts: prompts,
	}
	app.Event.Emit(EventPromptsUpdated, event)
}

func emitLogEntry(app *application.App, entry domain.LogEntry) {
	if app == nil {
		return
	}
	event := LogEntryEvent{
		Logger:    entry.Logger,
		Level:     string(entry.Level),
		Timestamp: entry.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
		Data:      entry.DataJSON,
	}
	app.Event.Emit(EventLogEntry, event)
}

func emitError(app *application.App, code, message, details string) {
	if app == nil {
		return
	}
	event := ErrorEvent{
		Code:    code,
		Message: message,
		Details: details,
	}
	app.Event.Emit(EventError, event)
}
