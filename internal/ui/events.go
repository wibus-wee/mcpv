package ui

import (
	"encoding/json"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"mcpd/internal/domain"
	"mcpd/internal/infra/mcpcodec"
)

// Event name constants for Wails event emission
const (
	// Core lifecycle events
	EventCoreState = "core:state"

	// Data update events
	EventToolsUpdated     = "tools:updated"
	EventResourcesUpdated = "resources:updated"
	EventPromptsUpdated   = "prompts:updated"

	// Status update events
	EventRuntimeStatusUpdated = "runtime:status"
	EventServerInitUpdated    = "server-init:status"
	EventActiveClientsUpdated = "clients:active"

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

// RuntimeStatusUpdatedEvent represents runtime status updates
type RuntimeStatusUpdatedEvent struct {
	ETag     string                `json:"etag"`
	Statuses []ServerRuntimeStatus `json:"statuses"`
}

// ServerInitUpdatedEvent represents server init status updates
type ServerInitUpdatedEvent struct {
	Statuses []ServerInitStatus `json:"statuses"`
}

// ActiveClientsUpdatedEvent represents active client updates
type ActiveClientsUpdatedEvent struct {
	Clients []ActiveClient `json:"clients"`
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
		raw, err := mcpcodec.MarshalToolDefinition(t)
		if err != nil {
			continue
		}
		tools = append(tools, ToolEntry{
			Name:       t.Name,
			ToolJSON:   raw,
			SpecKey:    t.SpecKey,
			ServerName: t.ServerName,
			Source:     string(domain.ToolSourceLive),
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
		raw, err := mcpcodec.MarshalResourceDefinition(r)
		if err != nil {
			continue
		}
		resources = append(resources, ResourceEntry{
			URI:          r.URI,
			ResourceJSON: raw,
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
		raw, err := mcpcodec.MarshalPromptDefinition(p)
		if err != nil {
			continue
		}
		prompts = append(prompts, PromptEntry{
			Name:       p.Name,
			PromptJSON: raw,
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
	data := mustMarshalLogData(entry.Data)
	event := LogEntryEvent{
		Logger:    entry.Logger,
		Level:     string(entry.Level),
		Timestamp: entry.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
		Data:      data,
	}
	app.Event.Emit(EventLogEntry, event)
}

func mustMarshalLogData(data map[string]any) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return raw
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

func emitRuntimeStatusUpdated(app *application.App, snapshot domain.RuntimeStatusSnapshot) {
	if app == nil {
		return
	}
	statuses := make([]ServerRuntimeStatus, 0, len(snapshot.Statuses))
	for _, s := range snapshot.Statuses {
		instances := make([]InstanceStatus, 0, len(s.Instances))
		metrics := PoolMetrics{
			StartCount:      s.Metrics.StartCount,
			StopCount:       s.Metrics.StopCount,
			TotalCalls:      s.Metrics.TotalCalls,
			TotalErrors:     s.Metrics.TotalErrors,
			TotalDurationMs: s.Metrics.TotalDuration.Milliseconds(),
		}
		if !s.Metrics.LastCallAt.IsZero() {
			metrics.LastCallAt = s.Metrics.LastCallAt.UTC().Format(time.RFC3339Nano)
		}
		for _, inst := range s.Instances {
			instances = append(instances, InstanceStatus{
				ID:              inst.ID,
				State:           string(inst.State),
				BusyCount:       inst.BusyCount,
				LastActive:      inst.LastActive.Format("2006-01-02T15:04:05.000Z07:00"),
				SpawnedAt:       inst.SpawnedAt.Format("2006-01-02T15:04:05.000Z07:00"),
				HandshakedAt:    inst.HandshakedAt.Format("2006-01-02T15:04:05.000Z07:00"),
				LastHeartbeatAt: inst.LastHeartbeatAt.Format("2006-01-02T15:04:05.000Z07:00"),
				LastStartCause:  mapStartCause(inst.LastStartCause),
			})
		}
		statuses = append(statuses, ServerRuntimeStatus{
			SpecKey:    s.SpecKey,
			ServerName: s.ServerName,
			Instances:  instances,
			Stats: PoolStats{
				Total:        s.Stats.Total,
				Ready:        s.Stats.Ready,
				Busy:         s.Stats.Busy,
				Starting:     s.Stats.Starting,
				Initializing: s.Stats.Initializing,
				Handshaking:  s.Stats.Handshaking,
				Draining:     s.Stats.Draining,
				Failed:       s.Stats.Failed,
			},
			Metrics: metrics,
		})
	}
	event := RuntimeStatusUpdatedEvent{
		ETag:     snapshot.ETag,
		Statuses: statuses,
	}
	app.Event.Emit(EventRuntimeStatusUpdated, event)
}

func emitServerInitUpdated(app *application.App, snapshot domain.ServerInitStatusSnapshot) {
	if app == nil {
		return
	}
	statuses := make([]ServerInitStatus, 0, len(snapshot.Statuses))
	for _, s := range snapshot.Statuses {
		statuses = append(statuses, ServerInitStatus{
			SpecKey:    s.SpecKey,
			ServerName: s.ServerName,
			MinReady:   s.MinReady,
			Ready:      s.Ready,
			Failed:     s.Failed,
			State:      string(s.State),
			LastError:  s.LastError,
			UpdatedAt:  s.UpdatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
		})
	}
	event := ServerInitUpdatedEvent{
		Statuses: statuses,
	}
	app.Event.Emit(EventServerInitUpdated, event)
}

func emitActiveClientsUpdated(app *application.App, snapshot domain.ActiveClientSnapshot) {
	if app == nil {
		return
	}
	clients := make([]ActiveClient, 0, len(snapshot.Clients))
	for _, client := range snapshot.Clients {
		clients = append(clients, ActiveClient{
			Client:        client.Client,
			PID:           client.PID,
			Tags:          append([]string(nil), client.Tags...),
			LastHeartbeat: client.LastHeartbeat.Format("2006-01-02T15:04:05.000Z07:00"),
		})
	}
	event := ActiveClientsUpdatedEvent{
		Clients: clients,
	}
	app.Event.Emit(EventActiveClientsUpdated, event)
}
